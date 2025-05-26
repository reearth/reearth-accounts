package user

import (
	"github.com/reearth/reearthx/account/accountdomain"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"golang.org/x/text/language"
)

type Builder struct {
	u            *User
	err          error
	passwordText string
	email        string
}

func New() *Builder {
	return &Builder{
		u: &User{},
	}
}

func (b *Builder) Build() (*User, error) {
	if b.err != nil {
		return nil, b.err
	}
	if b.u.id.IsEmpty() {
		return nil, ErrInvalidID
	}
	if b.u.name == "" {
		return nil, ErrInvalidName
	}
	if !b.u.theme.Valid() {
		b.u.theme = user.ThemeDefault
	}
	if b.passwordText != "" {
		if err := b.u.SetPassword(b.passwordText); err != nil {
			return nil, err
		}
	}

	if b.u.metadata != nil {
		b.u.SetMetadata(b.u.metadata)
	}

	return b.u, nil
}

func (b *Builder) MustBuild() *User {
	r, err := b.Build()
	if err != nil {
		panic(err)
	}
	return r
}

func (b *Builder) ID(id ID) *Builder {
	b.u.id = id
	return b
}

func (b *Builder) Name(name string) *Builder {
	b.u.name = name
	return b
}

func (b *Builder) Email(email string) *Builder {
	b.u.email = email
	return b
}

func (b *Builder) Alias(alias string) *Builder {
	b.u.alias = alias
	return b
}

func (b *Builder) Metadata(metadata *Metadata) *Builder {
	b.u.metadata = metadata
	return b
}

func (b *Builder) Workspace(workspace accountdomain.WorkspaceID) *Builder {
	b.u.workspace = workspace
	return b
}

func (b *Builder) LangFrom(lang string) *Builder {
	if lang == "" {
		b.u.lang = language.Und
	} else if l, err := language.Parse(lang); err == nil {
		b.u.lang = l
	}
	return b
}

func (b *Builder) Theme(t user.Theme) *Builder {
	b.u.theme = t
	return b
}

func (b *Builder) EncodedPassword(p user.EncodedPassword) *Builder {
	b.u.password = p.Clone()
	return b
}

func (b *Builder) Verification(v *user.Verification) *Builder {
	b.u.verification = v
	return b
}

func (b *Builder) PasswordReset(pr *user.PasswordReset) *Builder {
	b.u.passwordReset = pr
	return b
}

func (b *Builder) Auths(auths []user.Auth) *Builder {
	for _, a := range auths {
		b.u.AddAuth(a)
	}
	return b
}
