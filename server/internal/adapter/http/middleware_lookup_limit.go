package http

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/internal/adapter/ratelimit"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// UserLookupLimit charges one lookup per request against the viewer's budget.
func UserLookupLimit(l *ratelimit.Limiter) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			viewer := ratelimit.ViewerKey(clientOf(c), httpinternal.User(c), c.RealIP())
			if !l.Allow(c.Request().Context(), viewer, c.Path(), 1) {
				return httpinternal.NewError(http.StatusTooManyRequests, "too many user lookups; retry later", nil)
			}
			return next(c)
		}
	}
}

func clientOf(c echo.Context) *workspace.Client {
	if cc := adapter.ClientCallOf(c.Request().Context()); cc != nil {
		return cc.Client
	}
	return nil
}
