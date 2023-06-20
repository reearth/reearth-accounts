package app

import (
	"context"

	"github.com/labstack/echo/v4"
	"github.com/reearth/reearth-account/internal/adapter"
	"github.com/reearth/reearth-account/internal/usecase/interactor"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
)

func UsecaseMiddleware(
	acr *accountrepo.Container,
	acg *accountgateway.Container,
	config interactor.ContainerConfig) echo.MiddlewareFunc {
	return ContextMiddleware(func(ctx context.Context) context.Context {
		var acr2 *accountrepo.Container
		if op := adapter.Operator(ctx); op != nil {
			// apply filters to repos
			acr2 = acr.Filtered(accountrepo.WorkspaceFilterFromOperator(op))
		} else {
			acr2 = acr
		}
		uc := interactor.NewContainer(acr2, acg, config)
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
