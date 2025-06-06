package app

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-accounts/internal/adapter"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
)

func UsecaseMiddleware(
	r *repo.Container,
	acg *gateway.Container,
	enforcer interactor.WorkspaceMemberCountEnforcer,
	cerbosAdapter gateway.CerbosGateway,
	config interactor.ContainerConfig) echo.MiddlewareFunc {
	return ContextMiddleware(func(ctx context.Context) context.Context {
		var r2 *repo.Container
		if op := adapter.Operator(ctx); op != nil {
			// apply filters to repos
			r2 = r.Filtered(repo.WorkspaceFilterFromOperator(op))
		} else {
			r2 = r
		}
		uc := interactor.NewContainer(r2, acg, enforcer, cerbosAdapter, config)
		ctx = adapter.AttachUsecases(ctx, &uc)
		return ctx
	})
}

func ContextMiddleware(fn func(ctx context.Context) context.Context) echo.MiddlewareFunc {
	return func(next echo.HandlerFunc) echo.HandlerFunc {
		return func(c echo.Context) error {
			req := c.Request()
			c.SetRequest(req.WithContext(fn(req.Context())))
			return next(c)
		}
	}
}
