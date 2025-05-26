package user

import (
	"errors"
	"slices"

	"github.com/reearth/reearthx/account/accountdomain"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"golang.org/x/text/language"
)

type User struct {
	id            ID
	name          string
	alias         string
	email         string
	metadata      *Metadata
	password      user.EncodedPassword
	workspace     accountdomain.WorkspaceID
	auths         []user.Auth
	lang          language.Tag
	theme         user.Theme
	verification  *user.Verification
	passwordReset *user.PasswordReset
	host          string
}

var ErrInvalidName = errors.New("invalid user name")

func (u *User) ID() ID {
	if u == nil {
		return ID{}
	}
	return u.id
}

func (u *User) Name() string {
	if u == nil {
		return ""
	}
	return u.name
}

func (u *User) Alias() string {
	if u == nil {
		return ""
	}
	return u.alias
}

func (u *User) Email() string {
	if u == nil {
		return ""
	}
	return u.email
}

func (u *User) SetAlias(alias string) {
	if u == nil {
		return
	}
	u.alias = alias
}

func (u *User) Metadata() *Metadata {
	if u == nil {
		return nil
	}
	return u.metadata
}

func (u *User) SetMetadata(metadata *Metadata) {
	if u == nil {
		return
	}
	u.metadata = metadata
}

func (u *User) Workspace() accountdomain.WorkspaceID {
	if u == nil {
		return accountdomain.WorkspaceID{}
	}
	return u.workspace
}

func (u *User) Verification() *user.Verification {
	if u == nil {
		return nil
	}
	return u.verification
}

func (u *User) Password() []byte {
	if u == nil {
		return nil
	}
	return u.password
}

func (u *User) SetPassword(pass string) error {
	p, err := user.NewEncodedPassword(pass)
	if err != nil {
		return err
	}

	u.password = p
	return nil
}

func (u *User) PasswordReset() *user.PasswordReset {
	if u == nil {
		return nil
	}
	return u.passwordReset
}

func (u *User) Lang() language.Tag {
	if u == nil {
		return language.Und
	}
	return u.lang
}

func (u *User) Theme() user.Theme {
	if u == nil {
		return user.ThemeDefault
	}
	return u.theme
}

func (u *User) Auths() []user.Auth {
	if u == nil {
		return nil
	}
	return slices.Clone(u.auths)
}

func (u *User) ContainAuth(a user.Auth) bool {
	if u == nil {
		return false
	}
	for _, b := range u.auths {
		if a == b || a.Provider == b.Provider {
			return true
		}
	}
	return false
}

func (u *User) AddAuth(a user.Auth) bool {
	if u == nil {
		return false
	}
	if !u.ContainAuth(a) {
		u.auths = append(u.auths, a)
		return true
	}
	return false
}
