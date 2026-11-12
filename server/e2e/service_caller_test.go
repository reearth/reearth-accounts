package e2e

import (
	"context"
	"crypto/rand"
	"crypto/rsa"
	"encoding/json"
	"net/http"
	"net/http/httptest"
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
	tok := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss": p.issuer(),
		"aud": svcAudience,
		"sub": clientID + "@clients",
		"azp": clientID,
		"gty": "client-credentials",
		"iat": time.Now().Unix(),
		"exp": time.Now().Add(time.Hour).Unix(),
	})
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
	callers, err := json.Marshal([]map[string]string{
		{"issuer": idp.issuer(), "client_id": "svc-flow", "integration_id": svcIntegration.String()},
		{"issuer": idp.issuer(), "client_id": "svc-lonely", "integration_id": svcLonely.String()},
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
	flow := "Bearer " + idp.clientToken(t, "svc-flow")
	lonely := "Bearer " + idp.clientToken(t, "svc-lonely")

	t.Run("read is scoped to the integration's workspaces", func(t *testing.T) {
		arr := e.GET("/api/workspaces").WithQuery("user_id", svcOwnerUID.String()).
			WithHeader("Authorization", flow).
			Expect().Status(http.StatusOK).JSON().Array()
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, idsOf(arr))
	})

	t.Run("write allowed by the owner role", func(t *testing.T) {
		e.PATCH("/api/workspaces/"+svcWsOwned.String()).WithHeader("Authorization", flow).
			WithJSON(map[string]any{"name": "renamed by service"}).
			Expect().Status(http.StatusOK).JSON().Object().HasValue("name", "renamed by service")
	})

	t.Run("write denied by the writer role", func(t *testing.T) {
		e.PATCH("/api/workspaces/"+svcWsWritable.String()).WithHeader("Authorization", flow).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("write denied outside its memberships", func(t *testing.T) {
		e.PATCH("/api/workspaces/"+svcWsNotAMember.String()).WithHeader("Authorization", flow).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("service with no membership sees nothing", func(t *testing.T) {
		e.GET("/api/workspaces").WithQuery("user_id", svcOwnerUID.String()).
			WithHeader("Authorization", lonely).
			Expect().Status(http.StatusOK).JSON().Array().IsEmpty()
		e.PATCH("/api/workspaces/"+svcWsOwned.String()).WithHeader("Authorization", lonely).
			WithJSON(map[string]any{"name": "nope"}).
			Expect().Status(http.StatusForbidden)
	})

	t.Run("every workspace read path is scoped", func(t *testing.T) {
		get := func(auth, path string, q map[string]string) *httpexpect.Response {
			r := e.GET(path).WithHeader("Authorization", auth)
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

	t.Run("the user directory stays closed to a service", func(t *testing.T) {
		for path, q := range map[string][2]string{
			"/api/users/me":                      {},
			"/api/users/search":                  {"keyword", "svc"},
			"/api/users/by-name-or-email":        {"nameOrEmail", "svc-owner"},
			"/api/users/" + svcOwnerUID.String(): {},
			"/api/users":                         {"ids", svcOwnerUID.String()},
		} {
			r := e.GET(path).WithHeader("Authorization", flow)
			if q[0] != "" {
				r = r.WithQuery(q[0], q[1])
			}
			r.Expect().Status(http.StatusUnauthorized)
		}
	})

	t.Run("unregistered client is 401", func(t *testing.T) {
		e.GET("/api/workspaces").WithQuery("user_id", svcOwnerUID.String()).
			WithHeader("Authorization", "Bearer "+idp.clientToken(t, "svc-unknown")).
			Expect().Status(http.StatusUnauthorized)
	})
}

func gql(e *httpexpect.Expect, auth, query string, vars map[string]any) *httpexpect.Object {
	return e.POST("/api/graphql").WithHeader("Authorization", auth).
		WithJSON(GraphQLRequest{Query: query, Variables: vars}).
		Expect().Status(http.StatusOK).JSON().Object()
}

func TestServiceCaller_GraphQL(t *testing.T) {
	e, idp := startServiceCallerServer(t)
	flow := "Bearer " + idp.clientToken(t, "svc-flow")
	const findByUser = `query($u: ID!) { findByUser(userId: $u) { id } }`
	const rename = `mutation($w: ID!, $n: String!) { updateWorkspace(input: {workspaceId: $w, name: $n}) { workspace { id name } } }`

	t.Run("read is scoped to the integration's workspaces", func(t *testing.T) {
		arr := gql(e, flow, findByUser, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUser").Array()
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String()}, idsOf(arr))
	})

	t.Run("paginated read is scoped", func(t *testing.T) {
		const q = `query($u: ID!) { findByUserWithPagination(userId: $u, pagination: {page: 1, size: 10}) { totalCount } }`
		gql(e, flow, q, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUserWithPagination").Object().HasValue("totalCount", 2)
		gql(e, "Bearer "+idp.clientToken(t, "svc-lonely"), q, map[string]any{"u": svcOwnerUID.String()}).
			Value("data").Object().Value("findByUserWithPagination").Object().HasValue("totalCount", 0)
	})

	t.Run("the user directory stays closed to a service", func(t *testing.T) {
		for _, q := range []string{
			`{ searchUser(keyword: "svc") { id } }`,
			`query($u: ID!) { user(id: $u) { id } }`,
			`{ userByNameOrEmail(nameOrEmail: "svc-owner") { id } }`,
			`query($u: ID!) { findUsersByIDs(ids: [$u]) { id } }`,
		} {
			gql(e, flow, q, map[string]any{"u": svcOwnerUID.String()}).Value("errors").Array().NotEmpty()
		}
	})

	t.Run("write allowed by the owner role", func(t *testing.T) {
		gql(e, flow, rename, map[string]any{"w": svcWsOwned.String(), "n": "gql renamed"}).
			Value("data").Object().Value("updateWorkspace").Object().
			Value("workspace").Object().HasValue("name", "gql renamed")
	})

	t.Run("write denied by the writer role", func(t *testing.T) {
		gql(e, flow, rename, map[string]any{"w": svcWsWritable.String(), "n": "nope"}).
			Value("errors").Array().NotEmpty()
	})

	t.Run("unregistered client is 401", func(t *testing.T) {
		e.POST("/api/graphql").WithHeader("Authorization", "Bearer "+idp.clientToken(t, "svc-unknown")).
			WithJSON(GraphQLRequest{Query: findByUser, Variables: map[string]any{"u": svcOwnerUID.String()}}).
			Expect().Status(http.StatusUnauthorized)
	})
}
