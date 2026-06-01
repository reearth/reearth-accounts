package repo

import (
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type Container struct {
	User        user.Repo
	Workspace   workspace.Repo
	Role        role.Repo
	Permittable permittable.Repo
	Transaction usecasex.Transaction
	Users       []user.Repo
	Config      config.Repo
}

var (
	ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))
)

func (c *Container) Filtered(f workspace.WorkspaceFilter) *Container {
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

// WorkspaceFilterFromOperator creates a WorkspaceFilter from an Operator
func WorkspaceFilterFromOperator(o *workspace.Operator) workspace.WorkspaceFilter {
	return workspace.WorkspaceFilterFromOperator(o)
}
