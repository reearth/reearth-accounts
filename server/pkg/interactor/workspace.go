package interactor

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearth-accounts/server/pkg/usecase"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
)

type Workspace struct {
	repos *repo.Container
}

func NewWorkspace(r *repo.Container) interfaces.Workspace {
	return &Workspace{
		repos: r,
	}
}

func (i *Workspace) Fetch(ctx context.Context, ids []id.WorkspaceID, operator *usecase.Operator) ([]*workspace.Workspace, error) {
	return i.repos.Workspace.FindByIDs(ctx, ids)
}

func (i *Workspace) FindByID(ctx context.Context, wsID id.WorkspaceID, operator *usecase.Operator) (*workspace.Workspace, error) {
	return i.repos.Workspace.FindByID(ctx, wsID)
}

func (i *Workspace) FindByIDs(ctx context.Context, ids id.WorkspaceIDList, operator *usecase.Operator) ([]*workspace.Workspace, error) {
	return i.repos.Workspace.FindByIDs(ctx, ids)
}

func (i *Workspace) FindByUser(ctx context.Context, userID id.UserID, operator *usecase.Operator) ([]*workspace.Workspace, error) {
	return i.repos.Workspace.FindByUser(ctx, userID)
}

func (i *Workspace) Create(ctx context.Context, name string, userID id.UserID, operator *usecase.Operator) (*workspace.Workspace, error) {
	ws := workspace.New().
		Name(name).
		Members([]*workspace.Member{
			{
				UserID: userID,
				Role:   workspace.RoleOwner,
			},
		}).
		MustBuild()

	if err := i.repos.Workspace.Create(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (i *Workspace) Update(ctx context.Context, wsID id.WorkspaceID, name string, operator *usecase.Operator) (*workspace.Workspace, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, wsID)
	if err != nil {
		return nil, err
	}

	ws.SetName(name)

	if err := i.repos.Workspace.Save(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (i *Workspace) Remove(ctx context.Context, wsID id.WorkspaceID, operator *usecase.Operator) error {
	return i.repos.Workspace.Remove(ctx, wsID)
}

func (i *Workspace) AddMember(ctx context.Context, wsID id.WorkspaceID, userID id.UserID, role workspace.Role, operator *usecase.Operator) (*workspace.Workspace, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, wsID)
	if err != nil {
		return nil, err
	}

	ws.AddMember(userID, role)

	if err := i.repos.Workspace.Save(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (i *Workspace) RemoveMember(ctx context.Context, wsID id.WorkspaceID, userID id.UserID, operator *usecase.Operator) (*workspace.Workspace, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, wsID)
	if err != nil {
		return nil, err
	}

	ws.RemoveMember(userID)

	if err := i.repos.Workspace.Save(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (i *Workspace) UpdateMember(ctx context.Context, wsID id.WorkspaceID, userID id.UserID, role workspace.Role, operator *usecase.Operator) (*workspace.Workspace, error) {
	ws, err := i.repos.Workspace.FindByID(ctx, wsID)
	if err != nil {
		return nil, err
	}

	ws.UpdateMember(userID, role)

	if err := i.repos.Workspace.Save(ctx, ws); err != nil {
		return nil, err
	}
	return ws, nil
}

func (i *Workspace) FindByUserWithPagination(ctx context.Context, userID id.UserID, pagination *usecasex.Pagination, operator *usecase.Operator) ([]*workspace.Workspace, *usecasex.PageInfo, error) {
	return i.repos.Workspace.FindByUserWithPagination(ctx, userID, pagination)
}
