package gql

import (
	"context"

	"github.com/labstack/gommon/log"
	"github.com/reearth/reearth-accounts/server/internal/adapter/gql/gqlmodel"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interfaces"
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
		log.Errorf("[FindByID] failed to convert workspace id: %s, workspace id: %v", err.Error(), workpaceId)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	w, err := usecases(ctx).Workspace.FetchByID(ctx, wid)
	if err != nil {
		log.Errorf("[FindByID] failed to fetch workspace by id: %s, workspace id: %v", err.Error(), wid)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		log.Errorf("[FindByID] failed to build existing user set from workspace: %s, workspace id: %v", err.Error(), wid)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	converted, err := gqlmodel.ToWorkspace(w, exists, r.Storage)
	if err != nil {
		log.Errorf("[FindByID] failed to convert workspace: %s, workspace id: %v", err.Error(), wid)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByIDs(ctx context.Context, workpaceIds []gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	wids, err := gqlmodel.ToIDs[id.Workspace](workpaceIds)
	if err != nil {
		log.Errorf("[FindByIDs] failed to convert workspace ids: %s, workspace ids: %v", err.Error(), workpaceIds)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	ws, err := usecases(ctx).Workspace.Fetch(ctx, wids, getOperator(ctx))
	if err != nil {
		log.Errorf("[FindByIDs] failed to fetch workspaces: %s, workspace ids: %v", err.Error(), wids)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		log.Errorf("[FindByIDs] failed to build existing user set from workspaces: %s, workspace ids: %v", err.Error(), wids)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspaces(ws, exists, r.Storage), nil
}

func (r *queryResolver) FindByName(ctx context.Context, name string) (*gqlmodel.Workspace, error) {
	w, err := usecases(ctx).Workspace.FetchByName(ctx, name)
	if err != nil {
		return nil, err
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		log.Errorf("[FindByName] failed to build existing user set from workspace: %s, name: %s", err.Error(), name)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	converted, err := gqlmodel.ToWorkspace(w, exists, r.Storage)
	if err != nil {
		log.Errorf("[FindByName] failed to convert workspace: %s, name: %s", err.Error(), name)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByAlias(ctx context.Context, alias string) (*gqlmodel.Workspace, error) {
	w, err := usecases(ctx).Workspace.FetchByAlias(ctx, alias)
	if err != nil {
		log.Errorf("[FindByAlias] failed to fetch workspace: %s, alias: %s", err.Error(), alias)
		return nil, err
	}

	exists, err := buildExistingUserSetFromWorkspace(ctx, w)
	if err != nil {
		log.Errorf("[FindByAlias] failed to build existing user set from workspace: %s, alias: %s", err.Error(), alias)
		return nil, err
	}

	converted, err := gqlmodel.ToWorkspace(w, exists, r.Storage)
	if err != nil {
		log.Errorf("[FindByID] failed to convert workspace: %s, workspace id: %v", err.Error(), w.ID().String())
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return converted, nil
}

func (r *queryResolver) FindByUser(ctx context.Context, userID gqlmodel.ID) ([]*gqlmodel.Workspace, error) {
	uid, err := gqlmodel.ToID[id.User](userID)
	if err != nil {
		log.Errorf("[FindByUser] failed to convert user id: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	ws, err := usecases(ctx).Workspace.FindByUser(ctx, uid, getOperator(ctx))
	if err != nil {
		log.Errorf("[FindByUser] failed to find workspaces by user: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, ws)
	if err != nil {
		log.Errorf("[FindByUser] failed to build existing user set from workspaces: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return gqlmodel.ToWorkspaces(ws, exists, r.Storage), nil
}

func (r *queryResolver) FindByUserWithPagination(ctx context.Context, userID gqlmodel.ID, pagination gqlmodel.Pagination) (*gqlmodel.WorkspacesWithPagination, error) {
	uid, err := gqlmodel.ToID[id.User](userID)
	if err != nil {
		log.Errorf("[FindByUserWithPagination] failed to convert user id: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	res, err := usecases(ctx).Workspace.FetchByUserWithPagination(ctx, uid, interfaces.FetchByUserWithPaginationParam{
		Page: int64(pagination.Page),
		Size: int64(pagination.Size),
	})
	if err != nil {
		log.Errorf("[FindByUserWithPagination] failed to find workspaces by user with pagination: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	exists, err := buildExistingUserSetFromWorkspaces(ctx, res.Workspaces)
	if err != nil {
		log.Errorf("[FindByUserWithPagination] failed to build existing user set from workspaces: %s, user id: %v", err.Error(), userID)
		return nil, gqlerror.ReturnAccountsError(ctx, err)
	}

	return &gqlmodel.WorkspacesWithPagination{
		Workspaces: gqlmodel.ToWorkspaces(res.Workspaces, exists, r.Storage),
		TotalCount: res.TotalCount,
	}, nil
}
