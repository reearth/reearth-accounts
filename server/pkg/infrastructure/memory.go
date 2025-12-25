package infrastructure

import (
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/repo"
)

// NewMemoryUser creates a new in-memory User repository
func NewMemoryUser() repo.User {
	internal := memory.NewUser()
	return NewUserAdapter(internal)
}

// NewMemoryWorkspace creates a new in-memory Workspace repository
func NewMemoryWorkspace() repo.Workspace {
	internal := memory.NewWorkspace()
	return NewWorkspaceAdapter(internal)
}

// NewMemoryRole creates a new in-memory Role repository
func NewMemoryRole() repo.Role {
	// Role interface is the same in internal and pkg, no adapter needed
	return memory.NewRole()
}

// NewMemoryPermittable creates a new in-memory Permittable repository
func NewMemoryPermittable() repo.Permittable {
	// Permittable interface is the same in internal and pkg, no adapter needed
	return memory.NewPermittable()
}
