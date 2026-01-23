package gqlmodel

import (
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/samber/lo"

	"github.com/reearth/reearthx/util"
)

func ToUser(u *user.User) *User {
	if u == nil {
		return nil
	}

	metadata := UserMetadata{
		Description: u.Metadata().Description(),
		Lang:        u.Metadata().Lang().String(),
		PhotoURL:    u.Metadata().PhotoURL(),
		Theme:       Theme(u.Metadata().Theme()),
		Website:     u.Metadata().Website(),
	}

	var v *Verification
	if u.Verification() != nil {
		v = &Verification{
			Code:       u.Verification().Code(),
			Expiration: u.Verification().Expiration().Format("2006-01-02T15:04:05.000Z"),
			Verified:   u.Verification().IsVerified(),
		}
	}

	return &User{
		ID:        IDFrom(u.ID()),
		Name:      u.Name(),
		Alias:     u.Alias(),
		Email:     u.Email(),
		Host:      lo.EmptyableToPtr(u.Host()),
		Workspace: IDFrom(u.Workspace()),
		Auths: util.Map(u.Auths(), func(a user.Auth) string {
			return a.Provider
		}),
		Metadata:     &metadata,
		Verification: v,
	}
}

func ToUsers(ul user.List) []*User {
	if ul == nil {
		return nil
	}

	users := make([]*User, 0, len(ul))
	for _, u := range ul {
		users = append(users, ToUser(u))
	}
	return users
}

func ToUserFromSimple(u *user.Simple) *User {
	if u == nil {
		return nil
	}

	return &User{
		ID:    IDFrom(u.ID),
		Name:  u.Name,
		Email: u.Email,
	}
}

func ToUserForAuthorization(u *user.User) *User {
	if u == nil {
		return nil
	}

	return &User{
		ID:    IDFrom(u.ID()),
		Name:  u.Name(),
		Email: u.Email(),
	}
}

func ToMe(u *user.User) *Me {
	if u == nil {
		return nil
	}

	metadata := UserMetadata{
		Description: u.Metadata().Description(),
		Lang:        u.Metadata().Lang().String(),
		PhotoURL:    u.Metadata().PhotoURL(),
		Theme:       Theme(u.Metadata().Theme()),
		Website:     u.Metadata().Website(),
	}

	return &Me{
		ID:            IDFrom(u.ID()),
		Name:          u.Name(),
		Alias:         u.Alias(),
		Email:         u.Email(),
		Metadata:      &metadata,
		MyWorkspaceID: IDFrom(u.Workspace()),
		Auths: util.Map(u.Auths(), func(a user.Auth) string {
			return a.Provider
		}),
	}
}

func ToTheme(t *Theme) *user.Theme {
	if t == nil {
		return nil
	}

	th := user.ThemeDefault
	switch *t {
	case ThemeDark:
		th = user.ThemeDark
	case ThemeLight:
		th = user.ThemeLight
	}
	return &th
}

func FromRole(r Role) role.RoleType {
	switch r {
	case RoleReader:
		return role.RoleReader
	case RoleWriter:
		return role.RoleWriter
	case RoleMaintainer:
		return role.RoleMaintainer
	case RoleOwner:
		return role.RoleOwner
	}
	return role.RoleType("")
}

func ToRole(r role.RoleType) Role {
	switch r {
	case role.RoleReader:
		return RoleReader
	case role.RoleWriter:
		return RoleWriter
	case role.RoleMaintainer:
		return RoleMaintainer
	case role.RoleOwner:
		return RoleOwner
	}
	return Role("")
}
