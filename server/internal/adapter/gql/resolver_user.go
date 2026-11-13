package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/id"
)

func (r *Resolver) Me() MeResolver {
	return &meResolver{r}
}

type meResolver struct{ *Resolver }

func (r *meResolver) MyWorkspace(ctx context.Context, obj *gqlmodel.Me) (*gqlmodel.Workspace, error) {
	return dataloaders(ctx).Workspace.Load(obj.MyWorkspaceID)
}

func (r *meResolver) Workspaces(ctx context.Context, obj *gqlmodel.Me) ([]*gqlmodel.Workspace, error) {
	return loaders(ctx).Workspace.FindByUser(ctx, obj.ID)
}

func (r *queryResolver) FindUsersByIDs(ctx context.Context, userIds []gqlmodel.ID) ([]*gqlmodel.User, error) {
	uids, err := gqlmodel.ToIDs[id.User](userIds)
	if err != nil {
		return nil, err
	}

	res, err := usecases(ctx).User.FetchByID(ctx, uids)
	if err != nil {
		return nil, err
	}

	return gqlmodel.ToUsers(res), nil
}

func (r *queryResolver) FindUsersByIDsWithPagination(ctx context.Context, userIds []gqlmodel.ID, alias *string, pagination gqlmodel.Pagination) (*gqlmodel.UsersWithPagination, error) {
	uids, err := gqlmodel.ToIDs[id.User](userIds)
	if err != nil {
		return nil, err
	}

	res, err := usecases(ctx).User.FetchByIDsWithPagination(ctx, uids, alias, interfaces.FetchByIDsWithPaginationParam{
		Page: int64(pagination.Page),
		Size: int64(pagination.Size),
	})
	if err != nil {
		return nil, err
	}

	return &gqlmodel.UsersWithPagination{
		Users:      gqlmodel.ToUsers(res.Users),
		TotalCount: res.TotalCount,
	}, nil
}
