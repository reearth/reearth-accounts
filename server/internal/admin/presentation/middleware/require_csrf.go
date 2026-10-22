package middleware

import (
	"crypto/subtle"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
)

// RequireCSRF enforces the double-submit CSRF check: for any method other than
// GET/HEAD/OPTIONS, the X-Csrf-Token header must be non-empty and equal to the
// admin_csrf cookie, otherwise the request is rejected with 403. Safe methods
// pass through untouched, so the middleware can be applied to a whole route
// group.
//
// It must not guard the sign-in endpoint (the cookie does not exist yet) nor
// logout (it has to clear cookies even when they are stale or missing).
func RequireCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		switch c.Request().Method {
		case http.MethodGet, http.MethodHead, http.MethodOptions:
			return next(c)
		}

		header := c.Request().Header.Get(session.CSRFHeaderName)
		if header == "" {
			return echo.NewHTTPError(http.StatusForbidden, "missing csrf token")
		}
		ck, err := c.Cookie(session.CSRFCookieName)
		if err != nil || ck.Value == "" {
			return echo.NewHTTPError(http.StatusForbidden, "missing csrf token")
		}
		if subtle.ConstantTimeCompare([]byte(ck.Value), []byte(header)) != 1 {
			return echo.NewHTTPError(http.StatusForbidden, "invalid csrf token")
		}
		return next(c)
	}
}
