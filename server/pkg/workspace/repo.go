package workspace

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

type Repo interface {
	Filtered(WorkspaceFilter) Repo
	FindByID(context.Context, ID) (*Workspace, error)
	FindByName(context.Context, string) (*Workspace, error)
	FindByAlias(ctx context.Context, alias string) (*Workspace, error)
	FindByAliases(ctx context.Context, aliases []string) (List, error)
	FindByIDs(context.Context, IDList) (List, error)
	FindByUser(context.Context, user.ID) (List, error)
	FindByUserWithPagination(ctx context.Context, id user.ID, pagination *usecasex.Pagination) (List, *usecasex.PageInfo, error)
	FindByIntegration(context.Context, IntegrationID) (List, error)
	FindByIntegrations(context.Context, IntegrationIDList) (List, error)
	Create(context.Context, *Workspace) error
	Save(context.Context, *Workspace) error
	SaveAll(context.Context, List) error
	Remove(context.Context, ID) error
	RemoveAll(context.Context, IDList) error
}
