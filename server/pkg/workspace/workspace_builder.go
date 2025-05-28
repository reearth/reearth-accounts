package workspace

import (
	"errors"

	"github.com/reearth/reearthx/util"
)

var ErrMembersRequired = errors.New("members required")

type Builder struct {
	w            *Workspace
	members      map[UserID]Member
	integrations map[IntegrationID]Member
	personal     bool
	err          error
}

func New() *Builder {
	return &Builder{w: &Workspace{}}
}

func (b *Builder) Build() (*Workspace, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.w.id.IsEmpty() {
		return nil, ErrInvalidID
	}

	if b.members == nil && b.integrations == nil {
		b.w.members = NewMembers()
	} else {
		b.w.members = NewMembersWith(b.members, b.integrations, false)
	}
	b.w.members.fixed = b.personal
	return b.w, nil
}

func (b *Builder) MustBuild() *Workspace {
	r, err := b.Build()
	if err != nil {
		panic(err)
	}
	return r
}

func (b *Builder) ID(id ID) *Builder {
	b.w.id = id
	return b
}

func (b *Builder) ParseID(id string) *Builder {
	b.w.id, b.err = IDFrom(id)
	return b
}

func (b *Builder) NewID() *Builder {
	b.w.id = NewID()
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.w.name = name
	return b
}

func (b *Builder) DisplayName(displayName string) *Builder {
	b.w.displayName = displayName
	return b
}

func (b *Builder) Members(members map[UserID]Member) *Builder {
	b.members = members
	return b
}

func (b *Builder) Integrations(integrations map[IntegrationID]Member) *Builder {
	b.integrations = integrations
	return b
}

func (b *Builder) Personal(p bool) *Builder {
	b.personal = p
	return b
}

func (b *Builder) Location(location string) *Builder {
	b.w.location = location
	return b
}

func (b *Builder) Policy(p *PolicyID) *Builder {
	b.w.policy = util.CloneRef(p)
	return b
}
