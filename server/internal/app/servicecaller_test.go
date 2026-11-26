package app

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
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

const publicQuery = `{"query":"{ authConfig { __typename } }"}`

func callWith(h http.Handler, token string, actAs ...string) (*httptest.ResponseRecorder, *workspace.Operator) {
	return callBody(h, token, "", actAs...)
}

func callBody(h http.Handler, token, body string, actAs ...string) (*httptest.ResponseRecorder, *workspace.Operator) {
	req := httptest.NewRequest(http.MethodPost, "/api/graphql", strings.NewReader(body))
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	for _, v := range actAs {
		req.Header.Add(ActAsHeader, v)
	}
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

func (f serviceFixture) actAs() string { return "integration:" + f.integration.String() }

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

func callersJSON(issuer, clientID string, actAs ...string) string {
	if actAs == nil {
		actAs = []string{}
	}
	b, _ := json.Marshal([]map[string]any{{"issuer": issuer, "client_id": clientID, "name": clientID, "act_as": actAs}})
	return string(b)
}

func TestServiceCallerAuth(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{idp}, callersJSON(idp.issuer(), "svc-flow", "integration"))
	h := authStack(t, cfg)

	t.Run("client acting for an integration gets a userless operator scoped to its memberships", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, clientClaims("svc-flow")), f.actAs())
		require.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, op)
		assert.Nil(t, op.User)
		assert.Equal(t, &f.integration, op.Integration)
		assert.Equal(t, &workspace.Client{ID: "svc-flow", Name: "svc-flow"}, op.Client)
		assert.Equal(t, id.WorkspaceIDList{f.readable}, op.ReadableWorkspaces)
		assert.Equal(t, id.WorkspaceIDList{f.writable}, op.WritableWorkspaces)
		assert.False(t, op.IsReadableWorkspace(f.other))
		assert.False(t, op.IsWritableWorkspace(f.readable))
	})

	t.Run("RFC 9068 shaped token with client_id and no gty is accepted", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": "svc-flow@clients", "client_id": "svc-flow"}), f.actAs())
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, &f.integration, op.Integration)
	})

	t.Run("client token without the header is anonymous", func(t *testing.T) {
		rr, op := callBody(h, idp.sign(t, clientClaims("svc-flow")), publicQuery)
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, op)

		rr, op = callWith(h, idp.sign(t, clientClaims("svc-flow")))
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Nil(t, op)
	})

	t.Run("unregistered client is 401", func(t *testing.T) {
		rr, _ := callWith(h, idp.sign(t, clientClaims("svc-unknown")), f.actAs())
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("user token is unchanged", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "azp": "spa"}))
		require.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, op.User)
		assert.Equal(t, f.userID, *op.User)
		assert.Nil(t, op.Integration)
		assert.Nil(t, op.Client)
	})

	t.Run("user token with non-string claims is unchanged", func(t *testing.T) {
		rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "gty": []string{"refresh_token", "password"}, "client_id": 42}))
		require.Equal(t, http.StatusOK, rr.Code)
		assert.Equal(t, f.userID, *op.User)
	})

	t.Run("user token with the header is 401, even on a public operation", func(t *testing.T) {
		rr, _ := callBody(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "azp": "spa"}), publicQuery, f.actAs())
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})

	t.Run("header without a token is 401, even on a public operation", func(t *testing.T) {
		rr, _ := callBody(h, "", publicQuery)
		require.Equal(t, http.StatusOK, rr.Code)
		rr, _ = callBody(h, "", publicQuery, f.actAs())
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
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
			rr, _ := callWith(h, idp.sign(t, claims), f.actAs())
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}

	t.Run("token from an unconfigured issuer is 401", func(t *testing.T) {
		claims := clientClaims("svc-flow")
		claims["iss"] = idp.server.URL // no trailing slash: not an exact match
		rr, _ := callWith(h, idp.sign(t, claims), f.actAs())
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestServiceCallerActingForAUser(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	gone := user.New().NewID().Name("gone").Email("gone@example.com").MustBuild()
	gone.Deactivate()
	require.NoError(t, f.repos.User.Save(context.Background(), gone))
	callers, _ := json.Marshal([]map[string]any{
		{"issuer": idp.issuer(), "client_id": "svc-users", "name": "svc-users", "act_as": []string{"user"}},
		{"issuer": idp.issuer(), "client_id": "svc-int", "name": "svc-int", "act_as": []string{"integration"}},
	})
	h := authStack(t, serviceConfig(t, f.repos, []*testIdP{idp}, string(callers)))
	tok := idp.sign(t, clientClaims("svc-users"))

	t.Run("the operator is the user's own, marked as a client call", func(t *testing.T) {
		_, own := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub}))
		rr, op := callWith(h, tok, "user:"+f.userID.String())
		require.Equal(t, http.StatusOK, rr.Code)
		require.NotNil(t, op)
		assert.Equal(t, &workspace.Client{ID: "svc-users", Name: "svc-users"}, op.Client)
		own.Client = op.Client
		assert.Equal(t, own, op)
		assert.True(t, op.ViaClient())
		assert.False(t, op.IsUserSelf())
		assert.False(t, op.IsService())
	})

	for name, actAs := range map[string]string{
		"an unknown user":    "user:" + user.NewID().String(),
		"a deactivated user": "user:" + gone.ID().String(),
	} {
		t.Run(name+" is 401", func(t *testing.T) {
			rr, _ := callWith(h, tok, actAs)
			assert.Equal(t, http.StatusUnauthorized, rr.Code)
		})
	}

	t.Run("a client without user in act_as is 401", func(t *testing.T) {
		rr, _ := callWith(h, idp.sign(t, clientClaims("svc-int")), "user:"+f.userID.String())
		assert.Equal(t, http.StatusUnauthorized, rr.Code)
	})
}

