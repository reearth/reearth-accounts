package http

import (
	"context"

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
// non-nil user for a resolved request, (nil, operator, nil) for a service caller
// (ADR-0002), or a non-nil error for an unexpected failure.
// This lets mock-auth mode (which has no AuthInfo) still resolve the fixed mock user.
// The resolver may replace c's request context, so the middlewares attach onto
// the context it leaves behind.
type AuthResolver func(c echo.Context, ai *appx.AuthInfo) (*user.User, *workspace.Operator, error)

// RequiredAuth attaches User+Operator to context; returns 401 when the request cannot
// be resolved to a user or a service caller. The resolver is always invoked (even with nil AuthInfo) so
// mock-auth mode resolves the fixed mock user.
func RequiredAuth(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			u, op, err := resolve(c, adapter.GetAuthInfo(ctx))
			if err != nil {
				return err
			}
			if u == nil && op == nil {
				logAnonymousClient(c, false)
				return httpinternal.ErrUnauthorized
			}
			c.SetRequest(c.Request().WithContext(attach(c.Request().Context(), u, op)))
			return next(c)
		}
	}
}

// RequiredUser is RequiredAuth for routes a service caller may not use, such as the
// user directory: a service caller is scoped to its integration's workspaces
// (ADR-0002 decision 5).
func RequiredUser(resolve AuthResolver) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			u, op, err := resolve(c, adapter.GetAuthInfo(ctx))
			if err != nil {
				return err
			}
			if u == nil {
				logAnonymousClient(c, false)
				return httpinternal.ErrUnauthorized
			}
			c.SetRequest(c.Request().WithContext(attach(c.Request().Context(), u, op)))
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
			if u != nil || op != nil {
				c.SetRequest(c.Request().WithContext(attach(c.Request().Context(), u, op)))
			} else {
				logAnonymousClient(c, true)
			}
			return next(c)
		}
	}
}

// logAnonymousClient logs the verdict on an anonymous client call, which the
// resolver attaches without an operator because only the route knows whether it
// admits an anonymous caller.
func logAnonymousClient(c echo.Context, accepted bool) {
	ctx := c.Request().Context()
	if cc := adapter.ClientCallOf(ctx); cc != nil && cc.Operator == nil {
		cc.LogVerdict(ctx, accepted)
	}
}

func attach(ctx context.Context, u *user.User, op *workspace.Operator) context.Context {
	if u != nil {
		ctx = adapter.AttachUser(ctx, u)
	}
	return adapter.AttachOperator(ctx, op)
}
