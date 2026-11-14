package servicetoken

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

type fakeIdP struct {
	t         *testing.T
	key       *rsa.PublicKey
	srv       *httptest.Server
	fetches   atomic.Int32
	expiresIn int64

	mu         sync.Mutex
	assertions []jwt.MapClaims
	headers    []map[string]any
	forms      []map[string]string
}

func newFakeIdP(t *testing.T, key *rsa.PublicKey) *fakeIdP {
	f := &fakeIdP{t: t, key: key, expiresIn: 3600}
	f.srv = httptest.NewServer(http.HandlerFunc(f.serve))
	t.Cleanup(f.srv.Close)
	return f
}

func (f *fakeIdP) tokenURL() string { return f.srv.URL + "/oauth/token" }

func (f *fakeIdP) serve(w http.ResponseWriter, r *http.Request) {
	n := f.fetches.Add(1)
	time.Sleep(20 * time.Millisecond) // widen the window for concurrent callers
	if err := r.ParseForm(); err != nil {
		http.Error(w, err.Error(), http.StatusBadRequest)
		return
	}
	tok, err := jwt.Parse(r.PostForm.Get("client_assertion"), func(*jwt.Token) (any, error) { return f.key, nil },
		jwt.WithValidMethods([]string{"RS256"}), jwt.WithoutClaimsValidation())
	if err != nil {
		http.Error(w, err.Error(), http.StatusUnauthorized)
		return
	}
	f.mu.Lock()
	f.assertions = append(f.assertions, tok.Claims.(jwt.MapClaims))
	f.headers = append(f.headers, tok.Header)
	form := map[string]string{}
	for k := range r.PostForm {
		form[k] = r.PostForm.Get(k)
	}
	f.forms = append(f.forms, form)
	f.mu.Unlock()
	_ = json.NewEncoder(w).Encode(map[string]any{
		"access_token": fmt.Sprintf("token-%d", n),
		"token_type":   "Bearer",
		"expires_in":   f.expiresIn,
	})
}

func newSource(t *testing.T) (*Source, *fakeIdP) {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp := newFakeIdP(t, &key.PublicKey)
	s, err := New(Config{TokenURL: idp.tokenURL(), ClientID: "svc-flow", Audience: "https://accounts", Key: key, KeyID: "k1"})
	require.NoError(t, err)
	return s, idp
}

func TestSource_ConcurrentCallersShareOneFetch(t *testing.T) {
	s, idp := newSource(t)
	var wg sync.WaitGroup
	got := make([]string, 50)
	for i := range got {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			tok, err := s.Token(context.Background())
			assert.NoError(t, err)
			got[i] = tok
		}(i)
	}
	wg.Wait()
	assert.Equal(t, int32(1), idp.fetches.Load())
	for _, tok := range got {
		assert.Equal(t, "token-1", tok)
	}
}

func TestSource_RefetchesShortlyBeforeExpiry(t *testing.T) {
	s, idp := newSource(t)
	now := time.Now()
	s.now = func() time.Time { return now }

	tok, err := s.Token(context.Background())
	require.NoError(t, err)
	assert.Equal(t, "token-1", tok)

	now = now.Add(58 * time.Minute)
	tok, _ = s.Token(context.Background())
	assert.Equal(t, "token-1", tok, "still cached two minutes before expiry")

	now = now.Add(90 * time.Second)
	tok, _ = s.Token(context.Background())
	assert.Equal(t, "token-2", tok, "refetched inside the refresh window")
	assert.Equal(t, int32(2), idp.fetches.Load())
}

func TestSource_AssertionClaims(t *testing.T) {
	s, idp := newSource(t)
	before := time.Now().Add(-time.Second)
	_, err := s.Token(context.Background())
	require.NoError(t, err)
	s.token = "" // force a second fetch to compare jti
	_, err = s.Token(context.Background())
	require.NoError(t, err)

	require.Len(t, idp.assertions, 2)
	c := idp.assertions[0]
	assert.Equal(t, "svc-flow", c["iss"])
	assert.Equal(t, "svc-flow", c["sub"])
	assert.Equal(t, []any{idp.tokenURL()}, c["aud"])
	exp, err := c.GetExpirationTime()
	require.NoError(t, err)
	assert.True(t, exp.After(before) && exp.Before(time.Now().Add(assertionLifetime+time.Second)))
	assert.NotEmpty(t, c["jti"])
	assert.NotEqual(t, c["jti"], idp.assertions[1]["jti"], "jti is unique per request")
	assert.Equal(t, "k1", idp.headers[0]["kid"])

	f := idp.forms[0]
	assert.Equal(t, "client_credentials", f["grant_type"])
	assert.Equal(t, clientAssertionType, f["client_assertion_type"])
	assert.Equal(t, "https://accounts", f["audience"])
	assert.NotContains(t, f, "client_secret")
}

