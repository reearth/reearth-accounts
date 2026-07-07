package presentation

import (
	"github.com/labstack/echo/v4"
	"github.com/labstack/echo/v4/middleware"
)

// AppMiddlewares holds application-level middleware for admin-api, applied to
// every route.
type AppMiddlewares struct {
	CacheControl  echo.MiddlewareFunc
	RequestLogger echo.MiddlewareFunc
}

// Middlewares returns all application middlewares as a slice.
func (m *AppMiddlewares) Middlewares() []echo.MiddlewareFunc {
	return []echo.MiddlewareFunc{
		m.CacheControl,
		m.RequestLogger,
	}
}

// NewAppMiddlewares is a Wire provider for the admin-api middleware bundle.
func NewAppMiddlewares() *AppMiddlewares {
	return &AppMiddlewares{
		CacheControl:  cacheControlMiddleware,
		RequestLogger: middleware.Logger(),
	}
}

// cacheControlMiddleware sets Cache-Control: private, no-store on every response.
// Cookie-authenticated admin routes return PII; without this header, shared
// caches and CDNs may serve one admin's response to another caller on the same
// path (RFC 7234 §3 — cookies do not make a response uncacheable by default).
func cacheControlMiddleware(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		c.Response().Header().Set("Cache-Control", "private, no-store")
		return next(c)
	}
}
