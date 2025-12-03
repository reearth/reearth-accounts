package mongodoc

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	permittable "github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/reearth/reearth-accounts/server/pkg/user"
)

type PermittableDocument struct {
	ID      string   `json:"id" jsonschema:"description=Permittable ID (ULID format)"`
	UserID  string   `json:"userid" jsonschema:"description=User ID this permittable represents"`
	RoleIDs []string `json:"roleids" jsonschema:"description=List of role IDs assigned to this user. Default: []"`
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
		ID:      id,
		UserID:  p.UserID().String(),
		RoleIDs: roleIds,
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

	roleIds, err := id.RoleIDListFrom(d.RoleIDs)
	if err != nil {
		return nil, err
	}

	return permittable.New().
		ID(uid).
		UserID(userId).
		RoleIDs(roleIds).
		Build()
}
