package interactor

import (
	"context"
	"fmt"
	"maps"
	"slices"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type WorkspaceMemberCountEnforcer func(context.Context, *workspace.Workspace, user.List, *usecase.Operator) error

type Workspace struct {
	repos              *repo.Container
	enforceMemberCount WorkspaceMemberCountEnforcer
	userquery          interfaces.UserQuery
}

func NewWorkspace(r *repo.Container, enforceMemberCount WorkspaceMemberCountEnforcer) interfaces.Workspace {
	return &Workspace{
		repos:              r,
		enforceMemberCount: enforceMemberCount,
		userquery:          NewUserQuery(r.User, r.Users...),
	}
}

func (i *Workspace) Fetch(ctx context.Context, ids workspace.IDList, operator *usecase.Operator) (workspace.List, error) {
	res, err := i.repos.Workspace.FindByIDs(ctx, ids)
	return filterWorkspaces(res, operator, err, false, true)
}

func (i *Workspace) FetchByID(ctx context.Context, id workspace.ID) (*workspace.Workspace, error) {
	return i.repos.Workspace.FindByID(ctx, id)
}

func (i *Workspace) FetchByName(ctx context.Context, name string) (*workspace.Workspace, error) {
	return i.repos.Workspace.FindByName(ctx, name)
}

func (i *Workspace) FetchByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	return i.repos.Workspace.FindByAlias(ctx, alias)
}

func (i *Workspace) FindByUser(ctx context.Context, id workspace.UserID, operator *usecase.Operator) (workspace.List, error) {
	res, err := i.repos.Workspace.FindByUser(ctx, id)
	return filterWorkspaces(res, operator, err, true, true)
}

func (i *Workspace) FetchByUserWithPagination(ctx context.Context, userID workspace.UserID, input interfaces.FetchByUserWithPaginationParam) (interfaces.FetchByUserWithPaginationResult, error) {
	workspaces, pageInfo, err := i.repos.Workspace.FindByUserWithPagination(ctx, userID, pagination.ToPagination(input.Page, input.Size))
	if err != nil {
		return interfaces.FetchByUserWithPaginationResult{}, err
	}

	return interfaces.FetchByUserWithPaginationResult{
		Workspaces: workspace.List(workspaces),
		TotalCount: int(pageInfo.TotalCount),
	}, nil
}

func (i *Workspace) Create(ctx context.Context, alias, name, description string, firstUser workspace.UserID, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	if len(strings.TrimSpace(name)) == 0 {
		return nil, user.ErrInvalidName
	}

	firstUsers, err := i.userquery.FetchByID(ctx, []user.ID{firstUser})
	if err != nil || len(firstUsers) == 0 {
		if err == nil {
			return nil, rerror.ErrNotFound
		}
		return nil, err
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		metadata := workspace.NewMetadata()
		metadata.SetDescription(description)

		ws, wErr := workspace.New().
			NewID().
			Alias(alias).
			Name(name).
			Metadata(metadata).
			Build()
		if wErr != nil {
			return nil, wErr
		}

		if err = ws.Members().Join(firstUsers[0], workspace.RoleOwner, *operator.User); err != nil {
			return nil, err
		}

		if err = i.repos.Workspace.Create(ctx, ws); err != nil {
			return nil, err
		}

		operator.AddNewWorkspace(ws.ID())
		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) Update(ctx context.Context, id workspace.ID, name string, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}
		if ws.Members().UserRole(*operator.User) != workspace.RoleOwner {
			return nil, interfaces.ErrOperationDenied
		}

		if len(strings.TrimSpace(name)) == 0 {
			return nil, user.ErrInvalidName
		}

		ws.Rename(name)

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) AddUserMember(ctx context.Context, workspaceID workspace.ID, users map[workspace.UserID]workspace.Role, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	keys := slices.Collect(maps.Keys(users))
	ul, err := i.userquery.FetchByID(ctx, keys)
	if err != nil {
		return nil, err
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithOwnableWorkspaces(workspaceID), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		if i.enforceMemberCount != nil {
			if err := i.enforceMemberCount(ctx, ws, ul, operator); err != nil {
				return nil, err
			}
		}

		for _, m := range ul {
			if m == nil {
				continue
			}

			err = ws.Members().Join(m, users[m.ID()], *operator.User)
			if err != nil {
				return nil, err
			}
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) AddIntegrationMember(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, role workspace.Role, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithOwnableWorkspaces(wId), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, wId)
		if err != nil {
			return nil, err
		}

		err = ws.Members().AddIntegration(iId, role, *operator.User)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) RemoveUserMember(ctx context.Context, id workspace.ID, u workspace.UserID, operator *usecase.Operator) (*workspace.Workspace, error) {
	return i.RemoveMultipleUserMembers(ctx, id, workspace.UserIDList{u}, operator)
}

func (i *Workspace) RemoveMultipleUserMembers(ctx context.Context, id workspace.ID, userIds workspace.UserIDList, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	if userIds.Len() == 0 {
		return nil, workspace.ErrNoSpecifiedUsers
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		isOwner := ws.Members().UserRole(*operator.User) == workspace.RoleOwner

		for _, uId := range userIds {
			isSelfLeave := *operator.User == uId

			if !isOwner && !isSelfLeave {
				return nil, interfaces.ErrOperationDenied
			}
			if isSelfLeave && ws.Members().IsOnlyOwner(uId) {
				return nil, interfaces.ErrOwnerCannotLeaveTheWorkspace
			}

			err := ws.Members().Leave(uId)
			if err != nil {
				return nil, err
			}
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) RemoveIntegration(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().WithOwnableWorkspaces(wId).Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, wId)
		if err != nil {
			return nil, err
		}

		err = ws.Members().DeleteIntegration(iId)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) RemoveIntegrations(ctx context.Context, wId workspace.ID, iIDs workspace.IntegrationIDList, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().WithOwnableWorkspaces(wId).Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, wId)
		if err != nil {
			return nil, err
		}

		err = ws.Members().DeleteIntegrations(iIDs)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) UpdateUserMember(ctx context.Context, id workspace.ID, u workspace.UserID, role workspace.Role, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithOwnableWorkspaces(id), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		if u == *operator.User {
			return nil, interfaces.ErrCannotChangeOwnerRole
		}

		err = ws.Members().UpdateUserRole(u, role)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) UpdateIntegration(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, role workspace.Role, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().WithOwnableWorkspaces(wId).Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, wId)
		if err != nil {
			return nil, err
		}

		err = ws.Members().UpdateIntegrationRole(iId, role)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) Remove(ctx context.Context, id workspace.ID, operator *usecase.Operator) error {
	if operator.User == nil {
		return interfaces.ErrInvalidOperator
	}

	return Run0(ctx, operator, i.repos, Usecase().Transaction().WithOwnableWorkspaces(id), func(ctx context.Context) error {
		ws, err := i.repos.Workspace.FindByID(ctx, id)
		if err != nil {
			return err
		}

		if ws.IsPersonal() {
			return workspace.ErrCannotModifyPersonalWorkspace
		}

		err = i.repos.Workspace.Remove(ctx, id)
		if err != nil {
			return err
		}

		return nil
	})
}

