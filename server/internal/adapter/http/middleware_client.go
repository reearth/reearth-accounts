package http

import (
	"fmt"
	"sort"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/server/internal/adapter"
	httpinternal "github.com/reearth/reearth-accounts/server/internal/adapter/http/internal"
	"github.com/reearth/reearthx/appx"
)

// ClientResolver classifies a request as a client call (ADR-0002). It returns
// (nil, nil) when the request is not one, and an error that renders as 401 when
// the call is refused. The app layer provides it.
type ClientResolver func(c echo.Context, ai *appx.AuthInfo) (*adapter.ClientCall, error)

var errClientRouteClosed = fmt.Errorf("%w: this route is not open to client tokens", httpinternal.ErrForbidden)

// ClientGate is where REST classifies a client call. It runs on the whole /api
// group before every route middleware, so it also covers the routes that have
// no auth middleware.
func ClientGate(resolve ClientResolver, routes map[string]adapter.ClientAccess) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			ctx := c.Request().Context()
			cc, err := resolve(c, adapter.GetAuthInfo(ctx))
			if err != nil {
				return err
			}
			if cc == nil {
				return next(c)
			}
			// Attached before the check, so the request log names the caller on
			// a refusal too.
			ctx = cc.Attach(ctx)
			c.SetRequest(c.Request().WithContext(ctx))
			admitted := routes[c.Request().Method+" "+c.Path()].Admits(cc.Subject.Kind())
			adapter.LogClientVerdict(ctx, admitted)
			if !admitted {
				return errClientRouteClosed
			}
			return next(c)
		}
	}
}

// checkClientRoutes fails when a table key names no registered route, so a
// renamed route cannot leave a stale entry behind.
func checkClientRoutes(e *echo.Echo, routes map[string]adapter.ClientAccess) error {
	registered := map[string]struct{}{}
	for _, r := range e.Routes() {
		registered[r.Method+" "+r.Path] = struct{}{}
	}
	var stale []string
	for k := range routes {
		if _, ok := registered[k]; !ok {
			stale = append(stale, k)
		}
	}
	if len(stale) == 0 {
		return nil
	}
	sort.Strings(stale)
	return fmt.Errorf("client route table names unregistered routes: %v", stale)
}
