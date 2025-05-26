package workspace

import (
	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearthx/account/accountdomain/workspace"
)

type Builder struct {
	w            *Workspace
	members      map[workspace.UserID]workspace.Member
	integrations map[workspace.IntegrationID]workspace.Member
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
		b.w.members = workspace.NewMembers()
	} else {
		b.w.members = workspace.NewMembersWith(b.members, b.integrations, false)
	}

	if b.w.metadata != nil {
		b.w.SetMetadata(b.w.metadata)
	}
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
	b.w.id = id.NewWorkspaceID()
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.w.name = name
	return b
}

func (b *Builder) Alias(alias string) *Builder {
	b.w.alias = alias
	return b
}

func (b *Builder) Email(email string) *Builder {
	b.w.email = email
	return b
}

func (b *Builder) BillingEmail(billingEmail string) *Builder {
	b.w.billingEmail = billingEmail
	return b
}

func (b *Builder) Metadata(metadata *Metadata) *Builder {
	b.w.metadata = metadata
	return b
}

func (b *Builder) Members(members map[workspace.UserID]workspace.Member) *Builder {
	b.members = members
	return b
}

func (b *Builder) Integrations(integrations map[workspace.IntegrationID]workspace.Member) *Builder {
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