func TestServiceCallerIssuerIsPartOfTheRegistration(t *testing.T) {
	registered, other := newTestIdP(t), newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{registered, other}, callersJSON(registered.issuer(), "svc-flow", "integration"))
	h := authStack(t, cfg)

	rr, _ := callWith(h, other.sign(t, clientClaims("svc-flow")), f.actAs())
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "same client id from another trusted issuer")

	rr, _ = callWith(h, registered.sign(t, clientClaims("svc-flow")), f.actAs())
	assert.Equal(t, http.StatusOK, rr.Code)
}

func TestServiceCallerIntegrationWithoutMembershipSeesNothing(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	cfg := serviceConfig(t, f.repos, []*testIdP{idp}, callersJSON(idp.issuer(), "svc-flow", "integration"))

	rr, op := callWith(authStack(t, cfg), idp.sign(t, clientClaims("svc-flow")), "integration:"+id.NewIntegrationID().String())
	require.Equal(t, http.StatusOK, rr.Code)
	require.NotNil(t, op)
	assert.Empty(t, op.AllReadableWorkspaces())
	assert.False(t, op.IsReadableWorkspace(f.readable, f.writable, f.other))
}

func TestServiceCallersDecode(t *testing.T) {
	var s ServiceCallers
	require.NoError(t, s.Decode(""))
	assert.Empty(t, s)

	require.NoError(t, s.Decode(`[{"issuer":"https://idp/","client_id":"c1","name":"svc-one","act_as":["integration","user"]},
		{"issuer":"https://idp/","client_id":"c2","name":"anon","act_as":[]},
		{"issuer":"https://other/","client_id":"c1","name":"svc-one","act_as":["user"]}]`))
	assert.Equal(t, ServiceCallers{
		{issuer: "https://idp/", clientID: "c1"}:   {Issuer: "https://idp/", Client: workspace.Client{ID: "c1", Name: "svc-one"}, ActAs: ActAsIntegration | ActAsUser},
		{issuer: "https://idp/", clientID: "c2"}:   {Issuer: "https://idp/", Client: workspace.Client{ID: "c2", Name: "anon"}},
		{issuer: "https://other/", clientID: "c1"}: {Issuer: "https://other/", Client: workspace.Client{ID: "c1", Name: "svc-one"}, ActAs: ActAsUser},
	}, s)

	entry := func(fields string) string { return `[{"issuer":"https://idp/","client_id":"c1",` + fields + `}]` }
	for name, tc := range map[string]struct{ value, msg string }{
		"pre-amendment shape":   {`[{"issuer":"https://idp/","client_id":"c1","integration_id":"` + id.NewIntegrationID().String() + `"}]`, "act_as"},
		"unknown key":           {entry(`"name":"a","act_as":[],"extra":1`), `unknown key "extra"`},
		"upper-case act_as key": {entry(`"name":"a","ACT_AS":[]`), `unknown key "ACT_AS"`},
		"capitalized issuer":    {`[{"Issuer":"https://idp/","client_id":"c1","name":"a","act_as":[]}]`, `unknown key "Issuer"`},
		"upper-case client_id":  {`[{"issuer":"https://idp/","CLIENT_ID":"c1","name":"a","act_as":[]}]`, `unknown key "CLIENT_ID"`},
		"capitalized name":      {entry(`"Name":"a","act_as":[]`), `unknown key "Name"`},
		"repeated act_as":       {entry(`"name":"a","act_as":["user"],"act_as":[]`), `key "act_as" appears twice`},
		"repeated name":         {entry(`"name":"a","name":"b","act_as":[]`), `key "name" appears twice`},
		"entry not an object":   {`[null]`, "must be an object"},
		"not json":              {`{`, "invalid service callers json"},
		"not an array":          {`{"issuer":"x"}`, "invalid service callers json"},
		"null":                  {`null`, "must be an array"},
		"a string":              {`"x"`, "invalid service callers json"},
		"a number":              {`1`, "invalid service callers json"},
		"trailing data":         {entry(`"name":"a","act_as":[]`) + `[]`, "invalid service callers json"},
		"missing issuer":        {`[{"client_id":"c1","name":"a","act_as":[]}]`, "issuer, client_id and name are required"},
		"missing client_id":     {`[{"issuer":"https://idp/","name":"a","act_as":[]}]`, "issuer, client_id and name are required"},
		"missing name":          {entry(`"act_as":[]`), "issuer, client_id and name are required"},
		"missing act_as":        {entry(`"name":"a"`), "act_as is required"},
		"null act_as":           {entry(`"name":"a","act_as":null`), "act_as is required"},
		"unknown act_as value":  {entry(`"name":"a","act_as":["admin"]`), `"admin"`},
		"upper-case act_as":     {entry(`"name":"a","act_as":["User"]`), `"User"`},
		"duplicate act_as":      {entry(`"name":"a","act_as":["user","user"]`), "twice"},
		"upper-case name":       {entry(`"name":"Svc","act_as":[]`), "must match"},
		"name with a space":     {entry(`"name":"svc one","act_as":[]`), "must match"},
		"name with a newline":   {entry(`"name":"svc\nx","act_as":[]`), "must match"},
		"name starting with -":  {entry(`"name":"-svc","act_as":[]`), "must match"},
		"name over 63 chars":    {entry(`"name":"` + strings.Repeat("a", 64) + `","act_as":[]`), "must match"},
		"duplicate (iss, cid)":  {`[{"issuer":"https://idp/","client_id":"c1","name":"a","act_as":[]},{"issuer":"https://idp/","client_id":"c1","name":"b","act_as":["user"]}]`, "registered twice"},
	} {
		t.Run(name, func(t *testing.T) {
			var s ServiceCallers
			err := s.Decode(tc.value)
			require.Error(t, err)
			assert.Contains(t, err.Error(), tc.msg)
		})
	}

	require.NoError(t, s.Decode(entry(`"name":"`+strings.Repeat("a", 63)+`","act_as":[]`)))
	assert.Error(t, s.checkIssuers([]appx.JWTProvider{{ISS: "https://idp"}}), "issuer must match exactly")
	assert.NoError(t, s.checkIssuers([]appx.JWTProvider{{ISS: "https://idp/"}}))
}

