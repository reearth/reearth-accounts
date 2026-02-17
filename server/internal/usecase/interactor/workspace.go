package interactor

import (
	"context"
	"errors"
	"maps"
	"os"
	"slices"
	"strings"

	"github.com/reearth/reearth-accounts/server/internal/rbac"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/log"
	"github.com/reearth/reearthx/rerror"
	"github.com/samber/lo"
)

type WorkspaceMemberCountEnforcer func(context.Context, *workspace.Workspace, user.List, *workspace.Operator) error

type Workspace struct {
	repos              *repo.Container
	enforceMemberCount WorkspaceMemberCountEnforcer
	userquery          interfaces.UserQuery
	permittableRepo    permittable.Repo
	roleRepo           role.Repo
	// TODO: we need to generate policy for accounts
	// after that we need to check permission on each function
	cerbos interfaces.Cerbos
}

func NewWorkspace(r *repo.Container, enforceMemberCount WorkspaceMemberCountEnforcer, cerbos interfaces.Cerbos) interfaces.Workspace {
	return &Workspace{
		repos:              r,
		enforceMemberCount: enforceMemberCount,
		userquery:          NewUserQuery(r.User, r.Users...),
		permittableRepo:    r.Permittable,
		roleRepo:           r.Role,
		cerbos:             cerbos,
	}
}

