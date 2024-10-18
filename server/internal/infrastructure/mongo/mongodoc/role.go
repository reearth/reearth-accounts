package mongodoc

import (
	"github.com/reearth/reearth-account/pkg/id"
	"github.com/reearth/reearth-account/pkg/role"
)

type RoleDocument struct {
	ID   string
	Name string
}

type RoleConsumer = Consumer[*RoleDocument, *role.Role]

func NewRoleConsumer() *RoleConsumer {
	return NewConsumer[*RoleDocument, *role.Role](func(a *role.Role) bool {
		return true
	})
}

func NewRole(g role.Role) (*RoleDocument, string) {
	id := g.ID().String()
	return &RoleDocument{
		ID:   id,
		Name: g.Name(),
	}, id
}

func (d *RoleDocument) Model() (*role.Role, error) {
	if d == nil {
		return nil, nil
	}

	rid, err := id.RoleIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	return role.New().
		ID(rid).
		Name(d.Name).
		Build()
}
