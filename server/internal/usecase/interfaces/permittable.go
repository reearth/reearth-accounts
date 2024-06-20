package interfaces

import (
	"context"

	"github.com/reearth/reearth-account/pkg/permittable"
	"github.com/reearth/reearth-account/pkg/role"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

type UpdatePermittableParam struct {
	UserID  user.ID
	RoleIDs []role.ID
}

type Permittable interface {
	GetUsersWithRoles(context.Context) (user.List, map[user.ID]role.List, error)
	UpdatePermittable(context.Context, UpdatePermittableParam) (*permittable.Permittable, error)
}
