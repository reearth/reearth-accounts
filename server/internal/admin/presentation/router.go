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

	// Auth. Sign-in and logout are public: logout only clears the cookie,
	// and it must work even when the session token is expired/invalid since
	// the browser cannot delete an HttpOnly cookie itself. Sign-in cannot carry
	// a CSRF token (the admin_csrf cookie does not exist yet); logout checks it
	// only when the cookie is present, so a client without one can still clear
	// a stale session. That check does not cover a cross-site forced logout
	// (see RequireCSRFIfCookie).
	authg := api.Group("/v1/auth")
	authg.POST("/google", h.Auth.GoogleSignIn)
	authg.POST("/logout", h.Auth.Logout, mw.RequireCSRFIfCookie)

	v1 := api.Group("/v1")
	{
		sessionMw := echo.MiddlewareFunc(h.SessionMw)

		// Current admin user (any status)
		v1.GET("/me", h.Auth.Me, sessionMw)

		// Every approved-admin group must be built with approved... so its
		// mutating routes get the double-submit CSRF check after authentication
		// (401 for no session, then 403 for a bad token).
		// TestRouter_AllMutatingRoutesRequireCSRF fails for a mutating route that
		// misses it. RequireApproved also loads the AdminUser (and its role) into
		// the context; each route then adds a per-route RequirePermission that
		// authorizes the route's (resource, action) against the admin Cerbos
		// policy.
		approved := []echo.MiddlewareFunc{echo.MiddlewareFunc(h.RequireApproved), mw.RequireCSRF}

		// Admin-user management
		adminUsers := v1.Group("/admin-users", approved...)
		adminUsers.GET("", h.AdminUser.ListAdminUsers, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionList))
		adminUsers.POST("/:id/approve", h.AdminUser.ApproveAdminUser, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionApprove))
		adminUsers.POST("/:id/reject", h.AdminUser.RejectAdminUser, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionReject))
		adminUsers.PUT("/:id/roles", h.AdminUser.SetAdminUserRole, mw.RequirePermission(h.Checker, adminrbac.ResourceAdminUser, adminrbac.ActionAssignRole))

		// Users
		users := v1.Group("/users", approved...)
		users.GET("", h.User.ListUsers, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionList))
		users.GET("/:id", h.User.GetUser, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionRead))
		users.GET("/:id/workspaces", h.User.GetUserWorkspaces, mw.RequirePermission(h.Checker, adminrbac.ResourceUser, adminrbac.ActionRead))

		// Cross-tenant workspaces
		workspaces := v1.Group("/workspaces", approved...)
		workspaces.GET("", h.Workspace.ListWorkspaces, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionList))
		workspaces.GET("/:id", h.Workspace.GetWorkspace, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionRead))
		workspaces.GET("/:id/members", h.Workspace.GetWorkspaceMembers, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionReadMember))
		workspaces.GET("/:id/scim", h.Workspace.GetScimState, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionRead))
		workspaces.PATCH("/:id/scim", h.Workspace.SetScimAvailable, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionEdit))
		workspaces.POST("/:id/transfer-owner", h.Workspace.TransferWorkspaceOwner, mw.RequirePermission(h.Checker, adminrbac.ResourceWorkspace, adminrbac.ActionTransferOwner))
	}
}
