package permittable

import (
	"github.com/eukarya-inc/reearth-dashboard/pkg/id"
	"github.com/reearth/reearthx/account/accountdomain/user"
)

type Builder struct {
	p *Permittable
}

func New() *Builder {
	return &Builder{p: &Permittable{}}
}

func (b *Builder) Build() (*Permittable, error) {
	if b.p.id.IsNil() {
		return nil, ErrInvalidID
	}
	if b.p.userID.IsNil() {
		return nil, ErrInvalidID
	}
	return b.p, nil
}

func (b *Builder) MustBuild() *Permittable {
	u, err := b.Build()
	if err != nil {
		panic(err)
	}
	return u
}

func (b *Builder) ID(id ID) *Builder {
	b.p.id = id
	return b
}

func (b *Builder) NewID() *Builder {
	b.p.id = NewID()
	return b
}

func (b *Builder) UserID(userID user.ID) *Builder {
	b.p.userID = userID
	return b
}

func (b *Builder) RoleIDs(roleIDs []id.RoleID) *Builder {
	b.p.roleIDs = roleIDs
	return b
}
