package interfaces

import (
	"context"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/role"
)

type AddRoleParam struct {
	Name string
}

type UpdateRoleParam struct {
	ID   id.RoleID
	Name string
}

type RemoveRoleParam struct {
	ID id.RoleID
}

type Role interface {
	GetRoles(context.Context) (role.List, error)
	AddRole(context.Context, AddRoleParam) (*role.Role, error)
	UpdateRole(context.Context, UpdateRoleParam) (*role.Role, error)
	RemoveRole(context.Context, RemoveRoleParam) (id.RoleID, error)
}
