package repo

import (
	"context"

	"github.com/reearth/reearth-account/pkg/id"
	"github.com/reearth/reearth-account/pkg/role"
)

type Role interface {
	FindAll(context.Context) (role.List, error)
	FindByID(context.Context, id.RoleID) (*role.Role, error)
	FindByIDs(context.Context, id.RoleIDList) (role.List, error)
	Save(context.Context, role.Role) error
	Remove(context.Context, id.RoleID) error
}
