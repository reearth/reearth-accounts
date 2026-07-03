package presentation

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/auth"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/handler/user"
	mw "github.com/reearth/reearth-accounts/server/internal/admin/presentation/middleware"
)

// Handler aggregates all resource-specific admin handlers plus the middlewares.
type Handler struct {
	Auth      *auth.Handler
	User      *user.Handler
	AuthMw    echo.MiddlewareFunc
	SessionMw mw.SessionMiddleware
}

// NewHandler is a Wire provider that assembles the top-level admin Handler.
func NewHandler(authHandler *auth.Handler, userHandler *user.Handler, authMw echo.MiddlewareFunc, sessionMw mw.SessionMiddleware) *Handler {
	return &Handler{
		Auth:      authHandler,
		User:      userHandler,
		AuthMw:    authMw,
		SessionMw: sessionMw,
	}
}
