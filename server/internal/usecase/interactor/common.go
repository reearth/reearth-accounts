package interactor

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/reearth/reearth-accounts/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
)

type ContainerConfig struct {
	SignupSecret    string
	AuthSrvUIDomain string
}

func NewContainer(
	r *repo.Container,
	acr *repo.Container,
	acg *gateway.Container,
	enforcer WorkspaceMemberCountEnforcer,
	cerbosAdapter gateway.CerbosGateway,
	config ContainerConfig) interfaces.Container {
	return interfaces.Container{
		User:        NewUser(acr, acg, config.SignupSecret, config.AuthSrvUIDomain),
		Workspace:   NewWorkspace(acr, enforcer),
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
