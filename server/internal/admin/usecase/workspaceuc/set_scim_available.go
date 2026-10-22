package workspaceuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// SetWorkspaceScimAvailableUseCase enables or disables SCIM provisioning for a
// workspace. Enabling creates a blank ScimConfig if none exists; disabling
// removes it entirely (and with it the SCIM token and any group-role mappings).
type SetWorkspaceScimAvailableUseCase struct {
	repo workspace.Repo
}

// NewSetWorkspaceScimAvailableUseCase is a Wire provider.
func NewSetWorkspaceScimAvailableUseCase(repo workspace.Repo) *SetWorkspaceScimAvailableUseCase {
	return &SetWorkspaceScimAvailableUseCase{repo: repo}
}

// Execute updates the SCIM availability and returns the new state. Returns an
// error (including rerror.ErrNotFound) when the workspace cannot be resolved.
func (uc *SetWorkspaceScimAvailableUseCase) Execute(ctx context.Context, id workspace.ID, enabled bool) (bool, error) {
	ws, err := uc.repo.FindByID(ctx, id)
	if err != nil {
		return false, err
	}
	if enabled && !ws.ScimAvailable() {
		ws.SetScimConfig(workspace.NewScimConfig())
	} else if !enabled {
		ws.SetScimConfig(nil)
	}
	if err := uc.repo.Save(ctx, ws); err != nil {
		return false, err
	}
	return ws.ScimAvailable(), nil
}