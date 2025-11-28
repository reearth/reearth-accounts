package mongodoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	permittable "github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type WorkspaceRoleDocument struct {
	WorkspaceID string `bson:"workspace_id"`
	RoleID      string `bson:"role_id"`
}

type PermittableDocument struct {
	ID             string
	UserID         string
	UserRoles      []string                `bson:"user_roles"`
	WorkspaceRoles []WorkspaceRoleDocument `bson:"workspace_roles,omitempty"`
}

type PermittableConsumer = Consumer[*PermittableDocument, *permittable.Permittable]

func NewPermittableConsumer() *PermittableConsumer {
	return NewConsumer[*PermittableDocument, *permittable.Permittable](func(a *permittable.Permittable) bool {
		return true
	})
}

func NewPermittable(p permittable.Permittable) (*PermittableDocument, string) {
	id := p.ID().String()

	roleIds := make([]string, 0, len(p.RoleIDs()))
	for _, r := range p.RoleIDs() {
		roleIds = append(roleIds, r.String())
	}

	return &PermittableDocument{
		ID:        id,
		UserID:    p.UserID().String(),
		UserRoles: roleIds,
	}, id
}

func (d *PermittableDocument) Model() (*permittable.Permittable, error) {
	if d == nil {
		return nil, nil
	}

	uid, err := id.PermittableIDFrom(d.ID)
	if err != nil {
		return nil, err
	}

	userId, err := user.IDFrom(d.UserID)
	if err != nil {
		return nil, err
	}

	roleIds, err := id.RoleIDListFrom(d.UserRoles)
	if err != nil {
		return nil, err
	}

	return permittable.New().
		ID(uid).
		UserID(userId).
		RoleIDs(roleIds).
		Build()
}
