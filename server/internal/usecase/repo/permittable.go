package repo

import (
	"context"

	"github.com/reearth/reearth-account/pkg/id"
	"github.com/reearth/reearth-account/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

type Permittable interface {
	FindByUserID(context.Context, user.ID) (*permittable.Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (permittable.List, error)
	FindByRoleID(context.Context, id.RoleID) (permittable.List, error)
	Save(context.Context, permittable.Permittable) error
}
