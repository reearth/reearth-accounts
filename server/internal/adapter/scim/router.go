package scim

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// RegisterSCIMRouter mounts all SCIM 2.0 routes on the given Echo instance.
// Discovery endpoints are public; user-management routes require a valid SCIM Bearer token.
func RegisterSCIMRouter(e *echo.Echo, workspaceRepo workspace.Repo, scimUC interfaces.Scim, baseURL string) {
	discovery := NewDiscoveryHandler()
	users := NewUserHandler(scimUC, workspaceRepo, baseURL)

	// Public discovery endpoints (no auth).
	e.GET("/scim/v2/ServiceProviderConfig", discovery.ServiceProviderConfig)
	e.GET("/scim/v2/ResourceTypes", discovery.ResourceTypes)
	e.GET("/scim/v2/Schemas", discovery.Schemas)

	// Authenticated user-management endpoints.
	scim := e.Group("/scim/v2", ScimBearerAuth(workspaceRepo))
	scim.GET("/Users", users.List)
	scim.POST("/Users", users.Create)
	scim.GET("/Users/:id", users.Get)
	scim.PUT("/Users/:id", users.Replace)
	scim.PATCH("/Users/:id", users.Patch)
	scim.DELETE("/Users/:id", users.Delete)
}