func TestParseActAs(t *testing.T) {
	iid, uid := id.NewIntegrationID(), id.NewUserID()

	sub, err := parseActAs("integration:" + iid.String())
	require.NoError(t, err)
	assert.Equal(t, workspace.IntegrationSubject(iid), sub)
	assert.Equal(t, "integration:"+iid.String(), sub.String())

	sub, err = parseActAs("user:" + uid.String())
	require.NoError(t, err)
	assert.Equal(t, workspace.UserSubject(uid), sub)

	for _, v := range []string{
		"",
		"integration",
		"integration:",
		"user:",
		":" + iid.String(),
		"INTEGRATION:" + iid.String(),
		"User:" + uid.String(),
		"team:" + iid.String(),
		"integration:" + iid.String() + ":x",
		"integration::" + iid.String(),
		"integration:" + strings.ToUpper(iid.String()),
		"integration: " + iid.String(),
		"integration:nope",
		"user:" + uid.String() + ",user:" + uid.String(),
	} {
		_, err := parseActAs(v)
		assert.Error(t, err, "%q", v)
	}
}

func TestAuthenticate(t *testing.T) {
	idp := newTestIdP(t)
	iid, uid := id.NewIntegrationID(), id.NewUserID()
	var s ServiceCallers
	require.NoError(t, s.Decode(`[
		{"issuer":"`+idp.issuer()+`","client_id":"both","name":"both","act_as":["integration","user"]},
		{"issuer":"`+idp.issuer()+`","client_id":"int","name":"int","act_as":["integration"]},
		{"issuer":"`+idp.issuer()+`","client_id":"usr","name":"usr","act_as":["user"]},
		{"issuer":"`+idp.issuer()+`","client_id":"anon","name":"anon","act_as":[]}]`))
	client := func(cid string) *appx.AuthInfo {
		return &appx.AuthInfo{Token: idp.sign(t, clientClaims(cid)), Iss: idp.issuer(), Sub: cid + "@clients"}
	}
	userAI := &appx.AuthInfo{Token: idp.sign(t, jwt.MapClaims{"sub": "auth0|u1"}), Iss: idp.issuer(), Sub: "auth0|u1"}
	asInt := "integration:" + iid.String()
	asUser := "user:" + uid.String()

	accepted := []struct {
		name  string
		ai    *appx.AuthInfo
		actAs []string
		want  workspace.Subject
	}{
		{"integration subject", client("int"), []string{asInt}, workspace.IntegrationSubject(iid)},
		{"integration subject, client with both kinds", client("both"), []string{asInt}, workspace.IntegrationSubject(iid)},
		{"user subject", client("usr"), []string{asUser}, workspace.UserSubject(uid)},
		{"user subject, client with both kinds", client("both"), []string{asUser}, workspace.UserSubject(uid)},
		{"no header is anonymous", client("int"), nil, workspace.Subject{}},
		{"no header is anonymous, act_as []", client("anon"), nil, workspace.Subject{}},
	}
	for _, tc := range accepted {
		t.Run("accepts "+tc.name, func(t *testing.T) {
			ca, err := s.authenticate(tc.ai, tc.actAs)
			require.NoError(t, err)
			require.NotNil(t, ca)
			assert.Equal(t, tc.want, ca.subject)
			assert.Equal(t, tc.ai.Sub, ca.client.ID+"@clients")
		})
	}

	for name, ai := range map[string]*appx.AuthInfo{"no token": nil, "empty token": {}, "user token": userAI} {
		t.Run(name+" without a header is not a client call", func(t *testing.T) {
			ca, err := s.authenticate(ai, nil)
			assert.NoError(t, err)
			assert.Nil(t, ca)
		})
	}

	rejected := []struct {
		name   string
		ai     *appx.AuthInfo
		actAs  []string
		reason rejectReason
	}{
		{"user token with the header", userAI, []string{asInt}, reasonActAsWithoutClient},
		{"no token with the header", nil, []string{asInt}, reasonActAsWithoutClient},
		{"no token with an empty header", nil, []string{""}, reasonActAsWithoutClient},
		{"azp and client_id disagree", &appx.AuthInfo{Token: idp.sign(t, jwt.MapClaims{"sub": "int@clients", "azp": "int", "client_id": "both", "gty": "client-credentials"}), Iss: idp.issuer()}, nil, reasonBadClaims},
		{"bad claims", &appx.AuthInfo{Token: idp.sign(t, jwt.MapClaims{"sub": "int@clients", "azp": "int", "gty": "password"}), Iss: idp.issuer()}, []string{asInt}, reasonBadClaims},
		{"unregistered client", client("ghost"), []string{asInt}, reasonUnregistered},
		{"registered client from another issuer", &appx.AuthInfo{Token: idp.sign(t, clientClaims("int")), Iss: "https://other.example.com/"}, []string{asInt}, reasonUnregistered},
		{"empty header", client("int"), []string{""}, reasonMalformedActAs},
		{"repeated header", client("int"), []string{asInt, asInt}, reasonMalformedActAs},
		{"malformed header", client("int"), []string{"integration:nope"}, reasonMalformedActAs},
		{"unknown kind", client("both"), []string{"team:" + iid.String()}, reasonMalformedActAs},
		{"integration not in act_as", client("usr"), []string{asInt}, reasonKindNotAllowed},
		{"integration with act_as []", client("anon"), []string{asInt}, reasonKindNotAllowed},
		{"user not in act_as", client("int"), []string{asUser}, reasonKindNotAllowed},
	}
	for _, tc := range rejected {
		t.Run("rejects "+tc.name, func(t *testing.T) {
			ca, err := s.authenticate(tc.ai, tc.actAs)
			assert.Nil(t, ca)
			var r *clientRejected
			require.ErrorAs(t, err, &r)
			assert.Equal(t, tc.reason, r.reason)
		})
	}

	t.Run("an empty registry refuses every client token as unregistered", func(t *testing.T) {
		for _, actAs := range [][]string{nil, {asInt}} {
			ca, err := ServiceCallers(nil).authenticate(client("int"), actAs)
			assert.Nil(t, ca)
			var r *clientRejected
			require.ErrorAs(t, err, &r)
			assert.Equal(t, reasonUnregistered, r.reason)
		}
		ca, err := ServiceCallers(nil).authenticate(userAI, nil)
		assert.NoError(t, err)
		assert.Nil(t, ca)
	})
}

