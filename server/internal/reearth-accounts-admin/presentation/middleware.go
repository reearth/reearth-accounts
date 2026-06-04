package presentation

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// AppMiddlewares holds application-level middleware for admin-api, applied to
// every route.
type AppMiddlewares struct {
	RequestLogger echo.MiddlewareFunc
}

// Middlewares returns all application middlewares as a slice.
func (m *AppMiddlewares) Middlewares() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		m.RequestLogger,
	}
}

// NewAppMiddlewares is a Wire provider for the admin-api middleware bundle.
func NewAppMiddlewares() *AppMiddlewares {
	return &AppMiddlewares{
		RequestLogger: middleware.Logger(),
	}
}
