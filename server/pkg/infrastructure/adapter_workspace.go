package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	internalRepo "github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

// workspaceAdapter adapts internal repo.Workspace to pkg workspace.Repo interface
type workspaceAdapter struct {
	internal internalRepo.Workspace
}

// NewWorkspaceAdapter creates an adapter that bridges internal Workspace implementation to pkg Workspace interface
func NewWorkspaceAdapter(internal internalRepo.Workspace) workspace.Repo {
	return &workspaceAdapter{internal: internal}
}

func (a *workspaceAdapter) Filtered(f workspace.WorkspaceFilter) workspace.Repo {
	return &workspaceAdapter{
		internal: a.internal.Filtered(f),
	}
}

func (a *workspaceAdapter) FindByID(ctx context.Context, wid workspace.ID) (*workspace.Workspace, error) {
	return a.internal.FindByID(ctx, wid)
}

func (a *workspaceAdapter) FindByName(ctx context.Context, name string) (*workspace.Workspace, error) {
	return a.internal.FindByName(ctx, name)
}

func (a *workspaceAdapter) FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	return a.internal.FindByAlias(ctx, alias)
}

func (a *workspaceAdapter) FindByAliases(ctx context.Context, aliases []string) (workspace.List, error) {
	list, err := a.internal.FindByAliases(ctx, aliases)
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByIDs(ctx context.Context, ids workspace.IDList) (workspace.List, error) {
	list, err := a.internal.FindByIDs(ctx, ids)
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByUser(ctx context.Context, uid user.ID) (workspace.List, error) {
	list, err := a.internal.FindByUser(ctx, uid)
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByUserWithPagination(ctx context.Context, uid user.ID, pagination *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	list, pageInfo, err := a.internal.FindByUserWithPagination(ctx, uid, pagination)
	return workspace.List(list), pageInfo, err
}

func (a *workspaceAdapter) FindByIntegration(ctx context.Context, iid workspace.IntegrationID) (workspace.List, error) {
	list, err := a.internal.FindByIntegration(ctx, iid)
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByIntegrations(ctx context.Context, iids workspace.IntegrationIDList) (workspace.List, error) {
	list, err := a.internal.FindByIntegrations(ctx, iids)
	return workspace.List(list), err
}

func (a *workspaceAdapter) Create(ctx context.Context, ws *workspace.Workspace) error {
	return a.internal.Create(ctx, ws)
}

func (a *workspaceAdapter) Save(ctx context.Context, ws *workspace.Workspace) error {
	return a.internal.Save(ctx, ws)
}

func (a *workspaceAdapter) SaveAll(ctx context.Context, wsList workspace.List) error {
	return a.internal.SaveAll(ctx, wsList)
}

func (a *workspaceAdapter) Remove(ctx context.Context, wid workspace.ID) error {
	return a.internal.Remove(ctx, wid)
}

func (a *workspaceAdapter) RemoveAll(ctx context.Context, ids workspace.IDList) error {
	return a.internal.RemoveAll(ctx, ids)
}
