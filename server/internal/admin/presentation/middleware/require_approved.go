package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/auth/session"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// RequireApprovedMiddleware is a named type so Wire can distinguish it from the
// other middlewares (all echo.MiddlewareFunc underneath).
type RequireApprovedMiddleware echo.MiddlewareFunc

// NewRequireApprovedMiddleware builds middleware that authenticates via the
// admin_session cookie AND requires the admin user to be approved. It loads the
// user on every request (so a revoked admin loses access immediately) and
// stores it in the context as the operator. Unauthenticated → 401; a
// non-approved (pending/rejected) user → 403.
func NewRequireApprovedMiddleware(sess *session.Manager, repo adminuser.Repo) RequireApprovedMiddleware {
	return RequireApprovedMiddleware(func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			cookie, err := c.Cookie(session.CookieName)
			if err != nil || cookie.Value == "" {
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			id, err := sess.Parse(cookie.Value)
			if err != nil {
				// An empty signing secret is a server misconfiguration, not a
				// client auth failure — surface it as 500 so it isn't hidden.
				if errors.Is(err, session.ErrEmptySecret) {
					log.Errorfc(ctx, "[admin] session secret not configured: %v", err)
					return echo.NewHTTPError(http.StatusInternalServerError)
				}
				return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
			}

			u, err := repo.FindByID(ctx, id)
			if err != nil {
				if errors.Is(err, rerror.ErrNotFound) {
					return echo.NewHTTPError(http.StatusUnauthorized, "unauthorized")
				}
				log.Errorfc(ctx, "[admin] error loading admin user: %v", err)
				return echo.NewHTTPError(http.StatusInternalServerError)
			}

			if !u.IsApproved() {
				return echo.NewHTTPError(http.StatusForbidden, "not approved")
			}

			internal.SetAdminUser(c, u)
			return next(c)
		}
	})
}
