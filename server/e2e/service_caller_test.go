package e2e

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

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"gopkg.in/go-jose/go-jose.v2"
)

const svcAudience = "https://accounts.e2e"

var (
	svcOwnerUID     = id.NewUserID()
	svcIntegration  = id.NewIntegrationID()
	svcLonely       = id.NewIntegrationID()
	svcWsOwned      = id.NewWorkspaceID() // integration is owner
	svcWsWritable   = id.NewWorkspaceID() // integration is writer
	svcWsNotAMember = id.NewWorkspaceID()
)

type svcIdP struct {
	key *rsa.PrivateKey
	srv *httptest.Server
}

func newSvcIdP(t *testing.T) *svcIdP {
	t.Helper()
	key, err := rsa.GenerateKey(rand.Reader, 2048)
	require.NoError(t, err)
	set := jose.JSONWebKeySet{Keys: []jose.JSONWebKey{{Key: &key.PublicKey, KeyID: "k1", Algorithm: "RS256", Use: "sig"}}}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_ = json.NewEncoder(w).Encode(set)
	}))
	t.Cleanup(srv.Close)
	return &svcIdP{key: key, srv: srv}
}

func (p *svcIdP) issuer() string { return p.srv.URL + "/" }

func (p *svcIdP) clientToken(t *testing.T, clientID string) string {
	t.Helper()
	return p.token(t, jwt.MapClaims{"sub": clientID + "@clients", "azp": clientID, "gty": "client-credentials"})
}

// token signs claims over a valid issuer, audience and lifetime; claims override them.
func (p *svcIdP) token(t *testing.T, claims jwt.MapClaims) string {
	t.Helper()
	all := jwt.MapClaims{
		"iss": p.issuer(),
		"aud": svcAudience,
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	}
	for k, v := range claims {
		all[k] = v
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, all)
	tok.Header["kid"] = "k1"
	s, err := tok.SignedString(p.key)
	require.NoError(t, err)
	return s
}

func seedServiceCallers(ctx context.Context, r *repo.Container) error {
	u := user.New().ID(svcOwnerUID).Name("svc-owner").Email("svc-owner@e2e.com").
		Auths([]user.Auth{{Provider: "auth0", Sub: "auth0|svc-owner"}}).MustBuild()
	if err := r.User.Save(ctx, u); err != nil {
		return err
	}
	owner := map[workspace.UserID]workspace.Member{svcOwnerUID: {Role: role.RoleOwner, InvitedBy: svcOwnerUID}}
	for wid, r2 := range map[id.WorkspaceID]role.RoleType{svcWsOwned: role.RoleOwner, svcWsWritable: role.RoleWriter, svcWsNotAMember: ""} {
		b := workspace.New().ID(wid).Name("ws-" + wid.String()).Alias("a-" + wid.String()).Members(owner)
		if r2 != "" {
			b = b.Integrations(map[workspace.IntegrationID]workspace.Member{svcIntegration: {Role: r2, InvitedBy: svcOwnerUID}})
		}
		if err := r.Workspace.Save(ctx, b.MustBuild()); err != nil {
			return err
		}
	}
	return nil
}

func startServiceCallerServer(t *testing.T) (*httpexpect.Expect, *svcIdP) {
	t.Helper()
	idp := newSvcIdP(t)
	jwks := idp.srv.URL + "/jwks.json"
	cfg := &app.Config{Auth: app.AuthConfigs{{ISS: idp.issuer(), AUD: []string{svcAudience}, JWKSURI: &jwks}}}
	callers, err := json.Marshal([]map[string]any{
		{"issuer": idp.issuer(), "client_id": "svc-flow", "name": "flow-app", "act_as": []string{"integration", "user"}},
		{"issuer": idp.issuer(), "client_id": "svc-users", "name": "users-app", "act_as": []string{"user"}},
		{"issuer": idp.issuer(), "client_id": "svc-anon", "name": "anon-app", "act_as": []string{}},
	})
	require.NoError(t, err)
	require.NoError(t, cfg.ServiceCallers.Decode(string(callers)))
	e, _ := StartServerNoDebug(t, cfg, false, seedServiceCallers)
	return e, idp
}

func idsOf(arr *httpexpect.Array) []string {
	var ids []string
	for _, v := range arr.Iter() {
		ids = append(ids, v.Object().Value("id").String().Raw())
	}
	return ids
}

