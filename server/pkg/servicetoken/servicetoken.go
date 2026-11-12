// Package servicetoken gets OAuth client credentials tokens for calling the
// accounts API with no user behind the call (ADR-0002 decision 1). The client
// authenticates with private_key_jwt (RFC 7523), fetches a token on first use and
// caches it in memory until shortly before it expires.
package servicetoken

import (
	"context"
	"crypto"
	"crypto/ecdsa"
	"crypto/rand"
	"crypto/rsa"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

const (
	clientAssertionType = "urn:ietf:params:oauth:client-assertion-type:jwt-bearer"
	assertionLifetime   = 5 * time.Minute
	defaultRefreshEarly = time.Minute
)

type Config struct {
	// TokenURL is the identity provider's token endpoint, such as
	// https://<tenant>.auth0.com/oauth/token. It is also the assertion audience.
	TokenURL string
	ClientID string
	// Audience is the accounts API audience the token is requested for.
	Audience string
	// Key signs the client assertion. An *rsa.PrivateKey signs with RS256 and an
	// *ecdsa.PrivateKey with ES256 unless SigningMethod is set.
	Key           crypto.Signer
	KeyID         string
	SigningMethod jwt.SigningMethod
	// RefreshEarly is how long before expiry a cached token is replaced.
	// Defaults to one minute, and is capped at half the token's lifetime.
	RefreshEarly time.Duration
	HTTPClient   *http.Client
}

// Source returns a valid access token, fetching one only when none is cached or
// the cached one is about to expire. It is safe for concurrent use; concurrent
// callers share a single fetch, and each waits only as long as its own context
// allows.
type Source struct {
	cfg Config
	now func() time.Time

	mu        sync.Mutex
	token     string
	refreshAt time.Time
	inflight  *fetchCall
}

type fetchCall struct {
	done  chan struct{}
	token string
	err   error
}

func New(cfg Config) (*Source, error) {
	if cfg.TokenURL == "" || cfg.ClientID == "" || cfg.Audience == "" || cfg.Key == nil {
		return nil, errors.New("servicetoken: TokenURL, ClientID, Audience and Key are required")
	}
	if cfg.SigningMethod == nil {
		m, err := defaultSigningMethod(cfg.Key)
		if err != nil {
			return nil, err
		}
		cfg.SigningMethod = m
	}
	if cfg.RefreshEarly == 0 {
		cfg.RefreshEarly = defaultRefreshEarly
	}
	if cfg.HTTPClient == nil {
		cfg.HTTPClient = &http.Client{Timeout: 10 * time.Second}
	}
	return &Source{cfg: cfg, now: time.Now}, nil
}

func defaultSigningMethod(k crypto.Signer) (jwt.SigningMethod, error) {
	switch k.Public().(type) {
	case *rsa.PublicKey:
		return jwt.SigningMethodRS256, nil
	case *ecdsa.PublicKey:
		return jwt.SigningMethodES256, nil
	}
	return nil, fmt.Errorf("servicetoken: no default signing method for %T", k)
}

// Token returns a cached token or fetches a new one.
func (s *Source) Token(ctx context.Context) (string, error) {
	s.mu.Lock()
	if s.token != "" && s.now().Before(s.refreshAt) {
		tok := s.token
		s.mu.Unlock()
		return tok, nil
	}
	call := s.inflight
	if call == nil {
		call = &fetchCall{done: make(chan struct{})}
		s.inflight = call
		// The fetch outlives the caller that started it, so a cancelled caller
		// does not fail the others waiting on it.
		go s.run(context.WithoutCancel(ctx), call)
	}
	s.mu.Unlock()

	select {
	case <-call.done:
		return call.token, call.err
	case <-ctx.Done():
		return "", ctx.Err()
	}
}

func (s *Source) run(ctx context.Context, call *fetchCall) {
	tok, expiresIn, err := s.fetch(ctx)
	s.mu.Lock()
	if err == nil {
		s.token = tok
		s.refreshAt = s.now().Add(expiresIn - min(s.cfg.RefreshEarly, expiresIn/2))
	}
	s.inflight = nil
	s.mu.Unlock()
	call.token, call.err = tok, err
	close(call.done)
}

func (s *Source) fetch(ctx context.Context) (string, time.Duration, error) {
	assertion, err := s.assertion()
	if err != nil {
		return "", 0, err
	}
	form := url.Values{
		"grant_type":            {"client_credentials"},
		"client_id":             {s.cfg.ClientID},
		"client_assertion_type": {clientAssertionType},
		"client_assertion":      {assertion},
		"audience":              {s.cfg.Audience},
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, s.cfg.TokenURL, strings.NewReader(form.Encode()))
	if err != nil {
		return "", 0, err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	res, err := s.cfg.HTTPClient.Do(req)
	if err != nil {
		return "", 0, fmt.Errorf("servicetoken: token request: %w", err)
	}
	defer func() { _ = res.Body.Close() }()
	body, err := io.ReadAll(io.LimitReader(res.Body, 1<<20))
	if err != nil {
		return "", 0, fmt.Errorf("servicetoken: token response: %w", err)
	}
	if res.StatusCode != http.StatusOK {
		return "", 0, fmt.Errorf("servicetoken: token endpoint returned %d: %s", res.StatusCode, body)
	}
	var tr struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int64  `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tr); err != nil {
		return "", 0, fmt.Errorf("servicetoken: token response: %w", err)
	}
	if tr.AccessToken == "" || tr.ExpiresIn <= 0 {
		return "", 0, errors.New("servicetoken: token response lacks access_token or expires_in")
	}
	return tr.AccessToken, time.Duration(tr.ExpiresIn) * time.Second, nil
}

// assertion builds the RFC 7523 client assertion: iss and sub are the client ID,
// aud is the token endpoint, and jti is unique per request.
func (s *Source) assertion() (string, error) {
	jti := make([]byte, 16)
	if _, err := rand.Read(jti); err != nil {
		return "", err
	}
	now := s.now()
	t := jwt.NewWithClaims(s.cfg.SigningMethod, jwt.RegisteredClaims{
		Issuer:    s.cfg.ClientID,
		Subject:   s.cfg.ClientID,
		Audience:  jwt.ClaimStrings{s.cfg.TokenURL},
		IssuedAt:  jwt.NewNumericDate(now),
		NotBefore: jwt.NewNumericDate(now),
		ExpiresAt: jwt.NewNumericDate(now.Add(assertionLifetime)),
		ID:        hex.EncodeToString(jti),
	})
	if s.cfg.KeyID != "" {
		t.Header["kid"] = s.cfg.KeyID
	}
	return t.SignedString(s.cfg.Key)
}

// Transport adds the service token to requests that carry no Authorization
// header, so calls made for a user keep the user's token. Place it inside any
// transport that sets the user's token, so it sees the final headers.
type Transport struct {
	Source *Source
	// Base defaults to http.DefaultTransport.
	Base http.RoundTripper
}

func (t *Transport) RoundTrip(req *http.Request) (*http.Response, error) {
	base := t.Base
	if base == nil {
		base = http.DefaultTransport
	}
	if req.Header.Get("Authorization") != "" {
		return base.RoundTrip(req)
	}
	tok, err := t.Source.Token(req.Context())
	if err != nil {
		if req.Body != nil {
			_ = req.Body.Close()
		}
		return nil, err
	}
	req2 := req.Clone(req.Context())
	req2.Header.Set("Authorization", "Bearer "+tok)
	return base.RoundTrip(req2)
}
