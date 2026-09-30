package scim

import (
	"strings"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// RegisterSCIMRouter mounts all SCIM 2.0 routes on the given Echo instance.
// Discovery endpoints are public; user-management and group routes require a valid SCIM Bearer token.
func RegisterSCIMRouter(e *echo.Echo, workspaceRepo workspace.Repo, scimUC interfaces.Scim) {
	discovery := NewDiscoveryHandler()
	users := NewUserHandler(scimUC, workspaceRepo)
	groups := NewGroupHandler(scimUC, workspaceRepo)

	// Public discovery endpoints (no auth).
	e.GET("/scim/v2/ServiceProviderConfig", discovery.ServiceProviderConfig)
	e.GET("/scim/v2/ResourceTypes", discovery.ResourceTypes)
	e.GET("/scim/v2/Schemas", discovery.Schemas)

	// Authenticated user-management and group endpoints.
	// scimContentType normalises application/scim+json to application/json so
	// Echo's JSON binder can decode request bodies from SCIM clients.
	scim := e.Group("/scim/v2", scimContentType(), ScimBearerAuth(workspaceRepo))
	scim.GET("/Users", users.List)
	scim.POST("/Users", users.Create)
	scim.GET("/Users/:id", users.Get)
	scim.PUT("/Users/:id", users.Replace)
	scim.PATCH("/Users/:id", users.Patch)
	scim.DELETE("/Users/:id", users.Delete)

	scim.GET("/Groups", groups.List)
	scim.POST("/Groups", groups.Create)
	scim.GET("/Groups/:id", groups.Get)
	scim.PUT("/Groups/:id", groups.Replace)
	scim.PATCH("/Groups/:id", groups.Patch)
	scim.DELETE("/Groups/:id", groups.Delete)
}

// scimContentType normalises the Content-Type header: if a request arrives with
// "application/scim+json" (the SCIM-specific media type), it is rewritten to
// "application/json" so Echo's default JSON binding works without modification.
func scimContentType() echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ct := c.Request().Header.Get("Content-Type")
			if strings.HasPrefix(ct, "application/scim+json") {
				c.Request().Header.Set("Content-Type",
					strings.Replace(ct, "application/scim+json", "application/json", 1))
			}
			return next(c)
		}
	}
}
