package infrastructure

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
	Config      config.Repo
	Permittable permittable.Repo
	Role        role.Repo
	Transaction usecasex.Transaction
	User        user.Repo
	Workspace   workspace.Repo
}

var (
	ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))
)

func (c *Container) Filtered(workspace workspace.WorkspaceFilter) *Container {
	if c == nil {
		return c
	}
	return &Container{
		Config:      c.Config,
		Permittable: c.Permittable,
		Role:        c.Role,
		Transaction: c.Transaction,
		User:        c.User,
		Workspace:   c.Workspace.Filtered(workspace),
	}
}
