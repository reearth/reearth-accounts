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

// TestREST_FullFlow_Mongo runs the same flow against real MongoDB repositories
// (useMongo=true). It auto-skips when REEARTH_DB is not set (mongotest.Connect),
// so it is a no-op in environments without a database and exercises the full
// HTTP -> middleware -> handler -> interactor -> MongoDB path when one is present.
func TestREST_FullFlow_Mongo(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, true, seedDemoUser)

	exp.POST("/api/users/signup").
		WithJSON(map[string]any{"name": "Mongo Flow User", "email": "mongo-flow@example.com", "password": "Passw0rd!", "mock_auth": true}).
		Expect().Status(http.StatusOK).JSON().Object().HasValue("email", "mongo-flow@example.com")

	exp.GET("/api/users/me").Expect().Status(http.StatusOK).
		JSON().Object().HasValue("name", app.FIXED_MOCK_USERNAME)

	wid := exp.POST("/api/workspaces").
		WithJSON(map[string]any{"alias": "mongo-team", "name": "Mongo Team"}).
		Expect().Status(http.StatusOK).JSON().Object().Value("id").String().Raw()

	exp.POST("/api/workspaces/"+wid+"/members").
		WithJSON(map[string]any{"users": []map[string]any{
			{"user_id": restOtherUID.String(), "role": "writer"},
		}}).
		Expect().Status(http.StatusOK).JSON().Object().
		Value("members").Array().Length().IsEqual(2)

	exp.GET("/api/workspaces/" + wid).Expect().Status(http.StatusOK).
		JSON().Object().HasValue("id", wid)

	exp.DELETE("/api/workspaces/" + wid).Expect().Status(http.StatusNoContent)
}
