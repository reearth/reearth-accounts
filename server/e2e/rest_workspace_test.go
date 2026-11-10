package e2e

import (
	"context"
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/idx"
	"github.com/stretchr/testify/assert"
)

func TestREST_WorkspaceCRUD(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)

	// Create
	wsObj := exp.POST("/api/workspaces").
		WithJSON(map[string]any{"alias": "team-alpha", "name": "Team Alpha"}).
		Expect().Status(http.StatusOK).JSON().Object()
	wsObj.HasValue("name", "Team Alpha")
	wsObj.HasValue("alias", "team-alpha")
	wid := wsObj.Value("id").String().Raw()

	// Get
	exp.GET("/api/workspaces/"+wid).
		Expect().Status(http.StatusOK).JSON().Object().HasValue("id", wid)

	// Add a second seeded user as writer.
	addObj := exp.POST("/api/workspaces/" + wid + "/members").
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": restOtherUID.String(), "role": "writer"},
		}}).
		Expect().Status(http.StatusOK).JSON().Object()
	addObj.Value("members").Array().Length().IsEqual(2)

	// Delete
	exp.DELETE("/api/workspaces/" + wid).Expect().Status(http.StatusNoContent)

	// Confirm gone
	exp.GET("/api/workspaces/" + wid).Expect().Status(http.StatusNotFound)
}

func TestREST_WorkspaceSeveralOwners(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)

	wid := exp.POST("/api/workspaces").
		WithJSON(map[string]any{"alias": "team-owners", "name": "Team Owners"}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("id").String().Raw()

	members := exp.POST("/api/workspaces/" + wid + "/members").
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": restOtherUID.String(), "role": "owner"},
		}}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Raw()

	roles := map[string]any{}
	for _, m := range members {
		member := m.(map[string]any)
		roles[member["user_id"].(string)] = member["role"]
	}
	assert.Equal(t, map[string]any{
		restDemoUID.String():  "owner",
		restOtherUID.String(): "owner",
	}, roles)
}

func TestREST_WorkspaceListByUser(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/workspaces").WithQuery("user_id", restDemoUID.String()).
		Expect().Status(http.StatusOK).JSON().Array().NotEmpty()
}

func TestREST_WorkspaceDropsMissingUserMembers(t *testing.T) {
	wid := id.NewWorkspaceID()
	ghostUID := id.NewUserID()
	iid := id.NewIntegrationID()
	seed := func(ctx context.Context, r *repo.Container) error {
		if err := seedDemoUser(ctx, r); err != nil {
			return err
		}
		return r.Workspace.Save(ctx, workspace.New().ID(wid).
			Name("Ghost Team").
			Alias("ghost-team").
			Members(map[idx.ID[id.User]]workspace.Member{
				restDemoUID: {Role: role.RoleOwner, InvitedBy: restDemoUID},
				ghostUID:    {Role: role.RoleWriter, InvitedBy: restDemoUID},
			}).
			Integrations(map[idx.ID[id.Integration]]workspace.Member{
				iid: {Role: role.RoleReader, InvitedBy: restDemoUID},
			}).
			MustBuild())
	}
	exp, _ := StartServer(t, &app.Config{Mock_Auth: true}, false, seed)

	want := []map[string]any{
		{"user_id": restDemoUID.String(), "role": "owner"},
		{"integration_id": iid.String(), "role": "reader"},
	}

	got := exp.GET("/api/workspaces/" + wid.String()).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Raw()
	assert.ElementsMatch(t, want, membersOf(got))

	list := exp.GET("/api/workspaces").WithQuery("user_id", restDemoUID.String()).
		Expect().Status(http.StatusOK).JSON().Array().Raw()
	found := false
	for _, w := range list {
		ws := w.(map[string]any)
		if ws["id"] == wid.String() {
			found = true
			assert.ElementsMatch(t, want, membersOf(ws["members"].([]any)))
		}
	}
	assert.True(t, found, "expected the workspace in the user's list")
}

func membersOf(raw []any) []map[string]any {
	out := make([]map[string]any, 0, len(raw))
	for _, m := range raw {
		out = append(out, m.(map[string]any))
	}
	return out
}

func TestREST_WorkspaceListBadSelector(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	// No selector (or more than one) -> 400.
	exp.GET("/api/workspaces").Expect().Status(http.StatusBadRequest)
}

