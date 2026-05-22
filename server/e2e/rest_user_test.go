package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
)

func TestREST_Signup(t *testing.T) {
	cfg := &app.Config{} // signup is public/optional auth
	exp, _ := StartServer(t, cfg, false, nil)
	// mock_auth=true makes the interactor auto-create the required roles (RoleSelf/owner)
	// and skip the verification mail, so signup is self-contained against memory repos.
	exp.POST("/api/users/signup").
		WithJSON(map[string]any{"name": "Tester", "email": "tester@example.com", "password": "Passw0rd!", "mock_auth": true}).
		Expect().Status(http.StatusOK).JSON().Object().HasValue("email", "tester@example.com")
}

func TestREST_SignupValidation(t *testing.T) {
	cfg := &app.Config{}
	exp, _ := StartServer(t, cfg, false, nil)
	// Missing password + invalid email -> 400 validation failure.
	exp.POST("/api/users/signup").
		WithJSON(map[string]any{"name": "Tester", "email": "not-an-email"}).
		Expect().Status(http.StatusBadRequest)
}

func TestREST_MeWithMockAuth(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/users/me").Expect().Status(http.StatusOK).
		JSON().Object().
		ContainsKey("id").
		HasValue("name", app.FIXED_MOCK_USERNAME).
		HasValue("my_workspace_id", restDemoWID.String())
}

func TestREST_MeUnauthorized(t *testing.T) {
	cfg := &app.Config{} // non-mock, no token -> 401
	exp, _ := StartServer(t, cfg, false, nil)
	exp.GET("/api/users/me").Expect().Status(http.StatusUnauthorized)
}

func TestREST_UpdateMe(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.PATCH("/api/users/me").
		WithJSON(map[string]any{"name": "Renamed Demo"}).
		Expect().Status(http.StatusOK).
		JSON().Object().HasValue("name", "Renamed Demo")
}

func TestREST_Search(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/users/search").WithQuery("keyword", "Other REST").
		Expect().Status(http.StatusOK).JSON().Array()
}

func TestREST_GetUserByID(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/users/" + restOtherUID.String()).
		Expect().Status(http.StatusOK).
		JSON().Object().HasValue("id", restOtherUID.String())
}
