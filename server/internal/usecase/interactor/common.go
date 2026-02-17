package interactor

import (
	"context"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

type ContainerConfig struct {
	SignupSecret    string
	AuthSrvUIDomain string
}

func NewContainer(
	r *repo.Container,
	acg *gateway.Container,
	enforcer WorkspaceMemberCountEnforcer,
	cerbosAdapter gateway.CerbosGateway,
	config ContainerConfig,
) interfaces.Container {
	cerbos := NewCerbos(r, cerbosAdapter)
	return interfaces.Container{
		User:        NewUser(r, acg, config.SignupSecret, config.AuthSrvUIDomain),
		Workspace:   NewWorkspace(r, enforcer, cerbos),
		Cerbos:      cerbos,
		Role:        NewRole(r),
		Permittable: NewPermittable(r),
	}
}

func checkPermissions(ctx context.Context, cerbos gateway.CerbosGateway, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error) {
	if cerbos == nil {
		return nil, nil
	}

	return cerbos.CheckPermissions(ctx, principal, resources, actions)
}
