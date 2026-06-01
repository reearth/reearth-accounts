package pgdoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
)

type RoleRow struct {
	ID   string
	Name string
}

func NewRoleRow(r role.Role) RoleRow {
	return RoleRow{ID: r.ID().String(), Name: r.Name()}
}

func (r RoleRow) Model() (*role.Role, error) {
	rid, err := id.RoleIDFrom(r.ID)
	if err != nil {
		return nil, err
	}
	return role.New().ID(rid).Name(r.Name).Build()
}
