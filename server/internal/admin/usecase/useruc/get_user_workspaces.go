package useruc

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/rerror"
)

// GetUserWorkspacesUseCase lists the workspaces a user belongs to.
type GetUserWorkspacesUseCase struct {
	userRepo      user.Repo
	workspaceRepo workspace.Repo
}

// NewGetUserWorkspacesUseCase is a Wire provider for GetUserWorkspacesUseCase.
func NewGetUserWorkspacesUseCase(userRepo user.Repo, workspaceRepo workspace.Repo) *GetUserWorkspacesUseCase {
	return &GetUserWorkspacesUseCase{userRepo: userRepo, workspaceRepo: workspaceRepo}
}

// Execute returns the workspaces the user belongs to. The user is verified to
// exist first, so a missing user surfaces as rerror.ErrNotFound (404); an
// existing user that belongs to no workspace is a valid empty result, so
// rerror.ErrNotFound from FindByUser is translated into an empty list.
func (uc *GetUserWorkspacesUseCase) Execute(ctx context.Context, id user.ID) (workspace.List, error) {
	if _, err := uc.userRepo.FindByID(ctx, id); err != nil {
		return nil, err
	}

	list, err := uc.workspaceRepo.FindByUser(ctx, id)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return workspace.List{}, nil
		}
		return nil, err
	}
	return list, nil
}
