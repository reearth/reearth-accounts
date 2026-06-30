// Package authz evaluates admin authorization against the "accounts-admin"
// Cerbos service. It talks to Cerbos directly (independent of the GraphQL
// presentation layer) so the admin binary stays isolated.
package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

// Checker builds a Cerbos principal from the caller's permittable roles and
// evaluates it against the admin policy set.
type Checker struct {
	client          *cerbos.GRPCClient
	roleRepo        role.Repo
	permittableRepo permittable.Repo
}

// NewChecker is a Wire provider for the admin authorization checker. A nil
// client (Cerbos unconfigured) makes every check pass, for local development.
func NewChecker(client *cerbos.GRPCClient, roleRepo role.Repo, permittableRepo permittable.Repo) *Checker {
	return &Checker{client: client, roleRepo: roleRepo, permittableRepo: permittableRepo}
}

// Allowed reports whether caller may perform action on resource within the
// admin Cerbos service.
func (c *Checker) Allowed(ctx context.Context, caller user.ID, resource, action string) (bool, error) {
	if c.client == nil {
		return true, nil
	}

	pmt, err := c.permittableRepo.FindByUserID(ctx, caller)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return false, err
	}
	if pmt == nil {
		return false, nil
	}

	roles, err := c.roleRepo.FindByIDs(ctx, pmt.RoleIDs())
	if err != nil {
		return false, err
	}
	roleNames := make([]string, 0, len(roles))
	for _, r := range roles {
		roleNames = append(roleNames, r.Name())
	}

	principal := cerbos.NewPrincipal(caller.String(), roleNames...)
	kind := fmt.Sprintf("%s:%s", adminrbac.ServiceName, resource)
	resourceID := fmt.Sprintf("%s:%s:%s", adminrbac.ServiceName, resource, caller.String())

	batch := cerbos.NewResourceBatch()
	batch.Add(cerbos.NewResource(kind, resourceID), action)

	resp, err := c.client.CheckResources(ctx, principal, batch)
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	for _, result := range resp.Results {
		if result.Actions[action] == effectv1.Effect_EFFECT_ALLOW {
			return true, nil
		}
	}
	return false, nil
}
