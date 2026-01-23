package infrastructure

import (
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
)

// NewMemoryUser creates a new in-memory User repository
func NewMemoryUser() user.Repo {
	internal := memory.NewUser()
	return NewUserAdapter(internal)
}

// NewMemoryWorkspace creates a new in-memory Workspace repository
func NewMemoryWorkspace() workspace.Repo {
	internal := memory.NewWorkspace()
	return NewWorkspaceAdapter(internal)
}

// NewMemoryRole creates a new in-memory Role repository
func NewMemoryRole() role.Repo {
	// Role interface is the same in internal and pkg, no adapter needed
	return memory.NewRole()
}

// NewMemoryPermittable creates a new in-memory Permittable repository
func NewMemoryPermittable() permittable.Repo {
	// Permittable interface is the same in internal and pkg, no adapter needed
	return memory.NewPermittable()
}
