package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/go-jose/go-jose.v2"
)

const testAudience = "https://accounts.test"

// testIdP is a local identity provider: an RSA key and a JWKS endpoint.
type testIdP struct {
	key    *rsa.PrivateKey
	server *httptest.Server
}

func newTestIdP(t *testing.T) *testIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	jwks := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(jwks)
	}))
	t.Cleanup(srv.Close)
	return &testIdP{key: key, server: srv}
}

func (p *testIdP) issuer() string { return p.server.URL + "/" }

func (p *testIdP) provider() appx.JWTProvider {
	jwksURI := p.server.URL + "/.well-known/jwks.json"
	return appx.JWTProvider{ISS: p.issuer(), AUD: []string{testAudience}, JWKSURI: &jwksURI}
}

func (p *testIdP) sign(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	base := jwt.MapClaims{
		"iss": p.issuer(),
		"aud": testAudience,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range claims {
		base[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, base)
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(p.key)
	require.NoError(t, err)
	return s
}

func clientClaims(clientID string) jwt.MapClaims {
	return jwt.MapClaims{"sub": clientID + "@clients", "azp": clientID, "gty": "client-credentials"}
}

// authStack is the production GraphQL auth chain: appx JWT validation, then
// authMiddleware. The handler echoes the attached operator.
func authStack(t *testing.T, cfg *ServerConfig) http.Handler {
	t.Helper()
	jwtm, err := appx.AuthMiddleware(cfg.Config.Auths(), adapter.AuthInfoKey, true)
	require.NoError(t, err)
	return jwtm(authMiddleware(cfg)(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_ = json.NewEncoder(w).Encode(adapter.Operator(r.Context()))
	})))
}

func callWith(h http.Handler, token string) (*httptest.ResponseRecorder, *workspace.Operator) {
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", nil)
	req.Header.Set("Authorization", "Bearer "+token)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		return rr, nil
	}
	var op *workspace.Operator
	_ = json.Unmarshal(rr.Body.Bytes(), &op)
	return rr, op
}

type serviceFixture struct {
	repos       *repo.Container
	integration id.IntegrationID
	readable    id.WorkspaceID
	writable    id.WorkspaceID
	other       id.WorkspaceID
	userSub     string
	userID      id.UserID
}

func newServiceFixture(t *testing.T) serviceFixture {
	t.Helper()
	ctx := context.Background()
	repos := memory.New()
	f := serviceFixture{repos: repos, integration: id.NewIntegrationID(), userSub: "auth0|u1", userID: user.NewID()}

	u := user.New().ID(f.userID).Name("u1").Email("u1@example.com").
		Auths([]user.Auth{{Provider: "auth0", Sub: f.userSub}}).MustBuild()
	require.NoError(t, repos.User.Save(ctx, u))

	mk := func(r role.RoleType) id.WorkspaceID {
		b := workspace.New().NewID().Name("ws").
			Members(map[workspace.UserID]workspace.Member{f.userID: {Role: role.RoleOwner}})
		if r != "" {
			b = b.Integrations(map[workspace.IntegrationID]workspace.Member{f.integration: {Role: r, InvitedBy: f.userID}})
		}
		ws := b.MustBuild()
		require.NoError(t, repos.Workspace.Save(ctx, ws))
		return ws.ID()
	}
	f.readable = mk(role.RoleReader)
	f.writable = mk(role.RoleWriter)
	f.other = mk("")
	return f
}

func serviceConfig(t *testing.T, repos *repo.Container, idps []*testIdP, callers string) *ServerConfig {
	t.Helper()
	c := &Config{}
	for _, p := range idps {
		c.Auth = append(c.Auth, p.provider())
	}
	require.NoError(t, c.ServiceCallers.Decode(callers))
	require.NoError(t, c.ServiceCallers.checkIssuers(c.Auths()))
	return &ServerConfig{Config: c, Repos: repos}
}

func callersJSON(issuer, clientID string, iid id.IntegrationID) string {
	b, _ := json.Marshal([]serviceCallerEntry{{Issuer: issuer, ClientID: clientID, IntegrationID: iid.String()}})
	return string(b)
}

