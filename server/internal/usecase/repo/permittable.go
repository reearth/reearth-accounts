package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearth-accounts/pkg/user"
)

//go:generate mockgen -source=./permittable.go -destination=./mock_repo/mock_permittable.go -package mock_repo
type Permittable interface {
	FindByUserID(context.Context, user.ID) (*permittable.Permittable, error)
	FindByUserIDs(context.Context, user.IDList) (permittable.List, error)
	FindByRoleID(context.Context, id.RoleID) (permittable.List, error)
	Save(context.Context, permittable.Permittable) error
}
