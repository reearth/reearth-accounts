package mongodoc

import (
	"github.com/eukarya-inc/reearth-dashboard/pkg/group"
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
)

type GroupDocument struct {
	ID   string
	Name string
}

type GroupConsumer = Consumer[*GroupDocument, *group.Group]

func NewGroupConsumer() *GroupConsumer {
	return NewConsumer[*GroupDocument, *group.Group](func(a *group.Group) bool {
		return true
	})
}

func NewGroup(g group.Group) (*GroupDocument, string) {
	id := g.ID().String()
	return &GroupDocument{
		ID:   id,
		Name: g.Name(),
	}, id
}

func (d *GroupDocument) Model() (*group.Group, error) {
	if d == nil {
		return nil, nil
	}

	gid, err := id.GroupIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	return group.New().
		ID(gid).
		Name(d.Name).
		Build()
}
