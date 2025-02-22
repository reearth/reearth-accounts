package mongodoc

import (
	"github.com/eukarya-inc/reearth-accounts/pkg/id"
	permittable "github.com/eukarya-inc/reearth-accounts/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

type PermittableDocument struct {
	ID      string
	UserID  string
	RoleIDs []string
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
