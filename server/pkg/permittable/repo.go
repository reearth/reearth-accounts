package permittable

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type Repo interface {
	FindByUserID(context.Context, user.ID) (*Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (List, error)
	FindByRoleID(context.Context, role.ID) (List, error)
	Save(context.Context, Permittable) error
}
