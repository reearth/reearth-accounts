package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	User        User
	Workspace   Workspace
	Role        Role
	Permittable Permittable
	Transaction usecasex.Transaction
	Users       []User
	Config      Config
}

var (
	ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))
)

func (c *Container) Filtered(f WorkspaceFilter) *Container {
	if c == nil {
		return c
	}
	return &Container{
		Workspace:   c.Workspace.Filtered(f),
		User:        c.User,
		Users:       c.Users,
		Role:        c.Role,
		Permittable: c.Permittable,
		Transaction: c.Transaction,
	}
}

// WorkspaceFilter is an alias to workspace.WorkspaceFilter
type WorkspaceFilter = workspace.WorkspaceFilter

// WorkspaceFilterFromOperator creates a WorkspaceFilter from an Operator
func WorkspaceFilterFromOperator(o *workspace.Operator) WorkspaceFilter {
	return workspace.WorkspaceFilterFromOperator(o)
}
