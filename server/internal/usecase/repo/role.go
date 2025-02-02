package repo

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/eukarya-inc/reearth-dashboard/pkg/role"
)

//go:generate mockgen -source=./role.go -destination=./mock_repo/mock_role.go -package mock_repo
type Role interface {
	FindAll(context.Context) (role.List, error)
	FindByID(context.Context, id.RoleID) (*role.Role, error)
	FindByIDs(context.Context, id.RoleIDList) (role.List, error)
	Save(context.Context, role.Role) error
	Remove(context.Context, id.RoleID) error
}
