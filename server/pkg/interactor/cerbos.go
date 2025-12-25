package interactor

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	"github.com/reearth/reearth-accounts/server/pkg/gateway"
	"github.com/reearth/reearth-accounts/server/pkg/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type Cerbos struct {
	cerbos          gateway.CerbosGateway
	roleRepo        repo.Role
	permittableRepo repo.Permittable
}

func NewCerbos(r *repo.Container, cerbos gateway.CerbosGateway) interfaces.Cerbos {
	return &Cerbos{
		cerbos:          cerbos,
		roleRepo:        r.Role,
		permittableRepo: r.Permittable,
	}
}

func (i *Cerbos) CheckPermission(ctx context.Context, userId user.ID, param interfaces.CheckPermissionParam) (*interfaces.CheckPermissionResult, error) {
	permittable, err := i.permittableRepo.FindByUserID(ctx, userId)
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

	principal := cerbos.NewPrincipal(userId.String(), roleNames...)

	resourceKind := fmt.Sprintf("%s:%s", param.Service, param.Resource)
	resourceId := fmt.Sprintf("%s:%s:%s:%s", userId.String(), param.Service, param.Resource, param.Action)
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
		log.Printf("Result Actions: %+v", result.Actions)

		actionResult, exists := result.Actions[param.Action]
		if !exists {
			log.Printf("Action %s not found in result.Actions\n", param.Action)
			continue
		}
		if actionResult == effectv1.Effect_EFFECT_ALLOW {
			allowed = true
			break
		}
	}

	log.Printf("Final permission result for user %s: %v", userId.String(), allowed)
	return &interfaces.CheckPermissionResult{
		Allowed: allowed,
	}, nil
}
