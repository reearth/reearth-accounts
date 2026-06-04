package presentation

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/handler/user"
)

// Handler aggregates all resource-specific admin handlers plus the auth middleware.
type Handler struct {
	User   *user.Handler
	AuthMw echo.MiddlewareFunc
}

// NewHandler is a Wire provider that assembles the top-level admin Handler.
func NewHandler(
	userHandler *user.Handler,
	authMw echo.MiddlewareFunc,
) *Handler {
	return &Handler{
		User:   userHandler,
		AuthMw: authMw,
	}
}
