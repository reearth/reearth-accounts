package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
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
	exp.GET("/api/workspaces/" + wid).
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

func TestREST_WorkspaceListByUser(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/workspaces").WithQuery("user_id", restDemoUID.String()).
		Expect().Status(http.StatusOK).JSON().Array().NotEmpty()
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
