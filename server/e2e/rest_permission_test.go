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

// --- Mock_Auth=false (real JWT pipeline) variant ---

const realJWTPermissionSub = "test|realjwt-permission"

func TestREST_RealJWT_PermissionBadRequest(t *testing.T) {
	key, cleanup := installRealJWT(t)
	defer cleanup()

	exp, _ := StartServer(t, realAuthConfig(), false, seedJWTUsers(realJWTPermissionSub))
	token := signTestToken(t, key, realJWTPermissionSub)

	// JWT validates -> OptionalAuth attaches the resolved user -> APIKeyOrAuth admits
	// the request -> handler validates an empty body -> 400.
	exp.POST("/api/permissions/check").
		WithHeader("Authorization", "Bearer "+token).
		WithJSON(map[string]any{}).
		Expect().Status(http.StatusBadRequest)
}
