package interfaces

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
)

type AddGroupParam struct {
	Name string
}

type UpdateGroupParam struct {
	ID   id.GroupID
	Name string
}

type RemoveGroupParam struct {
	ID id.GroupID
}

type Group interface {
	GetGroups(context.Context) (group.List, error)
	AddGroup(context.Context, AddGroupParam) (*group.Group, error)
	UpdateGroup(context.Context, UpdateGroupParam) (*group.Group, error)
	RemoveGroup(context.Context, RemoveGroupParam) (id.GroupID, error)
}
