package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var (
	ErrOwnerCannotLeaveTheWorkspace = rerror.NewE(i18n.T("owner user cannot leave from the workspace"))
	ErrCannotChangeOwnerRole        = rerror.NewE(i18n.T("cannot change the role of the workspace owner"))
	ErrCannotDeleteWorkspace        = rerror.NewE(i18n.T("cannot delete workspace because at least one project is left"))
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

type Workspace interface {
	Fetch(context.Context, workspace.IDList, *workspace.Operator) (workspace.List, error)
	FetchByID(context.Context, workspace.ID) (*workspace.Workspace, error)
	FetchByName(context.Context, string) (*workspace.Workspace, error)
	FetchByAlias(context.Context, string) (*workspace.Workspace, error)
	FetchByUserWithPagination(context.Context, user.ID, FetchByUserWithPaginationParam) (FetchByUserWithPaginationResult, error)
	FindByUser(context.Context, user.ID, *workspace.Operator) (workspace.List, error)
	Create(ctx context.Context, alias, name, description string, firstUser workspace.UserID, operator *workspace.Operator) (_ *workspace.Workspace, err error)
	Update(context.Context, workspace.ID, string, *workspace.Operator) (*workspace.Workspace, error)
	AddUserMember(context.Context, workspace.ID, map[user.ID]workspace.Role, *workspace.Operator) (*workspace.Workspace, error)
	AddIntegrationMember(context.Context, workspace.ID, workspace.IntegrationID, workspace.Role, *workspace.Operator) (*workspace.Workspace, error)
	UpdateUserMember(context.Context, workspace.ID, user.ID, workspace.Role, *workspace.Operator) (*workspace.Workspace, error)
	UpdateIntegration(context.Context, workspace.ID, workspace.IntegrationID, workspace.Role, *workspace.Operator) (*workspace.Workspace, error)
	RemoveUserMember(context.Context, workspace.ID, user.ID, *workspace.Operator) (*workspace.Workspace, error)
	RemoveMultipleUserMembers(context.Context, workspace.ID, user.IDList, *workspace.Operator) (*workspace.Workspace, error)
	RemoveIntegration(context.Context, workspace.ID, workspace.IntegrationID, *workspace.Operator) (*workspace.Workspace, error)
	RemoveIntegrations(context.Context, workspace.ID, workspace.IntegrationIDList, *workspace.Operator) (*workspace.Workspace, error)
	Remove(context.Context, workspace.ID, *workspace.Operator) error
}
