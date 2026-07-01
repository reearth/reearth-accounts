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
		// Users (requires admin auth)
		users := v1.Group("/users", h.AuthMw)
		users.GET("", h.User.ListUsers)
	}
}
