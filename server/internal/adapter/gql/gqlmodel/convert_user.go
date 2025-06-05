package gqlmodel

import (
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearth-accounts/pkg/workspace"

	"github.com/reearth/reearthx/util"
)

func ToUser(u *user.User) *User {
	if u == nil {
		return nil
	}

	return &User{
		ID:    IDFrom(u.ID()),
		Name:  u.Name(),
		Email: u.Email(),
	}
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

	var metadata UserMetadata
	if u.Metadata() != nil {
		metadata = UserMetadata{
			Description: optionalString(u.Metadata().Description()),
			Lang:        u.Metadata().Lang().String(),
			PhotoURL:    optionalString(u.Metadata().PhotoURL()),
			Theme:       Theme(u.Metadata().Theme()),
			Website:     optionalString(u.Metadata().Website()),
		}
	}

	return &Me{
		ID:            IDFrom(u.ID()),
		Name:          u.Name(),
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

func ToWorkspace(t *workspace.Workspace) *Workspace {
	if t == nil {
		return nil
	}

	usersMap := t.Members().Users()
	integrationsMap := t.Members().Integrations()
	members := make([]WorkspaceMember, 0, len(usersMap)+len(integrationsMap))
	for u, m := range usersMap {
		members = append(members, &WorkspaceUserMember{
			UserID: IDFrom(u),
			Role:   ToRole(m.Role),
		})
	}

	return &Workspace{
		ID:       IDFrom(t.ID()),
		Name:     t.Name(),
		Personal: t.IsPersonal(),
		Members:  members,
	}
}

func ToWorkspaces(ws workspace.List) []*Workspace {
	if ws == nil {
		return nil
	}

	workspaces := make([]*Workspace, 0, len(ws))
	for _, w := range ws {
		workspaces = append(workspaces, ToWorkspace(w))
	}
	return workspaces
}

func FromRole(r Role) workspace.Role {
	switch r {
	case RoleReader:
		return workspace.RoleReader
	case RoleWriter:
		return workspace.RoleWriter
	case RoleMaintainer:
		return workspace.RoleMaintainer
	case RoleOwner:
		return workspace.RoleOwner
	}
	return workspace.Role("")
}

func ToRole(r workspace.Role) Role {
	switch r {
	case workspace.RoleReader:
		return RoleReader
	case workspace.RoleWriter:
		return RoleWriter
	case workspace.RoleMaintainer:
		return RoleMaintainer
	case workspace.RoleOwner:
		return RoleOwner
	}
	return Role("")
}

func optionalString(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}
