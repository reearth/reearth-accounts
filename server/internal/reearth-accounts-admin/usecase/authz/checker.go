// Package authz evaluates admin authorization against the "accounts-admin"
// Cerbos service. It is framework-independent and consumed by the usecase layer.
package authz

import (
	"context"
	"errors"
	"fmt"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	adminrbac "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

// Checker builds a Cerbos principal from the caller's permittable roles and
// evaluates it against the admin policy set.
type Checker struct {
	cerbos          gateway.CerbosGateway
	roleRepo        role.Repo
	permittableRepo permittable.Repo
}

// NewChecker is a Wire provider for the admin authorization checker.
func NewChecker(cerbosGateway gateway.CerbosGateway, roleRepo role.Repo, permittableRepo permittable.Repo) *Checker {
	return &Checker{
		cerbos:          cerbosGateway,
		roleRepo:        roleRepo,
		permittableRepo: permittableRepo,
	}
}

// Allowed reports whether caller may perform action on resource within the
// admin Cerbos service. When the gateway is not configured (local dev) it
// allows the operation.
func (c *Checker) Allowed(ctx context.Context, caller user.ID, resource, action string) (bool, error) {
	if c.cerbos == nil {
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
	resourceKind := fmt.Sprintf("%s:%s", adminrbac.ServiceName, resource)
	resourceID := fmt.Sprintf("%s:%s:%s", adminrbac.ServiceName, resource, caller.String())
	res := cerbos.NewResource(resourceKind, resourceID)

	resp, err := c.cerbos.CheckPermissions(ctx, principal, []*cerbos.Resource{res}, []string{action})
	if err != nil {
		return false, err
	}
	if resp == nil {
		return false, nil
	}
	for _, r := range resp.Results {
		if r.Actions[action] == effectv1.Effect_EFFECT_ALLOW {
			return true, nil
		}
	}
	return false, nil
}
