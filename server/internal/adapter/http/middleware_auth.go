package http

import (
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/appx"
)

// AuthResolver resolves the AuthInfo (already validated by the global JWT middleware
// and stored under adapter.AuthInfoKey) into a domain user + operator. It is provided
// by the app layer (reusing app/auth.go's FindBySub + generateUserOperator pipeline)
// so this package does not import app.
type AuthResolver func(c echo.Context, ai *appx.AuthInfo) (*user.User, *workspace.Operator, error)

// RequiredAuth attaches User+Operator to context; returns 401 when no/invalid token.
func RequiredAuth(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			ai := adapter.GetAuthInfo(ctx)
			if ai == nil || ai.Sub == "" {
				return httpinternal.ErrUnauthorized
			}
			u, op, err := resolve(c, ai)
			if err != nil {
				return err
			}
			if u == nil {
				return httpinternal.ErrUnauthorized
			}
			ctx = adapter.AttachUser(ctx, u)
			ctx = adapter.AttachOperator(ctx, op)
			c.SetRequest(c.Request().WithContext(ctx))
			return next(c)
		}
	}
}

// OptionalAuth attaches User+Operator when a valid token is present, else proceeds anonymously.
func OptionalAuth(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			ai := adapter.GetAuthInfo(ctx)
			if ai != nil && ai.Sub != "" {
				if u, op, err := resolve(c, ai); err == nil && u != nil {
					ctx = adapter.AttachUser(ctx, u)
					ctx = adapter.AttachOperator(ctx, op)
					c.SetRequest(c.Request().WithContext(ctx))
				}
			}
			return next(c)
		}
	}
}
