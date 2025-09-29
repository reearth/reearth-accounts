package gql

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"
)

func buildExistingUserSetFromWorkspace(
	ctx context.Context,
	w *workspace.Workspace,
) (map[user.ID]struct{}, error) {
	if w == nil {
		return nil, nil
	}
	return buildExistingUserSetFromWorkspaces(ctx, workspace.List{w})
}

func buildExistingUserSetFromWorkspaces(
	ctx context.Context,
	ws workspace.List,
) (map[user.ID]struct{}, error) {
	uniq := make(map[user.ID]struct{}, 256)
	for _, w := range ws {
		if w == nil {
			continue
		}
		for uid := range w.Members().Users() {
			uniq[uid] = struct{}{}
		}
	}
	if len(uniq) == 0 {
		return nil, nil
	}

	ids := make(user.IDList, 0, len(uniq))
	for id := range uniq {
		ids = append(ids, id)
	}

	ul, err := usecases(ctx).User.FetchByID(ctx, ids)
	if err != nil {
		return nil, err
	}

	exists := make(map[user.ID]struct{}, len(ul))
	for i, u := range ul {
		if u != nil {
			exists[ids[i]] = struct{}{}
		}
	}
	return exists, nil
}
