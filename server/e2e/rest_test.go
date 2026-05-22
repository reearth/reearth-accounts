package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
)

// TestREST_FullFlow exercises the REST surface end-to-end against memory repos via
// the shared StartServer harness with mock auth resolving the seeded Demo User.
func TestREST_FullFlow(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)

	// 1. Public signup (mock_auth=true auto-creates roles, skips verification mail).
	exp.POST("/api/users/signup").
		WithJSON(map[string]any{"name": "Flow User", "email": "flow@example.com", "password": "Passw0rd!", "mock_auth": true}).
		Expect().Status(http.StatusOK).JSON().Object().HasValue("email", "flow@example.com")

	// 2. Current user (mock auth resolves Demo User).
	me := exp.GET("/api/users/me").Expect().Status(http.StatusOK).JSON().Object()
	me.HasValue("name", app.FIXED_MOCK_USERNAME)
	me.HasValue("my_workspace_id", restDemoWID.String())

	// 3. Create a workspace.
	wid := exp.POST("/api/workspaces").
		WithJSON(map[string]any{"alias": "flow-team", "name": "Flow Team"}).
		Expect().Status(http.StatusOK).JSON().Object().Value("id").String().Raw()

	// 4. Add a member; the workspace now has the owner + the added writer.
	exp.POST("/api/workspaces/"+wid+"/members").
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": restOtherUID.String(), "role": "writer"},
		}}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Length().IsEqual(2)

	// 5. Permission check requires a Cerbos host; skip when not configured.
	if cfg.CerbosHost != "" {
		exp.POST("/api/permissions/check").
			WithJSON(map[string]any{"service": "dashboard", "resource": "user", "action": "read"}).
			Expect().Status(http.StatusOK).JSON().Object().ContainsKey("allowed")
	}

	// 6. Swagger UI is served (debug server).
	exp.GET("/swagger/index.html").Expect().Status(http.StatusOK)
}

func TestREST_Swagger(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/swagger/index.html").Expect().Status(http.StatusOK)
}
