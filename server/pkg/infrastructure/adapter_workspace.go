package infrastructure

import (
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// NewWorkspaceAdapter returns the workspace.Repo directly since internal and pkg now use the same interface
func NewWorkspaceAdapter(internal workspace.Repo) workspace.Repo {
	return internal
}
