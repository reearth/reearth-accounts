package interactor

import (
	"context"
	"time"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type ContainerConfig struct {
	AllowedISS            []string
	AuthSrvUIDomain       string
	MFARecoveryMaxAuthAge time.Duration
	// PolicyCatalog is shared by every request of the process. Nil builds
	// one per container.
	PolicyCatalog *PolicyCatalog
	PolicySecret  legal.Secret
	SignupSecret  string
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
	catalog := config.PolicyCatalog
	if catalog == nil {
		catalog = NewPolicyCatalog(r.PolicyVersion, PolicyCatalogTTL)
	}
	return interfaces.Container{
		Cerbos:      cerbos,
		Permittable: NewPermittable(r),
		Policy:      NewPolicy(r, config.PolicySecret, catalog),
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
