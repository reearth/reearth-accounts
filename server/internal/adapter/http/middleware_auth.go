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
//
// The resolver MUST handle every authentication mode itself: it receives the parsed
// AuthInfo (nil when no token is present) and returns (nil, nil, nil) for an
// unauthenticated request (no token / mock user not seeded / user not found), a
// non-nil user for a resolved request, or a non-nil error for an unexpected failure.
// This lets mock-auth mode (which has no AuthInfo) still resolve the fixed mock user.
type AuthResolver func(c echo.Context, ai *appx.AuthInfo) (*user.User, *workspace.Operator, error)

// RequiredAuth attaches User+Operator to context; returns 401 when the request cannot
// be resolved to a user. The resolver is always invoked (even with nil AuthInfo) so
// mock-auth mode resolves the fixed mock user.
func RequiredAuth(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			u, op, err := resolve(c, adapter.GetAuthInfo(ctx))
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

// OptionalAuth attaches User+Operator when the request resolves to a user, and
// proceeds anonymously when the resolver succeeds but finds no user. Genuine resolver
// errors (e.g. a datastore outage) are propagated so they surface as 5xx rather than
// being silently downgraded to anonymous access. The resolver is always invoked so
// mock-auth mode resolves the fixed mock user.
func OptionalAuth(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			u, op, err := resolve(c, adapter.GetAuthInfo(ctx))
			if err != nil {
				return err
			}
			if u != nil {
				ctx = adapter.AttachUser(ctx, u)
				ctx = adapter.AttachOperator(ctx, op)
				c.SetRequest(c.Request().WithContext(ctx))
			}
			return next(c)
		}
	}
}
