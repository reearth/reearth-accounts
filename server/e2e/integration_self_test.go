package e2e

import (
	"context"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/golang-jwt/jwt/v5"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

var (
	selfOther    = id.NewIntegrationID()
	selfWsShared = id.NewWorkspaceID() // svcIntegration is reader, selfOther is writer
	selfWsOther  = id.NewWorkspaceID() // only selfOther, as owner
)

func seedIntegrationSelf(ctx context.Context, r *repo.Container) error {
	if err := seedServiceCallers(ctx, r); err != nil {
		return err
	}
	owner := map[workspace.UserID]workspace.Member{svcOwnerUID: {Role: role.RoleOwner, InvitedBy: svcOwnerUID}}
	for wid, ints := range map[id.WorkspaceID]map[workspace.IntegrationID]workspace.Member{
		selfWsShared: {
			svcIntegration: {Role: role.RoleReader, InvitedBy: svcOwnerUID},
			selfOther:      {Role: role.RoleWriter, InvitedBy: svcOwnerUID},
		},
		selfWsOther: {selfOther: {Role: role.RoleOwner, InvitedBy: svcOwnerUID}},
	} {
		ws := workspace.New().ID(wid).Name("ws-" + wid.String()).Alias("a-" + wid.String()).
			Members(owner).Integrations(ints).MustBuild()
		if err := r.Workspace.Save(ctx, ws); err != nil {
			return err
		}
	}
	return nil
}

type selfCallers struct {
	a, b, lonely, user, anonClient map[string]string
}

func startIntegrationSelfServer(t *testing.T) (*httpexpect.Expect, selfCallers) {
	t.Helper()
	e, idp := startServiceCallerServerWith(t, seedIntegrationSelf)
	tok := idp.clientToken(t, "svc-flow")
	return e, selfCallers{
		a:          actingFor(tok, "integration:"+svcIntegration.String()),
		b:          actingFor(tok, "integration:"+selfOther.String()),
		lonely:     actingFor(tok, "integration:"+svcLonely.String()),
		user:       actingFor(idp.token(t, jwt.MapClaims{"sub": "auth0|svc-owner"})),
		anonClient: actingFor(idp.clientToken(t, "svc-anon")),
	}
}

func restRoles(e *httpexpect.Expect, h map[string]string) map[string]string {
	o := e.GET("/api/integrations/self/workspaces").WithHeaders(h).
		Expect().Status(http.StatusOK).JSON().Object()
	roles := map[string]string{}
	for _, v := range o.Value("workspaces").Array().Iter() {
		w := v.Object()
		w.ContainsKey("name").ContainsKey("alias")
		roles[w.Value("id").String().Raw()] = w.Value("role").String().Raw()
	}
	return roles
}

func TestIntegrationSelf_REST(t *testing.T) {
	e, c := startIntegrationSelfServer(t)
	const path = "/api/integrations/self/workspaces"
	bRoles := map[string]string{selfWsShared.String(): "writer", selfWsOther.String(): "owner"}

	t.Run("refused callers", func(t *testing.T) {
		for _, m := range []string{http.MethodGet, http.MethodDelete} {
			e.Request(m, path).Expect().Status(http.StatusUnauthorized)
			e.Request(m, path).WithHeaders(c.user).Expect().Status(http.StatusUnauthorized)
			e.Request(m, path).WithHeaders(c.anonClient).Expect().Status(http.StatusForbidden).Body().Contains(clientGateRefusal)
		}
		assert.Len(t, restRoles(e, c.a), 3, "nothing left")
	})

	t.Run("lists the acting integration's workspaces and roles", func(t *testing.T) {
		e.GET(path).WithHeaders(c.a).Expect().Status(http.StatusOK).JSON().Object().
			HasValue("integration_id", svcIntegration.String())
		assert.Equal(t, map[string]string{
			svcWsOwned.String():    "owner",
			svcWsWritable.String(): "writer",
			selfWsShared.String():  "reader",
		}, restRoles(e, c.a))
		assert.Equal(t, bRoles, restRoles(e, c.b))
	})

	t.Run("an integration with no memberships gets empty results", func(t *testing.T) {
		e.GET(path).WithHeaders(c.lonely).Expect().Status(http.StatusOK).JSON().Object().
			HasValue("integration_id", svcLonely.String()).Value("workspaces").Array().IsEmpty()
		o := e.DELETE(path).WithHeaders(c.lonely).Expect().Status(http.StatusOK).JSON().Object()
		o.Value("removed_workspace_ids").Array().IsEmpty()
		o.Value("failed_workspace_ids").Array().IsEmpty()
	})

	t.Run("leaves only the acting integration's workspaces", func(t *testing.T) {
		o := e.DELETE(path).WithHeaders(c.a).Expect().Status(http.StatusOK).JSON().Object()
		var removed []string
		for _, v := range o.Value("removed_workspace_ids").Array().Iter() {
			removed = append(removed, v.String().Raw())
		}
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String(), selfWsShared.String()}, removed)
		o.Value("failed_workspace_ids").Array().IsEmpty()

		assert.Empty(t, restRoles(e, c.a))
		assert.Equal(t, bRoles, restRoles(e, c.b))
		e.GET("/api/workspaces/" + svcWsOwned.String()).WithHeaders(c.a).Expect().Status(http.StatusNotFound)
	})

	t.Run("a second call is a no-op", func(t *testing.T) {
		o := e.DELETE(path).WithHeaders(c.a).Expect().Status(http.StatusOK).JSON().Object()
		o.Value("removed_workspace_ids").Array().IsEmpty()
		o.Value("failed_workspace_ids").Array().IsEmpty()
	})
}

