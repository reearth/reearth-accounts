package repo

import (
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	Workspace   Workspace
	User        User
	Transaction usecasex.Transaction
}

func (c *Container) Filtered(wsFilter WorkspaceFilter) *Container {
	return &Container{
		Workspace:   c.Workspace.Filtered(wsFilter),
		User:        c.User,
		Transaction: c.Transaction,
	}
}