func (i *Workspace) TransferOwnership(ctx context.Context, workspaceID workspace.ID, newOwnerID workspace.UserID, operator *usecase.Operator) (*workspace.Workspace, error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithOwnableWorkspaces(workspaceID), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		if !ws.Members().HasUser(newOwnerID) {
			return nil, workspace.ErrTargetUserNotInTheWorkspace
		}

		if ws.Members().UserRole(newOwnerID) == workspace.RoleReader {
			return nil, workspace.ErrCannotChangeRoleToOwner
		}

		err = ws.Members().UpdateUserRole(newOwnerID, workspace.RoleOwner)
		if err != nil {
			return nil, err
		}

		err = ws.Members().UpdateUserRole(*operator.User, workspace.RoleMaintainer)
		if err != nil {
			return nil, err
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, err
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) applyDefaultPolicy(ws *workspace.Workspace, o *usecase.Operator) {
	if ws.Policy() == nil && o.DefaultPolicy != nil {
		ws.SetPolicy(o.DefaultPolicy)
	}
}

func filterWorkspaces(
	workspaces workspace.List,
	operator *usecase.Operator,
	err error,
	omitNil, applyDefaultPolicy bool,
) (workspace.List, error) {
	if err != nil {
		return nil, err
	}

	if operator == nil {
		if omitNil {
			return nil, nil
		}
		return make([]*workspace.Workspace, len(workspaces)), nil
	}

	for i, ws := range workspaces {
		if ws == nil || !operator.IsReadableWorkspace(ws.ID()) {
			workspaces[i] = nil
		}
	}

	if omitNil {
		workspaces = lo.Filter(workspaces, func(t *workspace.Workspace, _ int) bool {
			return t != nil
		})
	}

	if applyDefaultPolicy && operator.DefaultPolicy != nil {
		for _, ws := range workspaces {
			if ws == nil {
				continue
			}
			if ws.Policy() == nil {
				ws.SetPolicy(operator.DefaultPolicy)
			}
		}
	}

	return workspaces, nil
}

// TODO: Delete this once the permission check migration is complete.
func (i *Workspace) getMaintainerRole(ctx context.Context) (*role.Role, error) {
	// check and create maintainer role
	if i.repos.Role == nil {
		log.Print("Role repository is not set")
		return nil, nil
	}

	roles, err := i.repos.Role.FindAll(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to get roles: %w", err)
	}

	var maintainerRole *role.Role
	for _, r := range roles {
		if r.Name() == "maintainer" {
			maintainerRole = r
			log.Info("Found maintainer role")
			break
		}
	}

	if maintainerRole == nil {
		r, err := role.New().
			NewID().
			Name("maintainer").
			Build()
		if err != nil {
			return nil, fmt.Errorf("failed to create maintainer role domain: %w", err)
		}

		err = i.repos.Role.Save(ctx, *r)
		if err != nil {
			return nil, fmt.Errorf("failed to save maintainer role: %w", err)
		}

		maintainerRole = r
		log.Info("Created maintainer role")
	}

	return maintainerRole, nil
}

// TODO: Delete this once the permission check migration is complete.
func (i *Workspace) ensureUserHasMaintainerRole(ctx context.Context, userID user.ID, maintainerRoleID id.RoleID) error {
	if i.repos.Permittable == nil {
		log.Print("Role repository is not set")
		return nil
	}

	var p *permittable.Permittable
	var err error

	p, err = i.repos.Permittable.FindByUserID(ctx, userID)
	if err != nil && err != rerror.ErrNotFound {
		return err
	}

	if hasRole(p, maintainerRoleID) {
		return nil
	}

	if p == nil {
		p, err = permittable.New().
			NewID().
			UserID(userID).
			RoleIDs([]id.RoleID{maintainerRoleID}).
			Build()
		if err != nil {
			return err
		}
	} else {
		p.EditRoleIDs(append(p.RoleIDs(), maintainerRoleID))
	}

	err = i.repos.Permittable.Save(ctx, *p)
	if err != nil {
		return err
	}

	return nil
}

// TODO: Delete this once the permission check migration is complete.
func hasRole(p *permittable.Permittable, roleID role.ID) bool {
	if p == nil {
		return false
	}
	for _, r := range p.RoleIDs() {
		if r == roleID {
			return true
		}
	}
	return false
}
