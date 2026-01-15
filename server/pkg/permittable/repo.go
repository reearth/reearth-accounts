package permittable

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

//go:generate mockgen -source=./permittable.go -destination=./mock_permittable.go -package permittable
type Repo interface {
	FindByUserID(context.Context, user.ID) (*Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (List, error)
	FindByRoleID(context.Context, id.RoleID) (List, error)
	Save(context.Context, Permittable) error
}
