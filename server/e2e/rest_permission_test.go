package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
)

func TestREST_PermissionUnauthorized(t *testing.T) {
	cfg := &app.Config{} // non-mock, no token, no API key -> APIKeyOrAuth rejects
	exp, _ := StartServer(t, cfg, false, nil)
	exp.POST("/api/permissions/check").
		WithJSON(map[string]any{"service": "dashboard", "resource": "user", "action": "read"}).
		Expect().Status(http.StatusUnauthorized)
}

func TestREST_PermissionBadRequest(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	// Mock user resolves (OptionalAuth) so APIKeyOrAuth admits the request; an empty
	// body then fails validation in the handler -> 400 (without needing a Cerbos host).
	exp.POST("/api/permissions/check").
		WithJSON(map[string]any{}).
		Expect().Status(http.StatusBadRequest)
}
