package workspace

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

var (
	ErrDuplicateWorkspaceAlias = errors.New("duplicate workspace alias")
)

//go:generate mockgen -source=./repo.go -destination=./mock_workspace.go -package workspace
type Repo interface {
	Filtered(WorkspaceFilter) Repo
	Create(context.Context, *Workspace) error
	FindByAlias(ctx context.Context, alias string) (*Workspace, error)
	FindByAliases(ctx context.Context, aliases []string) (List, error)
	FindByEmailDomain(ctx context.Context, domain string) (*Workspace, error)
	FindByID(context.Context, ID) (*Workspace, error)
	FindByIDs(context.Context, IDList) (List, error)
	FindByIntegration(context.Context, IntegrationID) (List, error)
	FindByIntegrations(context.Context, IntegrationIDList) (List, error)
	FindByName(context.Context, string) (*Workspace, error)
	FindByUser(context.Context, user.ID) (List, error)
	FindByUserWithPagination(ctx context.Context, id user.ID, pagination *usecasex.Pagination) (List, *usecasex.PageInfo, error)
	Remove(context.Context, ID) error
	RemoveAll(context.Context, IDList) error
	Save(context.Context, *Workspace) error
	SaveAll(context.Context, List) error
}
