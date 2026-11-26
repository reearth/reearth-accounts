package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrCannotChangeOwnerRole        = rerror.NewE(i18n.T("cannot change the role of the workspace owner"))
	ErrCannotDeleteWorkspace        = rerror.NewE(i18n.T("cannot delete workspace because at least one project is left"))
	ErrCannotSelfPromote            = rerror.NewE(i18n.T("cannot promote own role to a higher level"))
	ErrOwnerCannotLeaveTheWorkspace = rerror.NewE(i18n.T("owner user cannot leave from the workspace"))
	ErrPermissionDenied             = rerror.NewE(i18n.T("permission denied"))
	ErrTooManyWorkspaceIDs          = rerror.NewE(i18n.T("too many workspace ids requested"))
	ErrWorkspaceWithProjects        = rerror.NewE(i18n.T("target workspace still has some project"))
)

type FetchByUserWithPaginationParam struct {
	Page int64
	Size int64
}

type FetchByUserWithPaginationResult struct {
	Workspaces workspace.List
	TotalCount int
}

type FindAllWorkspacesParam struct {
	Keyword         *string
	Status          workspace.StatusFilter
	Page            int64
	Size            int64
	ExcludePersonal bool
}

type FindAllWorkspacesResult struct {
	Workspaces workspace.List
	TotalCount int
}

type UpdateWorkspaceParam struct {
	ID          workspace.ID
	Name        *string
	Alias       *string
	Description *string
	Website     *string
	PhotoURL    *string
}

// IntegrationMembership is one workspace the acting integration belongs to,
// with its role there.
type IntegrationMembership struct {
	Workspace *workspace.Workspace
	Role      role.RoleType
}

// LeaveAllResult lists, by workspace, what LeaveAllAsIntegration did.
type LeaveAllResult struct {
	Removed id.WorkspaceIDList
	Failed  id.WorkspaceIDList
}

type Workspace interface {
	Fetch(context.Context, workspace.IDList, *workspace.Operator) (workspace.List, error)
	FetchByID(context.Context, workspace.ID, *workspace.Operator) (*workspace.Workspace, error)
	FetchByName(context.Context, string, *workspace.Operator) (*workspace.Workspace, error)
	FetchByAlias(context.Context, string, *workspace.Operator) (*workspace.Workspace, error)
	FetchByUserWithPagination(context.Context, user.ID, FetchByUserWithPaginationParam, *workspace.Operator) (FetchByUserWithPaginationResult, error)
	// FindAll lists workspaces across all tenants, unfiltered by any operator's
	// readable-workspace set. Restricted to the maintainer role (see
	// checkMaintainerPermission) since it exposes every workspace across every tenant.
	FindAll(context.Context, FindAllWorkspacesParam, *workspace.Operator) (FindAllWorkspacesResult, error)
	FindByUser(context.Context, user.ID, *workspace.Operator) (workspace.List, error)
	// Create creates a workspace. Unless skipOwnerMembership is true, firstUser is
	// automatically joined as RoleOwner (the normal end-user create flow); set it
	// to true for callers that manage membership themselves (e.g. LINKS-Veda).
	Create(ctx context.Context, alias, name, description string, firstUser workspace.UserID, skipOwnerMembership bool, operator *workspace.Operator) (_ *workspace.Workspace, err error)
	Update(context.Context, UpdateWorkspaceParam, *workspace.Operator) (*workspace.Workspace, error)
	AddUserMember(context.Context, workspace.ID, map[user.ID]role.RoleType, *workspace.Operator) (*workspace.Workspace, error)
	AddIntegrationMember(context.Context, workspace.ID, workspace.IntegrationID, role.RoleType, *workspace.Operator) (*workspace.Workspace, error)
	UpdateUserMember(context.Context, workspace.ID, user.ID, role.RoleType, *workspace.Operator) (*workspace.Workspace, error)
	// UpdateUserMemberViaService sets a member's role without the self-promotion guard in UpdateUserMember
	UpdateUserMemberViaService(context.Context, workspace.ID, user.ID, role.RoleType, *workspace.Operator) (*workspace.Workspace, error)
	UpdateIntegration(context.Context, workspace.ID, workspace.IntegrationID, role.RoleType, *workspace.Operator) (*workspace.Workspace, error)
	RemoveUserMember(context.Context, workspace.ID, user.ID, *workspace.Operator) (*workspace.Workspace, error)
	RemoveMultipleUserMembers(context.Context, workspace.ID, user.IDList, *workspace.Operator) (*workspace.Workspace, error)
	RemoveIntegration(context.Context, workspace.ID, workspace.IntegrationID, *workspace.Operator) (*workspace.Workspace, error)
	RemoveIntegrations(context.Context, workspace.ID, workspace.IntegrationIDList, *workspace.Operator) (*workspace.Workspace, error)
	Remove(context.Context, workspace.ID, *workspace.Operator) error
	// Deactivate soft-deletes a workspace (sets deleted_at); Restore reverses it.
	// Owner-only, unlike the maintainer-or-owner Update/Remove.
	Deactivate(ctx context.Context, id workspace.ID, operator *workspace.Operator) (*workspace.Workspace, error)
	Restore(ctx context.Context, id workspace.ID, operator *workspace.Operator) (*workspace.Workspace, error)
	TransferOwnership(ctx context.Context, workspaceID workspace.ID, newOwnerID workspace.UserID, operator *workspace.Operator) (*workspace.Workspace, error)
	// ActingIntegrationMemberships lists the workspaces of the integration a
	// client call acts for, and its role in each. It takes no integration ID:
	// the operator's subject is the only identity, so a client cannot ask
	// about an integration it does not act for.
	ActingIntegrationMemberships(ctx context.Context, operator *workspace.Operator) ([]IntegrationMembership, error)
	// LeaveAllAsIntegration removes the integration a client call acts for
	// from every workspace it belongs to, each in its own transaction. It
	// needs membership only, not a role, because leaving lowers privilege. A
	// workspace that fails, or still has the integration after a few
	// attempts, is listed in Failed and the rest still run. When ctx is
	// cancelled, the workspaces not yet tried are listed in Failed. A retry
	// finishes the job. The error is set only when the memberships cannot be
	// read.
	LeaveAllAsIntegration(ctx context.Context, operator *workspace.Operator) (LeaveAllResult, error)
}
