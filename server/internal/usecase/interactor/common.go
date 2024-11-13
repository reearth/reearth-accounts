package interactor

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	infraCerbos "github.com/eukarya-inc/reearth-dashboard/internal/infrastructure/cerbos"
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
	cerbosAdapter *infraCerbos.CerbosAdapter,
	config ContainerConfig) interfaces.Container {
	return interfaces.Container{
		User:        accountinteractor.NewUser(acr, acg, config.SignupSecret, config.AuthSrvUIDomain),
		Workspace:   accountinteractor.NewWorkspace(acr, enforcer),
		Cerbos:      NewCerbos(cerbosAdapter, r),
		Role:        NewRole(r),
		Permittable: NewPermittable(r, acr),
	}
}

func checkCerbosClient(cerbosClient any) (*infraCerbos.CerbosAdapter, bool) {
	if cerbosClient == nil {
		return nil, false
	}

	adapter, ok := cerbosClient.(*infraCerbos.CerbosAdapter)
	if !ok || adapter == nil {
		return nil, false
	}
	return adapter, true
}

func checkPermissions(ctx context.Context, cerbosClient any, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error) {
	cerbosAdapter, ok := checkCerbosClient(cerbosClient)
	if !ok {
		return nil, nil
	}

	return cerbosAdapter.CheckPermissions(ctx, principal, resources, actions)
}