func TestSource_TokenEndpointError(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	idp := newFakeIdP(t, &other.PublicKey) // rejects assertions signed by key
	s, err := New(Config{TokenURL: idp.tokenURL(), ClientID: "c", Audience: "a", Key: key})
	require.NoError(t, err)
	_, err = s.Token(context.Background())
	assert.ErrorContains(t, err, "401")
}

func TestTransport_AddsServiceTokenOnlyWithoutUser(t *testing.T) {
	s, _ := newSource(t)
	var seen []string
	api := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = append(seen, r.Header.Get("Authorization"))
	}))
	defer api.Close()
	c := &http.Client{Transport: &Transport{Source: s}}

	req, _ := http.NewRequest(http.MethodPost, api.URL, nil)
	_, err := c.Do(req)
	require.NoError(t, err)

	req, _ = http.NewRequest(http.MethodPost, api.URL, nil)
	req.Header.Set("Authorization", "Bearer user-token")
	_, err = c.Do(req)
	require.NoError(t, err)

	assert.Equal(t, []string{"Bearer token-1", "Bearer user-token"}, seen)
}

func TestSource_WaiterHonoursItsOwnContext(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	release := make(chan struct{})
	hang := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		<-release
	}))
	defer hang.Close()
	defer close(release)
	s, err := New(Config{TokenURL: hang.URL, ClientID: "c", Audience: "a", Key: key, HTTPClient: &http.Client{}})
	require.NoError(t, err)

	go func() { _, _ = s.Token(context.Background()) }()
	time.Sleep(50 * time.Millisecond)

	ctx, cancel := context.WithTimeout(context.Background(), 200*time.Millisecond)
	defer cancel()
	done := make(chan error, 1)
	go func() {
		_, err := s.Token(ctx)
		done <- err
	}()
	select {
	case err := <-done:
		assert.ErrorIs(t, err, context.DeadlineExceeded)
	case <-time.After(time.Second):
		t.Fatal("waiter blocked past its own deadline")
	}
}

func TestSource_CachesShortLivedTokens(t *testing.T) {
	s, idp := newSource(t)
	idp.expiresIn = 60 // equal to the default RefreshEarly
	for range 3 {
		_, err := s.Token(context.Background())
		require.NoError(t, err)
	}
	assert.Equal(t, int32(1), idp.fetches.Load())
}

type closeTracker struct {
	io.Reader
	closed bool
}

func (c *closeTracker) Close() error { c.closed = true; return nil }

func TestTransport_ClosesBodyWhenTokenFails(t *testing.T) {
	key, _ := rsa.GenerateKey(rand.Reader, 2048)
	other, _ := rsa.GenerateKey(rand.Reader, 2048)
	idp := newFakeIdP(t, &other.PublicKey)
	s, err := New(Config{TokenURL: idp.tokenURL(), ClientID: "c", Audience: "a", Key: key})
	require.NoError(t, err)

	body := &closeTracker{Reader: strings.NewReader("{}")}
	req, _ := http.NewRequest(http.MethodPost, "http://accounts.invalid", body)
	_, err = (&Transport{Source: s}).RoundTrip(req)
	assert.Error(t, err)
	assert.True(t, body.closed)
}

func TestSource_HungFetchDoesNotWedgeTheSource(t *testing.T) {
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp := newFakeIdP(t, &key.PublicKey)
	var hits atomic.Int32
	release := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if hits.Add(1) == 1 {
			select {
			case <-release:
			case <-r.Context().Done():
			}
			return
		}
		idp.serve(w, r)
	}))
	defer srv.Close()
	defer close(release)

	s, err := New(Config{TokenURL: srv.URL, ClientID: "svc-flow", Audience: "https://accounts", Key: key, HTTPClient: &http.Client{}})
	require.NoError(t, err)
	s.fetchTimeout = 100 * time.Millisecond

	_, err = s.Token(context.Background())
	assert.ErrorIs(t, err, context.DeadlineExceeded)
	tok, err := s.Token(context.Background())
	require.NoError(t, err)
	assert.NotEmpty(t, tok)
	assert.Equal(t, int32(2), hits.Load())
}
