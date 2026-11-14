package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
	"github.com/reearth/reearth-accounts/server/pkg/applog"
	"github.com/reearth/reearth-accounts/server/pkg/gqlclient/gqlerror"
	"github.com/reearth/reearth-accounts/server/pkg/id"
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
		applog.Error(ctx, "[FindByID] failed to convert workspace id", applog.Err(err), applog.WorkspaceID(workpaceId))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	w, err := usecases(ctx).Workspace.FetchByID(ctx, wid, getOperator(ctx))
	if err != nil {
		applog.Error(ctx, "[FindByID] failed to fetch workspace by id", applog.Err(err), applog.WorkspaceID(wid))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		applog.Error(ctx, "[FindByID] failed to build existing user set from workspace", applog.Err(err), applog.WorkspaceID(wid))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	converted, err := gqlmodel.ToWorkspace(ctx, w, exists, r.Photos)
	if err != nil {
		applog.Error(ctx, "[FindByID] failed to convert workspace", applog.Err(err), applog.WorkspaceID(wid))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByIDs(ctx context.Context, workpaceIds []gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	wids, err := gqlmodel.ToIDs[id.Workspace](workpaceIds)
	if err != nil {
		applog.Error(ctx, "[FindByIDs] failed to convert workspace ids", applog.Err(err), applog.WorkspaceIDs(workpaceIds))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	ws, err := usecases(ctx).Workspace.Fetch(ctx, wids, getOperator(ctx))
	if err != nil {
		applog.Error(ctx, "[FindByIDs] failed to fetch workspaces", applog.Err(err), applog.WorkspaceIDs(wids))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		applog.Error(ctx, "[FindByIDs] failed to build existing user set from workspaces", applog.Err(err), applog.WorkspaceIDs(wids))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspaces(ctx, ws, exists, r.Photos), nil
}

func (r *queryResolver) FindByName(ctx context.Context, name string) (*gqlmodel.Workspace, error) {
	w, err := usecases(ctx).Workspace.FetchByName(ctx, name, getOperator(ctx))
	if err != nil {
		return nil, err
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		applog.Error(ctx, "[FindByName] failed to build existing user set from workspace", applog.Err(err))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	converted, err := gqlmodel.ToWorkspace(ctx, w, exists, r.Photos)
	if err != nil {
		applog.Error(ctx, "[FindByName] failed to convert workspace", applog.Err(err))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByAlias(ctx context.Context, alias string) (*gqlmodel.Workspace, error) {
	w, err := usecases(ctx).Workspace.FetchByAlias(ctx, alias, getOperator(ctx))
	if err != nil {
		applog.Error(ctx, "[FindByAlias] failed to fetch workspace", applog.Err(err))
		return nil, err
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		applog.Error(ctx, "[FindByAlias] failed to build existing user set from workspace", applog.Err(err))
		return nil, err
	}

	converted, err := gqlmodel.ToWorkspace(ctx, w, exists, r.Photos)
	if err != nil {
		applog.Error(ctx, "[FindByID] failed to convert workspace", applog.Err(err), applog.WorkspaceID(w.ID()))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByUser(ctx context.Context, userID gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	uid, err := gqlmodel.ToID[id.User](userID)
	if err != nil {
		applog.Error(ctx, "[FindByUser] failed to convert user id", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	ws, err := usecases(ctx).Workspace.FindByUser(ctx, uid, getOperator(ctx))
	if err != nil {
		applog.Error(ctx, "[FindByUser] failed to find workspaces by user", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		applog.Error(ctx, "[FindByUser] failed to build existing user set from workspaces", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspaces(ctx, ws, exists, r.Photos), nil
}

func (r *queryResolver) FindByUserWithPagination(ctx context.Context, userID gqlmodel.ID, pagination gqlmodel.Pagination) (*gqlmodel.WorkspacesWithPagination, error) {
	uid, err := gqlmodel.ToID[id.User](userID)
	if err != nil {
		applog.Error(ctx, "[FindByUserWithPagination] failed to convert user id", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	res, err := usecases(ctx).Workspace.FetchByUserWithPagination(ctx, uid, interfaces.FetchByUserWithPaginationParam{
		Page: int64(pagination.Page),
		Size: int64(pagination.Size),
	}, getOperator(ctx))
	if err != nil {
		applog.Error(ctx, "[FindByUserWithPagination] failed to find workspaces by user with pagination", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, res.Workspaces)
	if err != nil {
		applog.Error(ctx, "[FindByUserWithPagination] failed to build existing user set from workspaces", applog.Err(err), applog.UserID(userID))
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return &gqlmodel.WorkspacesWithPagination{
		Workspaces: gqlmodel.ToWorkspaces(ctx, res.Workspaces, exists, r.Photos),
		TotalCount: res.TotalCount,
	}, nil
}
