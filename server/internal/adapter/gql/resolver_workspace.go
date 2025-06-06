package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/pkg/id"
)

func (r *Resolver) WorkspaceUserMember() WorkspaceUserMemberResolver {
	return &workspaceUserMemberResolver{r}
}

type workspaceUserMemberResolver struct{ *Resolver }

func (w workspaceUserMemberResolver) User(ctx context.Context, obj *gqlmodel.WorkspaceUserMember) (*gqlmodel.User, error) {
	return dataloaders(ctx).User.Load(obj.UserID)
}

func (r *queryResolver) FindByID(ctx context.Context, workpaceId gqlmodel.ID) (*gqlmodel.Workspace, error) {
	wid, err := gqlmodel.ToID[id.Workspace](workpaceId)
	if err != nil {
		return nil, err
	}

	res, err := usecases(ctx).Workspace.FetchByID(ctx, wid)
	if err != nil {
		return nil, err
	}

	return gqlmodel.ToWorkspace(res), nil
}

func (r *queryResolver) FindByName(ctx context.Context, name string) (*gqlmodel.Workspace, error) {
	res, err := usecases(ctx).Workspace.FetchByName(ctx, name)
	if err != nil {
		return nil, err
	}

	return gqlmodel.ToWorkspace(res), nil
}

// Temporary stub implementation to satisfy gqlgen after migrating GraphQL files from reearthx/account.
// This resolver was added to avoid compile-time errors.
// Will be implemented if needed, or removed if unused after migration.
func (r *queryResolver) FindByUser(ctx context.Context, userID gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	return nil, nil
}

func (r *queryResolver) FindByUserWithPagination(ctx context.Context, userID gqlmodel.ID, pagination gqlmodel.Pagination) (*gqlmodel.WorkspacesWithPagination, error) {
	uid, err := gqlmodel.ToID[id.User](userID)
	if err != nil {
		return nil, err
	}

	res, err := usecases(ctx).Workspace.FetchByUserWithPagination(ctx, uid, interfaces.FetchByUserWithPaginationParam{
		Page: int64(pagination.Page),
		Size: int64(pagination.Size),
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.WorkspacesWithPagination{
		Workspaces: gqlmodel.ToWorkspaces(res.Workspaces),
		TotalCount: res.TotalCount,
	}, nil
}
