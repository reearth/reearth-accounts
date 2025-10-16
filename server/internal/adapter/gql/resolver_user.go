package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/pkg/id"

	"github.com/reearth/reearthx/log"
)

func (r *Resolver) Me() MeResolver {
	return &meResolver{r}
}

type meResolver struct{ *Resolver }

func (r *meResolver) MyWorkspace(ctx context.Context, obj *gqlmodel.Me) (*gqlmodel.Workspace, error) {
	return dataloaders(ctx).Workspace.Load(obj.MyWorkspaceID)
}

func (r *meResolver) Workspaces(ctx context.Context, obj *gqlmodel.Me) ([]*gqlmodel.Workspace, error) {
	uid, err := gqlmodel.ToID[id.User](obj.ID)
	if err != nil {
		return nil, err
	}

	ws, err := usecases(ctx).Workspace.FindByUser(ctx, uid, getOperator(ctx))
	if err != nil {
		log.Error(ctx, "failed to find workspaces: %v", err)
		return nil, err
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		return nil, err
	}

	return gqlmodel.ToWorkspaces(ws, exists), nil
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
