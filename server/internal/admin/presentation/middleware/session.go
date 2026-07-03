package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
)

// SessionMiddleware is a named type so Wire can distinguish it from the Auth0
// AuthMiddleware (both are echo.MiddlewareFunc underneath).
type SessionMiddleware echo.MiddlewareFunc

// NewSessionMiddleware builds middleware that authenticates a request from the
// admin_session cookie: it validates the session token and stores the admin
// user ID in the context. It does NOT enforce approval status (any status
// passes) — approval gating is applied per-route in a later unit. Missing or
// invalid cookies yield 401.
func NewSessionMiddleware(sess *session.Manager) SessionMiddleware {
	return SessionMiddleware(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			cookie, err := c.Cookie(session.CookieName)
			if err != nil || cookie.Value == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			id, err := sess.Parse(cookie.Value)
			if err != nil {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			internal.SetSessionAdminUserID(c, id)
			return next(c)
		}
	})
}
