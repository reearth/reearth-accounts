package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// GetWorkspaceUseCase fetches a single workspace by ID for the admin console.
type GetWorkspaceUseCase struct {
	workspaceRepo workspace.Repo
}

// NewGetWorkspaceUseCase is a Wire provider for GetWorkspaceUseCase.
func NewGetWorkspaceUseCase(workspaceRepo workspace.Repo) *GetWorkspaceUseCase {
	return &GetWorkspaceUseCase{workspaceRepo: workspaceRepo}
}

// Execute returns the workspace with the given ID, or rerror.ErrNotFound if absent.
func (uc *GetWorkspaceUseCase) Execute(ctx context.Context, id workspace.ID) (*workspace.Workspace, error) {
	return uc.workspaceRepo.FindByID(ctx, id)
}
