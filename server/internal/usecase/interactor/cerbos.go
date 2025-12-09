package interactor

import (
	"context"
	"errors"
	"fmt"
	"log"

	"github.com/cerbos/cerbos-sdk-go/cerbos"
	effectv1 "github.com/cerbos/cerbos/api/genpb/cerbos/effect/v1"
	"github.com/reearth/reearth-accounts/server/internal/usecase/gateway"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type Cerbos struct {
	cerbos          gateway.CerbosGateway
	roleRepo        repo.Role
	permittableRepo repo.Permittable
	workspaceRepo   repo.Workspace
}

func NewCerbos(r *repo.Container, cerbos gateway.CerbosGateway) interfaces.Cerbos {
	return &Cerbos{
		cerbos:          cerbos,
		roleRepo:        r.Role,
		permittableRepo: r.Permittable,
		workspaceRepo:   r.Workspace,
	}
}

func (i *Cerbos) CheckPermission(ctx context.Context, userId user.ID, param interfaces.CheckPermissionParam) (*interfaces.CheckPermissionResult, error) {
	var roleIDList, workspaceRoleIDs id.RoleIDList
	var resourceId string
	p, err := i.permittableRepo.FindByUserID(ctx, userId)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	if p == nil {
		return &interfaces.CheckPermissionResult{
			Allowed: false,
		}, nil
	}

	if param.WorkspaceAlias != "" {
		workspaceRoleIDs, err = i.checkWorkspacePermission(ctx, p, param.WorkspaceAlias)
		if err != nil {
			return nil, err
		}
		roleIDList = append(roleIDList, workspaceRoleIDs...)
		// Resource ID includes workspace context
		resourceId = fmt.Sprintf("%s:%s:%s:%s", param.Service, param.Resource, param.WorkspaceAlias, userId.String())
	} else {
		// Resource ID without workspace context
		resourceId = fmt.Sprintf("%s:%s:%s", param.Service, param.Resource, userId.String())
	}

	roleIDList = append(roleIDList, p.RoleIDs()...)

	roleDomains, err := i.roleRepo.FindByIDs(ctx, roleIDList)
	if err != nil {
		return nil, err
	}

	roleNames := make([]string, 0, len(roleDomains))
	for _, role := range roleDomains {
		roleNames = append(roleNames, role.Name())
	}

	principal := cerbos.NewPrincipal(userId.String(), roleNames...)

	resourceKind := fmt.Sprintf("%s:%s", param.Service, param.Resource)
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

func (i *Cerbos) checkWorkspacePermission(ctx context.Context, permittable *permittable.Permittable, workspaceAlias string) (id.RoleIDList, error) {
	ws, err := i.workspaceRepo.FindByAlias(ctx, workspaceAlias)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return nil, err
	}
	if ws == nil {
		return nil, nil
	}

	var workspaceRoleIds id.RoleIDList
	for _, workspaceRole := range permittable.WorkspaceRoles() {
		if workspaceRole.ID() != ws.ID() {
			continue
		}

		workspaceRoleIds = append(workspaceRoleIds, workspaceRole.RoleID())
	}

	return workspaceRoleIds, nil
}
