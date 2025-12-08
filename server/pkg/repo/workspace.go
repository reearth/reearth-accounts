package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

type Workspace interface {
	Filtered(WorkspaceFilter) Workspace
	FindByID(context.Context, id.WorkspaceID) (*workspace.Workspace, error)
	FindByName(context.Context, string) (*workspace.Workspace, error)
	FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error)
	FindByIDs(context.Context, id.WorkspaceIDList) ([]*workspace.Workspace, error)
	FindByUser(context.Context, id.UserID) ([]*workspace.Workspace, error)
	FindByUserWithPagination(ctx context.Context, uid id.UserID, pagination *usecasex.Pagination) ([]*workspace.Workspace, *usecasex.PageInfo, error)
	FindByIntegration(context.Context, id.IntegrationID) ([]*workspace.Workspace, error)
	FindByIntegrations(context.Context, id.IntegrationIDList) ([]*workspace.Workspace, error)
	CheckWorkspaceAliasUnique(context.Context, id.WorkspaceID, string) error
	Create(context.Context, *workspace.Workspace) error
	Save(context.Context, *workspace.Workspace) error
	SaveAll(context.Context, []*workspace.Workspace) error
	Remove(context.Context, id.WorkspaceID) error
	RemoveAll(context.Context, id.WorkspaceIDList) error
}
