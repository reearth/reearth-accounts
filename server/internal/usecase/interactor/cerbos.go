package interactor

import (
	"context"
	"errors"
	"fmt"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/gateway"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/interfaces"
	"github.com/eukarya-inc/reearth-dashboard/internal/usecase/repo"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/rerror"
)

type Cerbos struct {
	cerbos          gateway.CerbosGateway
	roleRepo        repo.Role
	permittableRepo repo.Permittable
}

func NewCerbos(cerbos gateway.CerbosGateway, r *repo.Container) interfaces.Cerbos {
	return &Cerbos{
		cerbos:          cerbos,
		roleRepo:        r.Role,
		permittableRepo: r.Permittable,
	}
}

func (i *Cerbos) CheckPermission(ctx context.Context, param interfaces.CheckPermissionParam, user *user.User) (*interfaces.CheckPermissionResult, error) {
	permittable, err := i.permittableRepo.FindByUserID(ctx, user.ID())
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	if permittable == nil {
		return &interfaces.CheckPermissionResult{
			Allowed: false,
		}, nil
	}

	roleDomains, err := i.roleRepo.FindByIDs(ctx, permittable.RoleIDs())
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, 0, len(roleDomains))
	for _, role := range roleDomains {
		roleNames = append(roleNames, role.Name())
	}

	principal := cerbos.NewPrincipal(user.ID().String(), roleNames...)

	resourceKind := fmt.Sprintf("%s:%s", param.Service, param.Resource)
	resourceId := fmt.Sprintf("%s:%s:%s:%s", user.ID().String(), param.Service, param.Resource, param.Action)
	resource := cerbos.NewResource(resourceKind, resourceId)
	resources := []*cerbos.Resource{resource}

	resp, err := checkPermissions(ctx, i.cerbos, principal, resources, []string{param.Action})
	if err != nil {
		return nil, err
	}
	if resp == nil {
		return nil, interfaces.ErrOperationDenied
	}

	allowed := false
	for _, result := range resp.Results {
		actionResult, exists := result.Actions[param.Action]
		if !exists {
			fmt.Printf("Action %s not found in result.Actions\n", param.Action)
			continue
		}
		if actionResult == effectv1.Effect_EFFECT_ALLOW {
			allowed = true
			break
		}
	}

	return &interfaces.CheckPermissionResult{
		Allowed: allowed,
	}, nil
}
