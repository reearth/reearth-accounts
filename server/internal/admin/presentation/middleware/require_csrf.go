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
// Place it after the session/approval middleware so an unauthenticated request
// gets 401 rather than 403. It must not guard sign-in (the cookie does not
// exist yet); logout uses RequireCSRFIfCookie instead.
func RequireCSRF(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if isSafeMethod(c.Request().Method) {
			return next(c)
		}
		if err := validateCSRF(c); err != nil {
			return err
		}
		return next(c)
	}
}

// RequireCSRFIfCookie applies the RequireCSRF check only when an admin_csrf
// cookie is present, so logout can still clear a stale session for a client
// whose admin_csrf cookie is missing.
//
// It only rejects a forged request that carries the admin_csrf cookie, i.e.
// one sent from a same-site origin. It does not stop a cross-site forced
// logout: the admin cookies are SameSite=Lax, so a cross-site POST arrives
// without admin_csrf and passes. Blocking cross-site requests needs an
// Origin / Sec-Fetch-Site check in front of this service.
func RequireCSRFIfCookie(next echo.HandlerFunc) echo.HandlerFunc {
	return func(c echo.Context) error {
		if isSafeMethod(c.Request().Method) {
			return next(c)
		}
		if ck, err := c.Cookie(session.CSRFCookieName); err != nil || ck.Value == "" {
			return next(c)
		}
		if err := validateCSRF(c); err != nil {
			return err
		}
		return next(c)
	}
}

func isSafeMethod(method string) bool {
	switch method {
	case http.MethodGet, http.MethodHead, http.MethodOptions:
		return true
	}
	return false
}

func validateCSRF(c echo.Context) error {
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
	return nil
}