func TestServiceCaller_REST(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	flow := actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcIntegration.String())
	lonely := actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcLonely.String())

	t.Run("read is scoped to the integration's workspaces", func(t *testing.T) {
		arr := e.GET("/api/workspaces").WithQuery("user_id", svcOwnerUID.String()).
			WithHeaders(flow).
			Expect().Status(http.StatusOK).JSON().Array()
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, idsOf(arr))
	})

	t.Run("write allowed by the owner role", func(t *testing.T) {
		e.PATCH("/api/workspaces/"+svcWsOwned.String()).WithHeaders(flow).
			WithJSON(map[string]any{"name": "renamed by service"}).
			Expect().Status(http.StatusOK).JSON().Object().HasValue("name", "renamed by service")
	})

	t.Run("write denied by the writer role", func(t *testing.T) {
		e.PATCH("/api/workspaces/" + svcWsWritable.String()).WithHeaders(flow).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("write denied outside its memberships", func(t *testing.T) {
		e.PATCH("/api/workspaces/" + svcWsNotAMember.String()).WithHeaders(flow).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("service with no membership sees nothing", func(t *testing.T) {
		e.GET("/api/workspaces").WithQuery("user_id", svcOwnerUID.String()).
			WithHeaders(lonely).
			Expect().Status(http.StatusOK).JSON().Array().IsEmpty()
		e.PATCH("/api/workspaces/" + svcWsOwned.String()).WithHeaders(lonely).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("every workspace read path is scoped", func(t *testing.T) {
		get := func(auth map[string]string, path string, q map[string]string) *httpexpect.Response {
			r := e.GET(path).WithHeaders(auth)
			for k, v := range q {
				r = r.WithQuery(k, v)
			}
			return r.Expect()
		}
		get(flow, "/api/workspaces/"+svcWsOwned.String(), nil).Status(http.StatusOK)
		get(flow, "/api/workspaces/"+svcWsNotAMember.String(), nil).Status(http.StatusNotFound)
		get(lonely, "/api/workspaces/"+svcWsOwned.String(), map[string]string{"expand": "members.user"}).Status(http.StatusNotFound)
		get(lonely, "/api/workspaces", map[string]string{"name": "ws-" + svcWsOwned.String()}).Status(http.StatusNotFound)
		get(lonely, "/api/workspaces", map[string]string{"alias": "a-" + svcWsOwned.String()}).Status(http.StatusNotFound)
		get(lonely, "/api/workspaces", map[string]string{"ids": svcWsOwned.String() + "," + svcWsNotAMember.String()}).
			Status(http.StatusOK).JSON().Array().IsEmpty()
		ids := get(flow, "/api/workspaces", map[string]string{"ids": svcWsOwned.String() + "," + svcWsNotAMember.String()}).
			Status(http.StatusOK).JSON().Array()
		require.Equal(t, []string{svcWsOwned.String()}, idsOf(ids))

		page := get(flow, "/api/workspaces", map[string]string{"user_id": svcOwnerUID.String(), "page": "1", "page_size": "1"}).
			Status(http.StatusOK).JSON().Object()
		page.Value("items").Array().Length().IsEqual(1)
		page.Value("pagination").Object().HasValue("total", 2)
		get(flow, "/api/workspaces", map[string]string{"user_id": svcOwnerUID.String(), "page": "184467440737095518", "page_size": "50"}).
			Status(http.StatusOK).JSON().Object().Value("items").Array().IsEmpty()
		get(lonely, "/api/workspaces", map[string]string{"user_id": svcOwnerUID.String(), "page": "1"}).
			Status(http.StatusOK).JSON().Object().Value("pagination").Object().HasValue("total", 0)
	})

	t.Run("the user directory stays closed to an integration", func(t *testing.T) {
		for path, q := range map[string][2]string{
			"/api/users/me":                      {},
			"/api/users/search":                  {"keyword", "svc"},
			"/api/users/by-name-or-email":        {"nameOrEmail", "svc-owner"},
			"/api/users/" + svcOwnerUID.String(): {},
			"/api/users":                         {"ids", svcOwnerUID.String()},
		} {
			r := e.GET(path).WithHeaders(flow)
			if q[0] != "" {
				r = r.WithQuery(q[0], q[1])
			}
			r.Expect().Status(http.StatusForbidden)
		}
	})

}

// actingFor is the header set of a client call; an empty actAs sends no act-as header.
func actingFor(token string, actAs ...string) map[string]string {
	h := map[string]string{"Authorization": "Bearer " + token}
	if len(actAs) > 0 {
		h[app.ActAsHeader] = actAs[0]
	}
	return h
}

func serviceGQL(e *httpexpect.Expect, auth map[string]string, query string, vars map[string]any) *httpexpect.Object {
	return e.POST("/api/graphql").WithHeaders(auth).
		WithJSON(GraphQLRequest{Query: query, Variables: vars}).
		Expect().Status(http.StatusOK).JSON().Object()
}

func TestServiceCaller_GraphQL(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	flow := actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcIntegration.String())
	const findByUser = `query($u: ID!) { findByUser(userId: $u) { id } }`
	const rename = `mutation($w: ID!, $n: String!) { updateWorkspace(input: {workspaceId: $w, name: $n}) { workspace { id name } } }`

	t.Run("read is scoped to the integration's workspaces", func(t *testing.T) {
		arr := serviceGQL(e, flow, findByUser, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUser").Array()
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, idsOf(arr))
	})

	t.Run("paginated read is scoped", func(t *testing.T) {
		const q = `query($u: ID!) { findByUserWithPagination(userId: $u, pagination: {page: 1, size: 10}) { totalCount } }`
		serviceGQL(e, flow, q, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUserWithPagination").Object().HasValue("totalCount", 2)
		serviceGQL(e, actingFor(idp.clientToken(t, "svc-flow"), "integration:"+svcLonely.String()), q, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUserWithPagination").Object().HasValue("totalCount", 0)
	})

	t.Run("the user directory stays closed to an integration", func(t *testing.T) {
		for _, q := range []string{
			`{ searchUser(keyword: "svc") { id } }`,
			`query($u: ID!) { user(id: $u) { id } }`,
			`{ userByNameOrEmail(nameOrEmail: "svc-owner") { id } }`,
			`query($u: ID!) { findUsersByIDs(ids: [$u]) { id } }`,
		} {
			refusedGQL(t, e, flow, q, map[string]any{"u": svcOwnerUID.String()})
		}
	})

	t.Run("write allowed by the owner role", func(t *testing.T) {
		serviceGQL(e, flow, rename, map[string]any{"w": svcWsOwned.String(), "n": "gql renamed"}).
			Value("data").Object().Value("updateWorkspace").Object().
			Value("workspace").Object().HasValue("name", "gql renamed")
	})

	t.Run("write denied by the writer role", func(t *testing.T) {
		serviceGQL(e, flow, rename, map[string]any{"w": svcWsWritable.String(), "n": "nope"}).
			Value("errors").Array().NotEmpty()
	})

}

// refusedGQL asserts the client field gate refused the whole operation.
func refusedGQL(t *testing.T, e *httpexpect.Expect, auth map[string]string, query string, vars map[string]any) {
	t.Helper()
	o := e.POST("/api/graphql").WithHeaders(auth).
		WithJSON(GraphQLRequest{Query: query, Variables: vars}).
		Expect().Status(http.StatusForbidden).JSON().Object()
	assert.Nil(t, o.Raw()["data"], "no resolver ran")
	o.Value("errors").Array().Value(0).Object().Value("extensions").Object().HasValue("code", "FORBIDDEN")
}

const publicGQL = `{ authConfig { __typename } }`

// optionalREST is a route that takes an anonymous caller: an empty body is a
// 400 from the handler, so a 401 can only come from the auth layer.
func optionalREST(e *httpexpect.Expect, h map[string]string) *httpexpect.Response {
	return e.POST("/api/users/verifications").WithHeaders(h).WithJSON(map[string]any{}).Expect()
}

func publicGraphQL(e *httpexpect.Expect, h map[string]string) *httpexpect.Response {
	return e.POST("/api/graphql").WithHeaders(h).WithJSON(GraphQLRequest{Query: publicGQL}).Expect()
}

func TestServiceCaller_RefusedRequests(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	other := newSvcIdP(t)
	flowTok := idp.clientToken(t, "svc-flow")
	asInt := "integration:" + svcIntegration.String()
	asUser := "user:" + svcOwnerUID.String()
	userTok := idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"})
	clientClaims := func(extra jwt.MapClaims) jwt.MapClaims {
		c := jwt.MapClaims{"sub": "svc-flow@clients", "azp": "svc-flow", "gty": "client-credentials"}
		for k, v := range extra {
			c[k] = v
		}
		return c
	}

	refused := map[string]map[string]string{
		"user token with the header":               actingFor(userTok, asInt),
		"header with no token":                     {app.ActAsHeader: asInt},
		"empty header":                             actingFor(flowTok, ""),
		"malformed header":                         actingFor(flowTok, "integration:nope"),
		"two colons":                               actingFor(flowTok, asInt+":x"),
		"upper-case kind":                          actingFor(flowTok, "INTEGRATION:"+svcIntegration.String()),
		"unknown kind":                             actingFor(flowTok, "team:"+svcIntegration.String()),
		"kind not in act_as":                       actingFor(idp.clientToken(t, "svc-anon"), asInt),
		"integration for a user-only client":       actingFor(idp.clientToken(t, "svc-users"), asInt),
		"user subject":                             actingFor(flowTok, asUser),
		"user subject for a user-only client":      actingFor(idp.clientToken(t, "svc-users"), asUser),
		"unregistered client":                      actingFor(idp.clientToken(t, "svc-unknown"), asInt),
		"unregistered client with no header":       actingFor(idp.clientToken(t, "svc-unknown")),
		"client token from an unconfigured issuer": actingFor(other.token(t, jwt.MapClaims{"iss": other.issuer(), "sub": "svc-flow@clients", "azp": "svc-flow", "gty": "client-credentials"}), asInt),
		"client token with the wrong audience":     actingFor(idp.token(t, clientClaims(jwt.MapClaims{"aud": "https://elsewhere.example.com"})), asInt),
		"expired client token":                     actingFor(idp.token(t, clientClaims(jwt.MapClaims{"iat": time.Now().Add(-2 * time.Hour).Unix(), "exp": time.Now().Add(-time.Hour).Unix()})), asInt),
	}
	for name, h := range refused {
		t.Run(name+" is 401 on REST", func(t *testing.T) {
			optionalREST(e, h).Status(http.StatusUnauthorized)
			e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(h).Expect().Status(http.StatusUnauthorized)
		})
		t.Run(name+" is 401 on GraphQL", func(t *testing.T) {
			publicGraphQL(e, h).Status(http.StatusUnauthorized)
		})
	}

	t.Run("a repeated header is 401", func(t *testing.T) {
		repeat := func(r *httpexpect.Request) *httpexpect.Request {
			return r.WithHeader("Authorization", "Bearer "+flowTok).WithHeader(app.ActAsHeader, asInt).WithHeader(app.ActAsHeader, asInt)
		}
		repeat(e.POST("/api/users/verifications")).WithJSON(map[string]any{}).Expect().Status(http.StatusUnauthorized)
		repeat(e.POST("/api/graphql")).WithJSON(GraphQLRequest{Query: publicGQL}).Expect().Status(http.StatusUnauthorized)
	})

	t.Run("the same routes without a refused header", func(t *testing.T) {
		optionalREST(e, map[string]string{}).Status(http.StatusBadRequest)
		publicGraphQL(e, map[string]string{}).Status(http.StatusOK)
		e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(actingFor(flowTok, asInt)).Expect().Status(http.StatusOK)
	})
}

func TestServiceCaller_ClientTokenWithoutHeaderIsAnonymous(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	for _, cid := range []string{"svc-flow", "svc-anon"} {
		h := actingFor(idp.clientToken(t, cid))
		t.Run(cid+" on REST", func(t *testing.T) {
			e.GET("/api/auth/config").WithHeaders(h).Expect().Status(http.StatusOK)
			optionalREST(e, h).Status(http.StatusForbidden)
			e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(h).Expect().Status(http.StatusForbidden)
			e.GET("/api/workspaces/" + svcWsOwned.String()).Expect().Status(http.StatusUnauthorized)
		})
		t.Run(cid+" on GraphQL", func(t *testing.T) {
			publicGraphQL(e, h).Status(http.StatusOK).JSON().Object().Value("data").Object().ContainsKey("authConfig")
			byName := GraphQLRequest{Query: `query($n: String!) { findByName(name: $n) { id } }`, Variables: map[string]any{"n": "ws-" + svcWsOwned.String()}}
			refusedGQL(t, e, h, byName.Query, byName.Variables)
			e.POST("/api/graphql").WithJSON(byName).Expect().Status(http.StatusUnauthorized)
		})
	}
}

func TestServiceCaller_UserTokenStillWorks(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	h := actingFor(idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"}))

	e.GET("/api/users/me").WithHeaders(h).Expect().Status(http.StatusOK).JSON().Object().HasValue("id", svcOwnerUID.String())
	serviceGQL(e, h, `{ me { id } }`, nil).Value("data").Object().Value("me").Object().HasValue("id", svcOwnerUID.String())
	arr := serviceGQL(e, h, `query($u: ID!) { findByUser(userId: $u) { id } }`, map[string]any{"u": svcOwnerUID.String()}).
		Value("data").Object().Value("findByUser").Array()
	require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String(), svcWsNotAMember.String()}, idsOf(arr))
}

func TestServiceCaller_MockAuthRefusesTheHeader(t *testing.T) {
	e, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seedDemoUser)
	h := map[string]string{app.ActAsHeader: "integration:" + svcIntegration.String()}

	e.GET("/api/users/me").Expect().Status(http.StatusOK)
	e.GET("/api/users/me").WithHeaders(h).Expect().Status(http.StatusUnauthorized)
	e.POST("/api/graphql").WithJSON(GraphQLRequest{Query: `{ me { id } }`}).Expect().Status(http.StatusOK)
	e.POST("/api/graphql").WithHeaders(h).WithJSON(GraphQLRequest{Query: `{ me { id } }`}).Expect().Status(http.StatusUnauthorized)
	publicGraphQL(e, h).Status(http.StatusUnauthorized)
}

func TestServiceCaller_LogsWhoCalled(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	logs := captureLogs(t)
	tok := idp.clientToken(t, "svc-flow")
	h := actingFor(tok, "integration:"+svcIntegration.String())
	const probe = "integration:act-as-probe-value"

	e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(h).Expect().Status(http.StatusOK)
	serviceGQL(e, h, `query($u: ID!) { findByUser(userId: $u) { id } }`, map[string]any{"u": svcOwnerUID.String()})
	optionalREST(e, actingFor(tok, probe)).Status(http.StatusUnauthorized)

	out := logs.String()
	want := []string{
		`"caller_client_id": "svc-flow"`,
		`"caller_name": "flow-app"`,
		`"acting_for": "integration:` + svcIntegration.String() + `"`,
	}
	var accepted []string
	for _, l := range strings.Split(out, "\n") {
		if strings.Contains(l, "auth: client call") {
			accepted = append(accepted, l)
		}
	}
	require.Len(t, accepted, 2, out)
	entries := requestEntries(out)
	require.Len(t, entries, 3, out)
	for _, l := range append(accepted, entries[:2]...) {
		for _, w := range want {
			assert.Contains(t, l, w)
		}
	}
	assert.Contains(t, out, `"reason": "malformed_act_as"`)
	assert.NotContains(t, out, tok)
	assert.NotContains(t, out, "act-as-probe-value")
	assert.NotContains(t, out, "svc-owner@e2e.com")
}

func TestServiceCaller_EmptyRegistryRefusesClientTokens(t *testing.T) {
	idp := newSvcIdP(t)
	jwks := idp.srv.URL + "/jwks.json"
	e, _ := StartServerNoDebug(t, &app.Config{Auth: app.AuthConfigs{{ISS: idp.issuer(), AUD: []string{svcAudience}, JWKSURI: &jwks}}}, false, seedServiceCallers)
	logs := captureLogs(t)
	tok := idp.clientToken(t, "svc-flow")

	for name, h := range map[string]map[string]string{
		"without the header": actingFor(tok),
		"with the header":    actingFor(tok, "integration:"+svcIntegration.String()),
	} {
		t.Run(name+" on REST", func(t *testing.T) {
			optionalREST(e, h).Status(http.StatusUnauthorized)
		})
		t.Run(name+" on GraphQL", func(t *testing.T) {
			publicGraphQL(e, h).Status(http.StatusUnauthorized)
		})
	}
	assert.Equal(t, 4, strings.Count(logs.String(), `"reason": "unregistered_client"`), logs.String())
	assert.NotContains(t, logs.String(), "act_as_without_client_token")

	userTok := actingFor(idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"}))
	e.GET("/api/users/me").WithHeaders(userTok).Expect().Status(http.StatusOK)
	publicGraphQL(e, userTok).Status(http.StatusOK)
}

func TestServiceCaller_RequestLogNamesTheClient(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	logs := captureLogs(t)
	flow := idp.clientToken(t, "svc-flow")
	unknown := idp.clientToken(t, "svc-unknown")
	asInt := "integration:" + svcIntegration.String()
	findByName := GraphQLRequest{Query: `query($n: String!) { findByName(name: $n) { id } }`, Variables: map[string]any{"n": "ws-" + svcWsOwned.String()}}

	named := []string{`"caller_client_id": "svc-flow"`, `"caller_name": "flow-app"`}
	for _, tc := range []struct {
		name   string
		do     func()
		line   []string
		reason string
	}{
		{
			name: "integration call on an open REST route",
			do: func() {
				e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(actingFor(flow, asInt)).Expect().Status(http.StatusOK)
			},
			line: append(named, `"acting_for": "`+asInt+`"`),
		},
		{
			name: "anonymous call on an open REST route",
			do:   func() { e.GET("/api/auth/config").WithHeaders(actingFor(flow)).Expect().Status(http.StatusOK) },
			line: append(named, `"acting_for": "none"`),
		},
		{
			name: "anonymous call on a public GraphQL operation",
			do:   func() { publicGraphQL(e, actingFor(flow)).Status(http.StatusOK) },
			line: append(named, `"acting_for": "none"`),
		},
		{
			name:   "integration call refused by the REST gate",
			do:     func() { optionalREST(e, actingFor(flow, asInt)).Status(http.StatusForbidden) },
			line:   append(named, `"acting_for": "`+asInt+`"`),
			reason: "not_open_to_client",
		},
		{
			name: "integration call refused by the GraphQL gate",
			do: func() {
				e.POST("/api/graphql").WithHeaders(actingFor(flow, asInt)).WithJSON(GraphQLRequest{Query: `{ me { id } }`}).Expect().Status(http.StatusForbidden)
			},
			line:   append(named, `"acting_for": "`+asInt+`"`),
			reason: "not_open_to_client",
		},
		{
			name: "anonymous call refused by the REST gate",
			do: func() {
				e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(actingFor(flow)).Expect().Status(http.StatusForbidden)
			},
			line:   append(named, `"acting_for": "none"`),
			reason: "not_open_to_client",
		},
		{
			name: "anonymous call refused on GraphQL",
			do: func() {
				e.POST("/api/graphql").WithHeaders(actingFor(flow)).WithJSON(findByName).Expect().Status(http.StatusForbidden)
			},
			line:   append(named, `"acting_for": "none"`),
			reason: "not_open_to_client",
		},
		{
			name:   "refused call on REST",
			do:     func() { optionalREST(e, actingFor(unknown, asInt)).Status(http.StatusUnauthorized) },
			line:   []string{`"caller_client_id": "svc-unknown"`},
			reason: "unregistered_client",
		},
		{
			name:   "refused call on GraphQL",
			do:     func() { publicGraphQL(e, actingFor(unknown, asInt)).Status(http.StatusUnauthorized) },
			line:   []string{`"caller_client_id": "svc-unknown"`},
			reason: "unregistered_client",
		},
	} {
		t.Run(tc.name, func(t *testing.T) {
			before := len(logs.String())
			tc.do()
			out := logs.String()[before:]
			entries := requestEntries(out)
			require.Len(t, entries, 1, out)
			for _, w := range tc.line {
				assert.Contains(t, entries[0], w)
			}
			var verdicts []string
			for _, l := range strings.Split(out, "\n") {
				if strings.Contains(l, "\tauth: client call\t") || strings.Contains(l, "\tauth: rejected client call\t") {
					verdicts = append(verdicts, l)
				}
			}
			require.Len(t, verdicts, 1, out)
			for _, w := range tc.line {
				assert.Contains(t, verdicts[0], w)
			}
			if tc.reason == "" {
				assert.Contains(t, verdicts[0], "\tauth: client call\t")
			} else {
				assert.Contains(t, verdicts[0], "\tauth: rejected client call\t")
				assert.Contains(t, verdicts[0], `"reason": "`+tc.reason+`"`)
			}
			assert.NotContains(t, out, flow)
			assert.NotContains(t, out, unknown)
		})
	}
}
