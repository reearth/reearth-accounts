package presentation

import (
	"net/http"

	"github.com/labstack/echo/v4"
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

		// Admin-user management (requires an approved admin session)
		requireApproved := echo.MiddlewareFunc(h.RequireApproved)
		adminUsers := v1.Group("/admin-users", requireApproved)
		adminUsers.GET("", h.AdminUser.ListAdminUsers)
		adminUsers.POST("/:id/approve", h.AdminUser.ApproveAdminUser)

		// Users (requires admin auth)
		users := v1.Group("/users", h.AuthMw)
		users.GET("", h.User.ListUsers)
	}
}
