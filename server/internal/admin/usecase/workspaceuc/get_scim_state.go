package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// GetWorkspaceScimStateUseCase returns whether SCIM provisioning has been
// enabled for a workspace by the admin.
type GetWorkspaceScimStateUseCase struct {
	repo workspace.Repo
}

// NewGetWorkspaceScimStateUseCase is a Wire provider.
func NewGetWorkspaceScimStateUseCase(repo workspace.Repo) *GetWorkspaceScimStateUseCase {
	return &GetWorkspaceScimStateUseCase{repo: repo}
}

// Execute returns true when the workspace has a ScimConfig (admin has activated
// SCIM), false otherwise. Returns an error (including rerror.ErrNotFound) when
// the workspace cannot be resolved.
func (uc *GetWorkspaceScimStateUseCase) Execute(ctx context.Context, id workspace.ID) (bool, error) {
	ws, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	return ws.ScimAvailable(), nil
}