func (i *Workspace) Fetch(ctx context.Context, ids workspace.IDList, operator *workspace.Operator) (workspace.List, error) {
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

func (i *Workspace) FindByUser(ctx context.Context, id workspace.UserID, operator *workspace.Operator) (workspace.List, error) {
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

func (i *Workspace) Create(ctx context.Context, alias, name, description string, firstUser workspace.UserID, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

		if err = ws.Members().Join(firstUsers[0], role.RoleOwner, *operator.User); err != nil {
			return nil, err
		}

		if err = i.repos.Workspace.Create(ctx, ws); err != nil {
			if errors.Is(err, workspace.ErrDuplicateWorkspaceAlias) {
				return nil, interfaces.ErrWorkspaceAliasAlreadyExists
			}
			return nil, err
		}

		if err := i.updatePermittable(ctx, firstUsers[0].ID(), ws.ID(), role.RoleOwner); err != nil {
			return nil, err
		}

		operator.AddNewWorkspace(ws.ID())
		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) Update(ctx context.Context, param interfaces.UpdateWorkspaceParam, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
	if operator.User == nil {
		return nil, interfaces.ErrInvalidOperator
	}

	return Run1(ctx, operator, i.repos, Usecase().Transaction(), func(ctx context.Context) (*workspace.Workspace, error) {
		ws, err := i.repos.Workspace.FindByID(ctx, param.ID)
		if err != nil {
			return nil, applog.ErrorWithCallerLogging(ctx, "failed to find workspace", err)
		}

		if ws.IsPersonal() {
			return nil, workspace.ErrCannotModifyPersonalWorkspace
		}

		// Check permission via Cerbos or fallback to role-based check
		var cerbosChecked bool
		if i.cerbos != nil {
			result, cErr := i.cerbos.CheckPermission(ctx, *operator.User, interfaces.CheckPermissionParam{
				Service:        rbac.ServiceName,
				Resource:       rbac.ResourceWorkspace,
				Action:         rbac.ActionEdit,
				WorkspaceAlias: ws.Alias(),
			})
			if cErr != nil {
				return nil, applog.ErrorWithCallerLogging(ctx, "failed to check permission", cErr)
			}
			if result != nil {
				cerbosChecked = true
				if !result.Allowed {
					return nil, interfaces.ErrPermissionDenied
				}
			}
		}

		if !cerbosChecked {
			if ws.Members().UserRole(*operator.User) != role.RoleOwner {
				return nil, interfaces.ErrOperationDenied
			}
		}

		// Update name if provided
		if param.Name != nil {
			name := strings.TrimSpace(*param.Name)
			if len(name) == 0 {
				return nil, user.ErrInvalidName
			}
			ws.Rename(name)
		}

		// Update alias if provided
		if param.Alias != nil {
			aliasVal := strings.TrimSpace(*param.Alias)
			if aliasVal == "" {
				aliasVal = "w-" + param.ID.String()
			}
			if aliasVal != ws.Alias() {
				// When Cerbos is configured, check edit_alias permission separately for finer-grained control.
				// When Cerbos is not configured, alias editing is allowed by the general edit permission (owner-only) check above.
				if i.cerbos != nil {
					result, cErr := i.cerbos.CheckPermission(ctx, *operator.User, interfaces.CheckPermissionParam{
						Service:        rbac.ServiceName,
						Resource:       rbac.ResourceWorkspace,
						Action:         rbac.ActionEditAlias,
						WorkspaceAlias: ws.Alias(),
					})
					if cErr != nil {
						return nil, applog.ErrorWithCallerLogging(ctx, "failed to check alias edit permission", cErr)
					}
					if result != nil && !result.Allowed {
						return nil, interfaces.ErrPermissionDenied
					}
				}

				if existing, ferr := i.repos.Workspace.FindByAlias(ctx, aliasVal); ferr == nil && existing != nil && existing.ID() != ws.ID() {
					return nil, interfaces.ErrWorkspaceAliasAlreadyExists
				}
				ws.UpdateAlias(aliasVal)
			}
		}

		// Update metadata fields
		metadata := ws.Metadata()

		if param.Description != nil {
			metadata.SetDescription(*param.Description)
		}
		if param.Website != nil {
			metadata.SetWebsite(*param.Website)
		}

		// Handle photo URL
		// Note: Store the path, not the signed URL. The signed URL is generated on-demand in ToWorkspace converter.
		if param.PhotoURL != nil {
			metadata.SetPhotoURL(*param.PhotoURL)
		}

		ws.SetMetadata(*metadata)

		err = i.repos.Workspace.Save(ctx, ws)
		if err != nil {
			return nil, applog.ErrorWithCallerLogging(ctx, "failed to save workspace", err)
		}

		i.applyDefaultPolicy(ws, operator)
		return ws, nil
	})
}

func (i *Workspace) AddUserMember(ctx context.Context, workspaceID workspace.ID, users map[workspace.UserID]role.RoleType, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) AddIntegrationMember(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, role role.RoleType, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) RemoveUserMember(ctx context.Context, id workspace.ID, u workspace.UserID, operator *workspace.Operator) (*workspace.Workspace, error) {
	return i.RemoveMultipleUserMembers(ctx, id, workspace.UserIDList{u}, operator)
}

func (i *Workspace) RemoveMultipleUserMembers(ctx context.Context, id workspace.ID, userIds workspace.UserIDList, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) RemoveIntegration(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) RemoveIntegrations(ctx context.Context, wId workspace.ID, iIDs workspace.IntegrationIDList, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) UpdateUserMember(ctx context.Context, id workspace.ID, u workspace.UserID, role role.RoleType, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) UpdateIntegration(ctx context.Context, wId workspace.ID, iId workspace.IntegrationID, role role.RoleType, operator *workspace.Operator) (_ *workspace.Workspace, err error) {
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

func (i *Workspace) Remove(ctx context.Context, id workspace.ID, operator *workspace.Operator) error {
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

func (i *Workspace) TransferOwnership(ctx context.Context, workspaceID workspace.ID, newOwnerID workspace.UserID, operator *workspace.Operator) (*workspace.Workspace, error) {
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

		if ws.Members().UserRole(newOwnerID) == role.RoleReader {
			return nil, workspace.ErrCannotChangeRoleToOwner
		}

		err = ws.Members().UpdateUserRole(newOwnerID, role.RoleOwner)
		if err != nil {
			return nil, err
		}

		if err := i.updatePermittable(ctx, newOwnerID, ws.ID(), role.RoleOwner); err != nil {
			return nil, err
		}

		err = ws.Members().UpdateUserRole(*operator.User, role.RoleMaintainer)
		if err != nil {
			return nil, err
		}

		if err := i.updatePermittable(ctx, *operator.User, ws.ID(), role.RoleMaintainer); err != nil {
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

func (i *Workspace) applyDefaultPolicy(ws *workspace.Workspace, o *workspace.Operator) {
	if ws.Policy() == nil && o.DefaultPolicy != nil {
		ws.SetPolicy(o.DefaultPolicy)
	}
}

func filterWorkspaces(
	workspaces workspace.List,
	operator *workspace.Operator,
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

func (i *Workspace) updatePermittable(ctx context.Context, userID user.ID, workspaceID workspace.ID, roleName role.RoleType) error {
	r, err := i.roleRepo.FindByName(ctx, string(roleName))
	if err != nil {
		// If role not found and MockAuth is enabled, auto-create it
		if errors.Is(err, rerror.ErrNotFound) && os.Getenv("REEARTH_MOCK_AUTH") == "true" {
			log.Infof("[MockAuth] Auto-creating role for workspace: %s", roleName)
			newRole := role.New().NewID().Name(string(roleName)).MustBuild()
			if saveErr := i.roleRepo.Save(ctx, *newRole); saveErr != nil {
				return applog.ErrorWithCallerLogging(ctx, "failed to auto-create role", saveErr)
			}
			r = newRole
		} else {
			return err
		}
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
