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
	workspaceRepo workspace.Repo
}

// NewGetUserWorkspacesUseCase is a Wire provider for GetUserWorkspacesUseCase.
func NewGetUserWorkspacesUseCase(workspaceRepo workspace.Repo) *GetUserWorkspacesUseCase {
	return &GetUserWorkspacesUseCase{workspaceRepo: workspaceRepo}
}

// Execute returns the workspaces the user belongs to. A user that belongs to no
// workspace is a valid empty result, not a 404, so rerror.ErrNotFound from the
// repository is translated into an empty list.
func (uc *GetUserWorkspacesUseCase) Execute(ctx context.Context, id user.ID) (workspace.List, error) {
	list, err := uc.workspaceRepo.FindByUser(ctx, id)
	if err != nil {
		if errors.Is(err, rerror.ErrNotFound) {
			return workspace.List{}, nil
		}
		return nil, err
	}
	return list, nil
}
