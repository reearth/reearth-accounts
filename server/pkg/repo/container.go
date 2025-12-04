package repo

import (
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	Workspace   Workspace
	User        User
	Role        Role
	Permittable Permittable
	Transaction usecasex.Transaction
	Users       []User
}

func (c *Container) Filtered(wsFilter WorkspaceFilter) *Container {
	return &Container{
		Workspace:   c.Workspace.Filtered(wsFilter),
		User:        c.User,
		Role:        c.Role,
		Permittable: c.Permittable,
		Transaction: c.Transaction,
		Users:       c.Users,
	}
}