func TestServiceCallersDisabledLeavesUserTokensAlone(t *testing.T) {
	idp := newTestIdP(t)
	f := newServiceFixture(t)
	h := authStack(t, serviceConfig(t, f.repos, []*testIdP{idp}, ""))

	rr, op := callWith(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub, "gty": []string{"password"}}))
	require.Equal(t, http.StatusOK, rr.Code)
	assert.Equal(t, f.userID, *op.User)

	rr, _ = callBody(h, idp.sign(t, jwt.MapClaims{"sub": f.userSub}), publicQuery)
	assert.Equal(t, http.StatusOK, rr.Code)

	rr, _ = callWith(h, idp.sign(t, clientClaims("svc-flow")))
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	rr, _ = callBody(h, idp.sign(t, clientClaims("svc-flow")), publicQuery)
	assert.Equal(t, http.StatusUnauthorized, rr.Code, "an unregistered client is refused even on a public operation")
}

func TestMockAuthRefusesTheActAsHeader(t *testing.T) {
	f := newServiceFixture(t)
	h := authMiddleware(&ServerConfig{Config: &Config{Mock_Auth: true}, Repos: f.repos})(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {}))
	rr, _ := callBody(h, "", publicQuery, f.actAs())
	assert.Equal(t, http.StatusUnauthorized, rr.Code)
	rr, _ = callBody(h, "", publicQuery)
	assert.Equal(t, http.StatusOK, rr.Code)
}