func TestREST_WorkspaceUnauthorized(t *testing.T) {
	cfg := &app.Config{} // non-mock, no token
	exp, _ := StartServer(t, cfg, false, nil)
	exp.POST("/api/workspaces").
		WithJSON(map[string]any{"alias": "x", "name": "X"}).
		Expect().Status(http.StatusUnauthorized)
}

// --- Mock_Auth=false (real JWT pipeline) variants ---

const realJWTWorkspaceSub = "test|realjwt-workspace"

func TestREST_RealJWT_WorkspaceCRUD(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUsers(realJWTWorkspaceSub))
	token := signTestToken(t, key, realJWTWorkspaceSub)
	bearer := "Bearer " + token

	// Create
	wsObj := exp.POST("/api/workspaces").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"alias": "jwt-team", "name": "JWT Team"}).
		Expect().Status(http.StatusOK).JSON().Object()
	wsObj.HasValue("name", "JWT Team")
	wid := wsObj.Value("id").String().Raw()

	// Get
	exp.GET("/api/workspaces/"+wid).
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusOK).JSON().Object().HasValue("id", wid)

	// Add the second seeded user as writer.
	exp.POST("/api/workspaces/"+wid+"/members").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": jwtSecondUID.String(), "role": "writer"},
		}}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Length().IsEqual(2)

	// Delete
	exp.DELETE("/api/workspaces/"+wid).
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusNoContent)

	// Confirm gone
	exp.GET("/api/workspaces/"+wid).
		WithHeader("Authorization", bearer).
		Expect().Status(http.StatusNotFound)
}

func TestREST_RealJWT_WorkspaceListByUser(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUsers(realJWTWorkspaceSub))
	token := signTestToken(t, key, realJWTWorkspaceSub)

	exp.GET("/api/workspaces").WithQuery("user_id", jwtPrimaryUID.String()).
		WithHeader("Authorization", "Bearer "+token).
		Expect().Status(http.StatusOK).JSON().Array().NotEmpty()
}

// TestREST_RealJWT_ServiceMemberUpdate_BypassesSelfPromotionGuard covers PATCH
// /api/service/workspaces/:id/members/:user_id: JWT-authenticated like the
// regular member route, and a Maintainer/Owner may use it to change their own
// role, as long as it isn't the workspace's last remaining Owner (blocked on
// both routes).
func TestREST_RealJWT_ServiceMemberUpdate_BypassesSelfPromotionGuard(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUsers(realJWTWorkspaceSub))
	token := signTestToken(t, key, realJWTWorkspaceSub)
	bearer := "Bearer " + token

	wsObj := exp.POST("/api/workspaces").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"alias": "service-role-team", "name": "Service Role Team"}).
		Expect().Status(http.StatusOK).JSON().Object()
	wid := wsObj.Value("id").String().Raw()

	// No JWT at all is rejected, same as any other JWT-required route.
	exp.PATCH("/api/service/workspaces/" + wid + "/members/" + jwtPrimaryUID.String()).
		WithJSON(map[string]any{"role": "maintainer"}).
		Expect().Status(http.StatusUnauthorized)

	// jwtPrimaryUID is currently the sole Owner: demoting them (self or via the
	// service route) is blocked everywhere, since it would leave the workspace
	// ownerless.
	exp.PATCH("/api/service/workspaces/"+wid+"/members/"+jwtPrimaryUID.String()).
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"role": "maintainer"}).
		Expect().Status(http.StatusForbidden)

	// The service route never grants Owner.
	exp.PATCH("/api/service/workspaces/"+wid+"/members/"+jwtPrimaryUID.String()).
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"role": "owner"}).
		Expect().Status(http.StatusBadRequest)

	// The owner makes jwtSecondUID a second owner through the regular route.
	exp.POST("/api/workspaces/"+wid+"/members").
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": jwtSecondUID.String(), "role": "owner"},
		}}).
		Expect().Status(http.StatusOK)

	// jwtSecondUID remains Owner, so stepping down is now allowed.
	members := exp.PATCH("/api/service/workspaces/"+wid+"/members/"+jwtPrimaryUID.String()).
		WithHeader("Authorization", bearer).
		WithJSON(map[string]any{"role": "maintainer"}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Raw()

	found := false
	for _, m := range members {
		member, ok := m.(map[string]any)
		if ok && member["user_id"] == jwtPrimaryUID.String() {
			found = true
			assert.Equal(t, "maintainer", member["role"])
		}
	}
	assert.True(t, found, "expected jwtPrimaryUID in members")
}
