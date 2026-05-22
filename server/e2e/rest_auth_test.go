package e2e

import (
	"net/http"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/app"
)

func TestREST_AuthConfig(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.GET("/api/auth/config").Expect().Status(http.StatusOK).JSON().Object()
}

func TestREST_LogoutUnauthorized(t *testing.T) {
	cfg := &app.Config{} // no mock auth -> required auth must reject
	exp, _ := StartServer(t, cfg, false, nil)
	exp.POST("/api/auth/logout").Expect().Status(http.StatusUnauthorized)
}

func TestREST_LogoutWithMockAuth(t *testing.T) {
	cfg := &app.Config{Mock_Auth: true}
	exp, _ := StartServer(t, cfg, false, seedDemoUser)
	exp.POST("/api/auth/logout").Expect().Status(http.StatusOK).JSON().Object().ContainsKey("id")
}
