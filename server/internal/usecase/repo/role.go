package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/role"
)

//go:generate mockgen -source=./role.go -destination=./mock_repo/mock_role.go -package mock_repo
type Role interface {
	FindAll(context.Context) (role.List, error)
	FindByID(context.Context, id.RoleID) (*role.Role, error)
	FindByIDs(context.Context, id.RoleIDList) (role.List, error)
	Save(context.Context, role.Role) error
	Remove(context.Context, id.RoleID) error
}
