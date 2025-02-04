package interactor

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/gateway"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	"github.com/reearth/reearthx/account/accountusecase/accountgateway"
	"github.com/reearth/reearthx/account/accountusecase/accountinteractor"
	"github.com/reearth/reearthx/account/accountusecase/accountrepo"
)

type ContainerConfig struct {
	SignupSecret    string
	AuthSrvUIDomain string
}

func NewContainer(
	r *repo.Container,
	acr *accountrepo.Container,
	acg *accountgateway.Container,
	enforcer accountinteractor.WorkspaceMemberCountEnforcer,
	cerbosAdapter gateway.CerbosGateway,
	config ContainerConfig) interfaces.Container {
	return interfaces.Container{
		User:        accountinteractor.NewUser(acr, acg, config.SignupSecret, config.AuthSrvUIDomain),
		Workspace:   accountinteractor.NewWorkspace(acr, enforcer),
		Cerbos:      NewCerbos(cerbosAdapter, r),
		Role:        NewRole(r),
		Permittable: NewPermittable(r, acr),
	}
}

func checkPermissions(ctx context.Context, cerbos gateway.CerbosGateway, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error) {
	if cerbos == nil {
		return nil, nil
	}

	return cerbos.CheckPermissions(ctx, principal, resources, actions)
}