const (
	actingIntegrationQuery = `{ actingIntegration { id workspaces { role workspace { id name alias } } } }`
	leaveAllMutation       = `mutation { leaveAllWorkspaces { removedWorkspaceIds failedWorkspaceIds } }`
)

func gqlRoles(e *httpexpect.Expect, h map[string]string) map[string]string {
	ai := serviceGQL(e, h, actingIntegrationQuery, nil).Value("data").Object().Value("actingIntegration").Object()
	roles := map[string]string{}
	for _, v := range ai.Value("workspaces").Array().Iter() {
		w := v.Object()
		roles[w.Value("workspace").Object().Value("id").String().Raw()] = w.Value("role").String().Raw()
	}
	return roles
}

func gqlIDs(arr *httpexpect.Array) []string {
	ids := []string{}
	for _, v := range arr.Iter() {
		ids = append(ids, v.String().Raw())
	}
	return ids
}

func TestIntegrationSelf_GraphQL(t *testing.T) {
	e, c := startIntegrationSelfServer(t)
	bRoles := map[string]string{selfWsShared.String(): "writer", selfWsOther.String(): "owner"}

	t.Run("refused callers", func(t *testing.T) {
		for _, q := range []string{actingIntegrationQuery, leaveAllMutation} {
			e.POST("/api/graphql").WithJSON(GraphQLRequest{Query: q}).Expect().Status(http.StatusUnauthorized)
			o := e.POST("/api/graphql").WithHeaders(c.user).WithJSON(GraphQLRequest{Query: q}).
				Expect().Status(http.StatusOK).JSON().Object()
			o.Value("errors").Array().Value(0).Object().Value("message").String().Contains("invalid operator")
			refusedGQL(t, e, c.anonClient, q, nil)
		}
		assert.Len(t, gqlRoles(e, c.a), 3, "nothing left")
	})

	t.Run("lists the acting integration's workspaces and roles", func(t *testing.T) {
		serviceGQL(e, c.a, actingIntegrationQuery, nil).Value("data").Object().Value("actingIntegration").Object().
			HasValue("id", svcIntegration.String())
		assert.Equal(t, map[string]string{
			svcWsOwned.String():    "owner",
			svcWsWritable.String(): "writer",
			selfWsShared.String():  "reader",
		}, gqlRoles(e, c.a))
		assert.Equal(t, bRoles, gqlRoles(e, c.b))
		assert.Empty(t, gqlRoles(e, c.lonely))
	})

	t.Run("leaves only the acting integration's workspaces", func(t *testing.T) {
		p := serviceGQL(e, c.a, leaveAllMutation, nil).Value("data").Object().Value("leaveAllWorkspaces").Object()
		require.ElementsMatch(t, []string{svcWsOwned.String(), svcWsWritable.String(), selfWsShared.String()},
			gqlIDs(p.Value("removedWorkspaceIds").Array()))
		p.Value("failedWorkspaceIds").Array().IsEmpty()
		assert.Empty(t, gqlRoles(e, c.a))
		assert.Equal(t, bRoles, gqlRoles(e, c.b))

		again := serviceGQL(e, c.a, leaveAllMutation, nil).Value("data").Object().Value("leaveAllWorkspaces").Object()
		again.Value("removedWorkspaceIds").Array().IsEmpty()
		again.Value("failedWorkspaceIds").Array().IsEmpty()
	})
}
