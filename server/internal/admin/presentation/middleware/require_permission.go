package middleware

import (
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearthx/log"
)

// RequirePermission builds middleware that authorizes the current admin against
// the Cerbos admin policy for a route's (resource, action). Unlike the other
// admin middlewares it is parameterized per route, so it is a plain constructor
// rather than a Wire-provided named type. It must be layered on top of
// RequireApproved, which loads the AdminUser (and its role) into the context;
// this middleware then performs exactly one Cerbos check with zero extra reads.
func RequirePermission(chk *authz.Checker, resource, action string) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			u, err := internal.GetAdminUser(c) // loaded by RequireApproved; role already in memory
			if err != nil {
				return err // already an echo.HTTPError(401)
			}
			ok, err := chk.Allowed(c.Request().Context(), u.ID(), u.Role(), resource, action)
			if err != nil {
				log.Errorfc(c.Request().Context(), "[admin] permission check failed: %v", err)
				return echo.NewHTTPError(http.StatusInternalServerError)
			}
			if !ok {
				return echo.NewHTTPError(http.StatusForbidden, "forbidden")
			}
			return next(c)
		}
	}
}
