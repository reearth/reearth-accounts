package app

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/internal/adapter"
	infraCerbos "github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/cerbos"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interactor"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	"github.com/labstack/echo/v4"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountinteractor"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
)

func UsecaseMiddleware(
	r *repo.Container,
	acr *accountrepo.Container,
	acg *accountgateway.Container,
	enforcer accountinteractor.WorkspaceMemberCountEnforcer,
	cerbosAdapter *infraCerbos.CerbosAdapter,
	config interactor.ContainerConfig) echo.MiddlewareFunc {
	return ContextMiddleware(func(ctx context.Context) context.Context {
		var acr2 *accountrepo.Container
		if op := adapter.Operator(ctx); op != nil {
			// apply filters to repos
			acr2 = acr.Filtered(accountrepo.WorkspaceFilterFromOperator(op))
		} else {
			acr2 = acr
		}
		uc := interactor.NewContainer(r, acr2, acg, enforcer, cerbosAdapter, config)
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
