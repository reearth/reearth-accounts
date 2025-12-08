package memory

import (
	"github.com/reearth/reearth-accounts/server/pkg/repo"
	"github.com/reearth/reearthx/usecasex"
)

func New() *repo.Container {
	return &repo.Container{
		User:        NewUser(),
		Workspace:   NewWorkspace(),
		Role:        NewRole(),
		Permittable: NewPermittable(),
		Transaction: &usecasex.NopTransaction{},
	}
}

// NewMemoryUser returns a new in-memory User repository
func NewMemoryUser() repo.User {
	return NewUser()
}

// NewMemoryWorkspace returns a new in-memory Workspace repository
func NewMemoryWorkspace() repo.Workspace {
	return NewWorkspace()
}

// NewMemoryRole returns a new in-memory Role repository
func NewMemoryRole() repo.Role {
	return NewRole()
}

// NewMemoryPermittable returns a new in-memory Permittable repository
func NewMemoryPermittable() repo.Permittable {
	return NewPermittable()
}
