package presentation

import (
	"net/http"

	"github.com/labstack/echo/v4"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
)

// RegisterRoutes wires the admin HTTP routes onto the Echo instance.
func RegisterRoutes(e *echo.Echo, h *Handler) {
	e.GET("/", func(c echo.Context) error {
		return c.String(http.StatusOK, "OK")
	})

	api := e.Group("/api")
	v1 := api.Group("/v1")
	{
		sessionMw := echo.MiddlewareFunc(h.SessionMw)

		// Auth. Sign-in and logout are public: logout only clears the cookie,
		// and it must work even when the session token is expired/invalid since
		// the browser cannot delete an HttpOnly cookie itself.
		authg := v1.Group("/auth")
		authg.POST("/google", h.Auth.GoogleSignIn)
		authg.POST("/logout", h.Auth.Logout)

		// Current admin user (any status)
		v1.GET("/me", h.Auth.Me, sessionMw)

		// Admin-user management (requires an approved admin session). The group
		// requireApproved loads the AdminUser (and its role) into the context;
		// each route then adds a per-route RequirePermission that authorizes the
		// route's (resource, action) against the admin Cerbos policy.
		requireApproved := echo.MiddlewareFunc(h.RequireApproved)
		adminUsers := v1.Group("/admin-users", requireApproved)
		adminUsers.GET("", h.AdminUser.ListAdminUsers, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionList))
		adminUsers.POST("/:id/approve", h.AdminUser.ApproveAdminUser, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionApprove))
		adminUsers.POST("/:id/reject", h.AdminUser.RejectAdminUser, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionReject))

		// Users (requires an approved admin session)
		users := v1.Group("/users", requireApproved)
		users.GET("", h.User.ListUsers, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionList))
		users.GET("/:id", h.User.GetUser, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionRead))
		users.GET("/:id/workspaces", h.User.GetUserWorkspaces, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionRead))

		// Cross-tenant workspace listing (requires an approved admin session)
		workspaces := v1.Group("/workspaces", requireApproved)
		workspaces.GET("", h.Workspace.ListWorkspaces, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionList))
		workspaces.GET("/:id", h.Workspace.GetWorkspace, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionRead))
		workspaces.GET("/:id/members", h.Workspace.GetWorkspaceMembers, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionReadMember))
	}
}
