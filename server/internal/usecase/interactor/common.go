package interactor

import (
	"context"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
)

type ContainerConfig struct {
	AllowedISS            []string
	AuthSrvUIDomain       string
	MFARecoveryMaxAuthAge time.Duration
	SignupSecret          string
}

func NewContainer(
	r *repo.Container,
	acg *gateway.Container,
	enforcer WorkspaceMemberCountEnforcer,
	cerbosAdapter gateway.CerbosGateway,
	config ContainerConfig,
) interfaces.Container {
	cerbos := NewCerbos(r, cerbosAdapter)
	u := NewUser(r, acg, cerbos, config.SignupSecret, config.AuthSrvUIDomain,
		WithAllowedISS(config.AllowedISS...),
		WithMFARecoveryMaxAuthAge(config.MFARecoveryMaxAuthAge),
	)
	return interfaces.Container{
		Cerbos:      cerbos,
		Permittable: NewPermittable(r),
		Role:        r.Role,
		Scim:        NewScim(r, cerbos),
		User:        u,
		Workspace:   NewWorkspace(r, enforcer, cerbos),
	}
}

func checkPermissions(ctx context.Context, cerbos gateway.CerbosGateway, principal *cerbos.Principal, resources []*cerbos.Resource, actions []string) (*cerbos.CheckResourcesResponse, error) {
	if cerbos == nil {
		return nil, nil
	}

	return cerbos.CheckPermissions(ctx, principal, resources, actions)
}
