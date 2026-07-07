package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// ListWorkspaceMembersUseCase fetches a workspace and resolves the domain users
// of its members so callers can render names and emails alongside roles.
type ListWorkspaceMembersUseCase struct {
	userRepo      user.Repo
	workspaceRepo workspace.Repo
}

// NewListWorkspaceMembersUseCase is a Wire provider for ListWorkspaceMembersUseCase.
func NewListWorkspaceMembersUseCase(workspaceRepo workspace.Repo, userRepo user.Repo) *ListWorkspaceMembersUseCase {
	return &ListWorkspaceMembersUseCase{userRepo: userRepo, workspaceRepo: workspaceRepo}
}

// Execute returns the workspace and a map from member user ID to the resolved
// *user.User. Members whose users cannot be found are simply absent from the map;
// a missing workspace propagates rerror.ErrNotFound.
func (uc *ListWorkspaceMembersUseCase) Execute(ctx context.Context, id workspace.ID) (*workspace.Workspace, map[user.ID]*user.User, error) {
	ws, err := uc.workspaceRepo.FindByID(ctx, id)
	if err != nil {
		return nil, nil, err
	}

	ids := make(user.IDList, 0)
	if m := ws.Members(); m != nil {
		for uid := range m.Users() {
			ids = append(ids, uid)
		}
	}

	users := make(map[user.ID]*user.User, len(ids))
	if len(ids) > 0 {
		list, err := uc.userRepo.FindByIDs(ctx, ids)
		if err != nil {
			return nil, nil, err
		}
		for _, u := range list {
			if u != nil {
				users[u.ID()] = u
			}
		}
	}

	return ws, users, nil
}
