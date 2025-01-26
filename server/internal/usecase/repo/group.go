package repo

import (
	"context"

	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
)

type Group interface {
	FindAll(context.Context) (group.List, error)
	FindByID(context.Context, id.GroupID) (*group.Group, error)
	FindByIDs(context.Context, id.GroupIDList) (group.List, error)
	Save(context.Context, group.Group) error
	Remove(context.Context, id.GroupID) error
}
