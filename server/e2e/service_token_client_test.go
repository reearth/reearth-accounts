package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient"
	"github.com/reearth/reearth-accounts/server/pkg/servicetoken"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/go-jose/go-jose.v2"
)

// tokenIdP issues client credentials tokens to a client that proves itself with
// a private_key_jwt assertion, as Auth0 does.
func newTokenIdP(t *testing.T, clientID string, clientKey *rsa.PublicKey) (*httptest.Server, *atomic.Int32) {
	t.Helper()
	signKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	var issued atomic.Int32
	var srv *httptest.Server
	mux := http.NewServeMux()
	mux.HandleFunc("/jwks.json", func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &signKey.PublicKey, KeyID: "idp", Algorithm: "RS256", Use: "sig"}}})
	})
	mux.HandleFunc("/oauth/token", func(w http.ResponseWriter, r *http.Request) {
		_ = r.ParseForm()
		_, err := jwt.Parse(r.PostForm.Get("client_assertion"), func(*jwt.Token) (any, error) { return clientKey, nil },
			jwt.WithIssuer(clientID), jwt.WithSubject(clientID), jwt.WithAudience(srv.URL+"/oauth/token"))
		if err != nil || r.PostForm.Get("audience") != svcAudience {
			http.Error(w, "invalid_client", http.StatusUnauthorized)
			return
		}
		issued.Add(1)
		at := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
			"iss": srv.URL + "/", "aud": svcAudience, "sub": clientID + "@clients", "azp": clientID,
			"gty": "client-credentials", "iat": time.Now().Unix(), "exp": time.Now().Add(time.Hour).Unix(),
		})
		at.Header["kid"] = "idp"
		s, _ := at.SignedString(signKey)
		_ = json.NewEncoder(w).Encode(map[string]any{"access_token": s, "token_type": "Bearer", "expires_in": 3600})
	})
	srv = httptest.NewServer(mux)
	t.Cleanup(srv.Close)
	return srv, &issued
}

func TestServiceToken_GQLClientCallsAccountsAsService(t *testing.T) {
	clientKey, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	idp, issued := newTokenIdP(t, "svc-flow", &clientKey.PublicKey)

	jwks := idp.URL + "/jwks.json"
	cfg := &app.Config{Auth: app.AuthConfigs{{ISS: idp.URL + "/", AUD: []string{svcAudience}, JWKSURI: &jwks}}}
	callers, _ := json.Marshal([]map[string]string{{"issuer": idp.URL + "/", "client_id": "svc-flow", "integration_id": svcIntegration.String()}})
	require.NoError(t, cfg.ServiceCallers.Decode(string(callers)))
	repos := seededMemoryRepos(t, seedServiceCallers)
	host := startServerURL(t, cfg, repos, nil, false)

	src, err := servicetoken.New(servicetoken.Config{TokenURL: idp.URL + "/oauth/token", ClientID: "svc-flow", Audience: svcAudience, Key: clientKey})
	require.NoError(t, err)
	client := gqlclient.NewClient(host, 10, &servicetoken.Transport{Source: src})

	for range 3 {
		ws, err := client.WorkspaceRepo.FindByUser(context.Background(), svcOwnerUID.String())
		require.NoError(t, err)
		var ids []string
		for _, w := range ws {
			ids = append(ids, w.ID().String())
		}
		assert.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, ids)
	}
	assert.Equal(t, int32(1), issued.Load(), "token fetched once and reused")
}

func seededMemoryRepos(t *testing.T, seed Seeder) *repo.Container {
	t.Helper()
	r := memory.New()
	require.NoError(t, seed(context.Background(), r))
	return r
}