func TestServiceCallerAuth(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{idp}, callersJSON(idp.issuer(), "svc-flow", f.integration))
	h := authStack(t, cfg)

	t.Run("registered client gets a userless operator scoped to its memberships", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, clientClaims("svc-flow")))
		require.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, op)
		assert.Nil(t, op.User)
		assert.Equal(t, &f.integration, op.Integration)
		assert.Equal(t, id.WorkspaceIDList{f.readable}, op.ReadableWorkspaces)
		assert.Equal(t, id.WorkspaceIDList{f.writable}, op.WritableWorkspaces)
		assert.False(t, op.IsReadableWorkspace(f.other))
		assert.False(t, op.IsWritableWorkspace(f.readable))
	})

	t.Run("RFC 9068 shaped token with client_id and no gty is accepted", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": "svc-flow@clients", "client_id": "svc-flow"}))
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, &f.integration, op.Integration)
	})

	t.Run("unregistered client is 401", func(t *testing.T) {
		rr, _ := callWith(h, idp.sign(t, clientClaims("svc-unknown")))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("user token is unchanged", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "azp": "spa"}))
		require.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, op.User)
		assert.Equal(t, f.userID, *op.User)
		assert.Nil(t, op.Integration)
	})

	t.Run("user token with non-string claims is unchanged", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "gty": []string{"refresh_token", "password"}, "client_id": 42}))
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, f.userID, *op.User)
	})

	mixed := map[string]jwt.MapClaims{
		"client credentials token with a non-string azp": {"sub": "svc-flow@clients", "azp": 7, "gty": "client-credentials"},
		"client credentials grant with a user subject":   {"sub": f.userSub, "azp": "svc-flow", "gty": "client-credentials"},
		"client subject with a user grant":               {"sub": "svc-flow@clients", "azp": "svc-flow", "gty": "password"},
		"client credentials token with an email":         {"sub": "svc-flow@clients", "azp": "svc-flow", "gty": "client-credentials", "email": "u1@example.com"},
		"azp and client_id disagree":                     {"sub": "svc-flow@clients", "azp": "svc-flow", "client_id": "svc-other", "gty": "client-credentials"},
		"client credentials token without a client id":   {"sub": "svc-flow@clients", "gty": "client-credentials"},
	}
	for name, claims := range mixed {
		t.Run("rejects "+name, func(t *testing.T) {
			rr, _ := callWith(h, idp.sign(t, claims))
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}

	t.Run("token from an unconfigured issuer is 401", func(t *testing.T) {
		claims := clientClaims("svc-flow")
		claims["iss"] = idp.server.URL // no trailing slash: not an exact match
		rr, _ := callWith(h, idp.sign(t, claims))
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestServiceCallerIssuerIsPartOfTheRegistration(t *testing.T) {
	registered, other := newTestIdP(t), newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{registered, other}, callersJSON(registered.issuer(), "svc-flow", f.integration))
	h := authStack(t, cfg)

	rr, _ := callWith(h, other.sign(t, clientClaims("svc-flow")))
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "same client id from another trusted issuer")

	rr, _ = callWith(h, registered.sign(t, clientClaims("svc-flow")))
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestServiceCallerWithoutMembershipSeesNothing(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	lonely := id.NewIntegrationID()
	cfg := serviceConfig(t, f.repos, []*testIdP{idp}, callersJSON(idp.issuer(), "svc-lonely", lonely))

	rr, op := callWith(authStack(t, cfg), idp.sign(t, clientClaims("svc-lonely")))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Empty(t, op.AllReadableWorkspaces())
	assert.False(t, op.IsReadableWorkspace(f.readable, f.writable, f.other))
}

func TestServiceCallersDecode(t *testing.T) {
	iid := id.NewIntegrationID()
	var s ServiceCallers
	require.NoError(t, s.Decode(""))
	assert.Empty(t, s)

	require.NoError(t, s.Decode(callersJSON("https://idp/", "c1", iid)))
	assert.Equal(t, iid, s[serviceCallerKey{issuer: "https://idp/", clientID: "c1"}])

	assert.Error(t, s.Decode(`[{"issuer":"https://idp/","client_id":"c1","integration_id":"nope"}]`))
	assert.Error(t, s.Decode(`[{"client_id":"c1","integration_id":"`+iid.String()+`"}]`))
	dup := `{"issuer":"https://idp/","client_id":"c1","integration_id":"` + iid.String() + `"}`
	assert.Error(t, s.Decode("["+dup+","+dup+"]"))

	require.NoError(t, s.Decode(callersJSON("https://idp/", "c1", iid)))
	assert.Error(t, s.checkIssuers([]appx.JWTProvider{{ISS: "https://idp"}}), "issuer must match exactly")
	assert.NoError(t, s.checkIssuers([]appx.JWTProvider{{ISS: "https://idp/"}}))
}

func TestServiceCallersDisabledLeavesUserTokensAlone(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	h := authStack(t, serviceConfig(t, f.repos, []*testIdP{idp}, ""))

	rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "gty": []string{"password"}}))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, f.userID, *op.User)

	rr, _ = callWith(h, idp.sign(t, clientClaims("svc-flow")))
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "an unregistered client still has no user")
}
