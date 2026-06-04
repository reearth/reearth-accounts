// Package middleware holds admin-api specific Echo middleware.
package middleware

import (
	"errors"
	"net/http"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/presentation/internal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/appx"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
)

// authInfoKey is the private context key under which the validated JWT AuthInfo
// is stored by appx.AuthMiddleware. Kept unexported so it cannot be read
// outside this package.
type authInfoKey struct{}

var ctxAuthInfo = authInfoKey{}

// NewAuthMiddleware builds the admin auth middleware: it validates the Auth0
// JWT, loads the corresponding user by subject, and stores it in the context
// for handlers. Fine-grained authorization is enforced per-action in the
// usecase layer against the "accounts-admin" Cerbos service.
func NewAuthMiddleware(providers []appx.JWTProvider, userRepo user.Repo) (echo.MiddlewareFunc, error) {
	jwtMw, err := appx.AuthMiddleware(providers, ctxAuthInfo, true)
	if err != nil {
		return nil, err
	}
	jwtEcho := echo.WrapMiddleware(jwtMw)

	loadUser := func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()

			ai, ok := ctx.Value(ctxAuthInfo).(appx.AuthInfo)
			if !ok || ai.Sub == "" {
				log.Warnfc(ctx, "[admin] rejecting request with empty sub")
				return echo.NewHTTPError(http.StatusUnauthorized, "missing or invalid token")
			}

			usr, err := userRepo.FindBySub(ctx, ai.Sub)
			if err != nil {
				if errors.Is(err, rerror.ErrNotFound) {
					log.Warnfc(ctx, "[admin] user not found for sub=%s", ai.Sub)
					return echo.NewHTTPError(http.StatusUnauthorized, "admin not found")
				}
				log.Errorfc(ctx, "[admin] error finding user by sub=%s: %v", ai.Sub, err)
				return echo.NewHTTPError(http.StatusInternalServerError)
			}

			internal.SetUser(c, usr)
			return next(c)
		}
	}

	// Compose: validate JWT first, then load the user.
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return jwtEcho(loadUser(next))
	}, nil
}
