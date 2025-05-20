package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearth-accounts/pkg/role"
	"github.com/reearth/reearth-accounts/pkg/user"
)

type UpdatePermittableParam struct {
	UserID  user.ID
	RoleIDs []role.ID
}

type Permittable interface {
	GetUsersWithRoles(context.Context) (user.List, map[user.ID]role.List, error)
	UpdatePermittable(context.Context, UpdatePermittableParam) (*permittable.Permittable, error)
}
