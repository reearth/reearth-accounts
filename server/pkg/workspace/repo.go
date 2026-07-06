package workspace

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

var (
	ErrDuplicateWorkspaceAlias = errors.New("duplicate workspace alias")
	// ErrCursorPaginationUnsupported is returned by FindAll when cursor-based
	// pagination is requested. The admin cross-tenant list is offset-based only.
	ErrCursorPaginationUnsupported = errors.New("cursor pagination is not supported")
	// ErrNotImplemented is returned by a repository method that a given backend
	// does not implement (e.g. FindAll on the Postgres backend, since the admin
	// app runs on MongoDB).
	ErrNotImplemented = errors.New("not implemented for this backend")
)

//go:generate mockgen -source=./repo.go -destination=./mock_workspace.go -package workspace
type Repo interface {
	Filtered(WorkspaceFilter) Repo
	FindAll(ctx context.Context, keyword *string, pagination *usecasex.Pagination) (List, *usecasex.PageInfo, error)
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
