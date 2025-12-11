package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

var ErrDuplicateWorkspaceAlias = rerror.NewE(i18n.T("duplicate workspace alias"))

type Workspace interface {
	Filtered(WorkspaceFilter) Workspace
	FindByID(context.Context, workspace.ID) (*workspace.Workspace, error)
	FindByName(context.Context, string) (*workspace.Workspace, error)
	FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error)
	FindByIDs(context.Context, workspace.IDList) (workspace.List, error)
	FindByUser(context.Context, user.ID) (workspace.List, error)
	FindByUserWithPagination(ctx context.Context, id user.ID, pagination *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error)
	FindByIntegration(context.Context, workspace.IntegrationID) (workspace.List, error)
	// FindByIntegrations finds workspace list based on integrations IDs
	FindByIntegrations(context.Context, workspace.IntegrationIDList) (workspace.List, error)
	Create(context.Context, *workspace.Workspace) error
	Save(context.Context, *workspace.Workspace) error
	SaveAll(context.Context, workspace.List) error
	Remove(context.Context, workspace.ID) error
	RemoveAll(context.Context, workspace.IDList) error
}
