package http

import (
	"testing"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	"github.com/reearth/reearthx/appx"
	"github.com/stretchr/testify/assert"
)

func TestCheckClientRoutes(t *testing.T) {
	e := echo.New()
	e.GET("/api/workspaces/:id", func(echo.Context) error { return nil })

	assert.NoError(t, checkClientRoutes(e, map[string]adapter.ClientAccess{
		"GET /api/workspaces/:id": adapter.IntegrationClients,
	}))
	err := checkClientRoutes(e, map[string]adapter.ClientAccess{
		"GET /api/workspaces/:id":    adapter.IntegrationClients,
		"PATCH /api/workspaces/:wid": adapter.IntegrationClients,
	})
	assert.ErrorContains(t, err, "PATCH /api/workspaces/:wid")
}

func TestRegisterRESTRouterRefusesAMissingClientResolver(t *testing.T) {
	assert.Panics(t, func() { RegisterRESTRouter(echo.New(), RouterConfig{}) })
}

func TestRegisterRESTRouterRefusesAStaleClientRoute(t *testing.T) {
	saved := clientRoutes
	t.Cleanup(func() { clientRoutes = saved })
	clientRoutes = map[string]adapter.ClientAccess{"GET /api/no-such-route": adapter.IntegrationClients}

	resolve := func(echo.Context, *appx.AuthInfo) (*adapter.ClientCall, error) { return nil, nil }
	assert.PanicsWithError(t, "client route table names unregistered routes: [GET /api/no-such-route]", func() {
		RegisterRESTRouter(echo.New(), RouterConfig{ClientResolver: resolve})
	})
}
