package interfaces

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/pkg/permittable"
	"github.com/eukarya-inc/reearth-dashboard/pkg/role"
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
