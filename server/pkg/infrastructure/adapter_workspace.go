package infrastructure

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	internalRepo "github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearthx/usecasex"
)

// workspaceAdapter adapts internal repo.Workspace to pkg repo.Workspace interface
type workspaceAdapter struct {
	internal internalRepo.Workspace
}

// NewWorkspaceAdapter creates an adapter that bridges internal Workspace implementation to pkg Workspace interface
func NewWorkspaceAdapter(internal internalRepo.Workspace) repo.Workspace {
	return &workspaceAdapter{internal: internal}
}

func (a *workspaceAdapter) Filtered(f repo.WorkspaceFilter) repo.Workspace {
	internalFilter := internalRepo.WorkspaceFilter{
		Readable: f.Readable,
		Writable: f.Writable,
	}
	return &workspaceAdapter{
		internal: a.internal.Filtered(internalFilter),
	}
}

func (a *workspaceAdapter) FindByID(ctx context.Context, wid id.WorkspaceID) (*workspace.Workspace, error) {
	return a.internal.FindByID(ctx, workspace.ID(wid))
}

func (a *workspaceAdapter) FindByName(ctx context.Context, name string) (*workspace.Workspace, error) {
	return a.internal.FindByName(ctx, name)
}

func (a *workspaceAdapter) FindByAlias(ctx context.Context, alias string) (*workspace.Workspace, error) {
	return a.internal.FindByAlias(ctx, alias)
}

func (a *workspaceAdapter) FindByIDs(ctx context.Context, ids id.WorkspaceIDList) (workspace.List, error) {
	wsIDs := make(workspace.IDList, len(ids))
	for i, wid := range ids {
		wsIDs[i] = workspace.ID(wid)
	}
	list, err := a.internal.FindByIDs(ctx, wsIDs)
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByUser(ctx context.Context, uid id.UserID) (workspace.List, error) {
	list, err := a.internal.FindByUser(ctx, user.ID(uid))
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByUserWithPagination(ctx context.Context, uid id.UserID, pagination *usecasex.Pagination) (workspace.List, *usecasex.PageInfo, error) {
	list, pageInfo, err := a.internal.FindByUserWithPagination(ctx, user.ID(uid), pagination)
	return workspace.List(list), pageInfo, err
}

func (a *workspaceAdapter) FindByIntegration(ctx context.Context, iid id.IntegrationID) (workspace.List, error) {
	list, err := a.internal.FindByIntegration(ctx, workspace.IntegrationID(iid))
	return workspace.List(list), err
}

func (a *workspaceAdapter) FindByIntegrations(ctx context.Context, iids id.IntegrationIDList) (workspace.List, error) {
	integrationIDs := make(workspace.IntegrationIDList, len(iids))
	for i, iid := range iids {
		integrationIDs[i] = workspace.IntegrationID(iid)
	}
	list, err := a.internal.FindByIntegrations(ctx, integrationIDs)
	return workspace.List(list), err
}

func (a *workspaceAdapter) CheckWorkspaceAliasUnique(ctx context.Context, wid id.WorkspaceID, alias string) error {
	// Internal implementation doesn't have this method
	// We need to check manually
	ws, err := a.internal.FindByAlias(ctx, alias)
	if err != nil {
		// If not found, alias is unique
		return nil
	}
	if ws.ID() == workspace.ID(wid) {
		// Same workspace, alias is unique
		return nil
	}
	// Different workspace with same alias
	return internalRepo.ErrOperationDenied
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

func (a *workspaceAdapter) Remove(ctx context.Context, wid id.WorkspaceID) error {
	return a.internal.Remove(ctx, workspace.ID(wid))
}

func (a *workspaceAdapter) RemoveAll(ctx context.Context, ids id.WorkspaceIDList) error {
	wsIDs := make(workspace.IDList, len(ids))
	for i, wid := range ids {
		wsIDs[i] = workspace.ID(wid)
	}
	return a.internal.RemoveAll(ctx, wsIDs)
}
