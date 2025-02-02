package repo

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/eukarya-inc/reearth-dashboard/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

//go:generate mockgen -source=./permittable.go -destination=./mock_repo/mock_permittable.go -package mock_repo
type Permittable interface {
	FindByUserID(context.Context, user.ID) (*permittable.Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (permittable.List, error)
	FindByRoleID(context.Context, id.RoleID) (permittable.List, error)
	Save(context.Context, permittable.Permittable) error
}
