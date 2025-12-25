package interactor

import (
	"context"
	"errors"
	"maps"
	"slices"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/usecase"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type WorkspaceMemberCountEnforcer func(context.Context, *workspace.Workspace, user.List, *usecase.Operator) error

type Workspace struct {
	repos              *repo.Container
	enforceMemberCount WorkspaceMemberCountEnforcer
	userquery          interfaces.UserQuery
	permittableRepo    repo.Permittable
	roleRepo           repo.Role
}

func NewWorkspace(r *repo.Container, enforceMemberCount WorkspaceMemberCountEnforcer) interfaces.Workspace {
	return &Workspace{
		repos:              r,
		enforceMemberCount: enforceMemberCount,
		userquery:          NewUserQuery(r.User, r.Users...),
		permittableRepo:    r.Permittable,
		roleRepo:           r.Role,
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

		aliasVal := strings.TrimSpace(alias)
		wid := workspace.NewID()
		if aliasVal == "" {
			aliasVal = "w-" + wid.String()
		}

		ws, wErr := workspace.New().
			ID(wid).
			Alias(aliasVal).
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
			if errors.Is(err, repo.ErrDuplicateWorkspaceAlias) {
				return nil, interfaces.ErrWorkspaceAliasAlreadyExists
			}
			return nil, err
		}

		if err := i.updatePermittable(ctx, firstUsers[0].ID(), ws.ID(), workspace.RoleOwner); err != nil {
			return nil, err
		}

		operator.AddNewWorkspace(ws.ID())
		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) Update(ctx context.Context, id workspace.ID, name string, alias *string, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
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

		if alias != nil {
			aliasVal := strings.TrimSpace(*alias)
			if aliasVal == "" {
				aliasVal = "w-" + id.String()
			}
			if aliasVal != ws.Alias() {
				if existing, ferr := i.repos.Workspace.FindByAlias(ctx, aliasVal); ferr == nil && existing != nil && existing.ID() != ws.ID() {
					return nil, interfaces.ErrWorkspaceAliasAlreadyExists
				}
				ws.UpdateAlias(aliasVal)
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

func (i *Workspace) AddUserMember(ctx context.Context, workspaceID workspace.ID, users map[workspace.UserID]workspace.Role, operator *usecase.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	keys := slices.Collect(maps.Keys(users))

	ul, err := i.userquery.FetchByID(ctx, keys)
	if err != nil {
		return nil, applog.ErrorWithCallerLogging(ctx, "failed to fetch user", err)
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithWritableWorkspaces(workspaceID), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, workspaceID)
		if err != nil {
			return nil, applog.ErrorWithCallerLogging(ctx, "failed to fetch workspace", err)
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		if i.enforceMemberCount != nil {
			if err := i.enforceMemberCount(ctx, ws, ul, operator); err != nil {
				return nil, applog.ErrorWithCallerLogging(ctx, "failed to enforce member count", err)
			}
		}

		for _, m := range ul {
			if m == nil {
				continue
			}

			err = ws.Members().Join(m, users[m.ID()], *operator.User)
			if err != nil {
				return nil, applog.ErrorWithCallerLogging(ctx, "failed to join user to workspace", err)
			}

			if err := i.updatePermittable(ctx, m.ID(), ws.ID(), users[m.ID()]); err != nil {
				return nil, applog.ErrorWithCallerLogging(ctx, "failed to update permittable", err)
			}
		}

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, applog.ErrorWithCallerLogging(ctx, "failed to save workspace", err)
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

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithWritableWorkspaces(id), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, id)
		if err != nil {
			return nil, err
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		for _, uId := range userIds {
			isSelfLeave := *operator.User == uId

			if isSelfLeave && ws.Members().IsOnlyOwner(uId) {
				return nil, interfaces.ErrOwnerCannotLeaveTheWorkspace
			}

			err := ws.Members().Leave(uId)
			if err != nil {
				return nil, err
			}

			if err := i.removePermittable(ctx, uId, ws.ID()); err != nil {
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

		err = ws.DeleteIntegrations(iIDs)
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

	return Run1(ctx, operator, i.repos, Usecase().Transaction().WithWritableWorkspaces(id), func(ctx context.Context) (*workspace.Workspace, error) {
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

		if err := i.updatePermittable(ctx, u, ws.ID(), role); err != nil {
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

		for uId := range ws.Members().Users() {
			if err := i.removePermittable(ctx, uId, id); err != nil {
				return err
			}
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

		if err := i.updatePermittable(ctx, newOwnerID, ws.ID(), workspace.RoleOwner); err != nil {
			return nil, err
		}

		err = ws.Members().UpdateUserRole(*operator.User, workspace.RoleMaintainer)
		if err != nil {
			return nil, err
		}

		if err := i.updatePermittable(ctx, *operator.User, ws.ID(), workspace.RoleMaintainer); err != nil {
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

func (i *Workspace) updatePermittable(ctx context.Context, userID user.ID, workspaceID workspace.ID, roleName workspace.Role) error {
	r, err := i.roleRepo.FindByName(ctx, string(roleName))
	if err != nil {
		return err
	}

	p, err := i.permittableRepo.FindByUserID(ctx, userID)
	if err != nil && !errors.Is(err, rerror.ErrNotFound) {
		return err
	}

	if p == nil {
		p, err = permittable.New().
			NewID().
			UserID(userID).
			Build()
		if err != nil {
			return err
		}
	}

	p.UpdateWorkspaceRole(workspaceID, r.ID())

	return i.permittableRepo.Save(ctx, *p)
}

func (i *Workspace) removePermittable(ctx context.Context, userID user.ID, workspaceID workspace.ID) error {
	p, err := i.permittableRepo.FindByUserID(ctx, userID)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return nil
		}
		return err
	}

	p.RemoveWorkspaceRole(workspaceID)

	return i.permittableRepo.Save(ctx, *p)
}
