package httpmodel

import (
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"golang.org/x/text/language"
)

// ParseUserID parses a string into a user ID.
func ParseUserID(s string) (id.UserID, error) { return id.UserIDFrom(s) }

// ParseWorkspaceID parses a string into a workspace ID.
func ParseWorkspaceID(s string) (id.WorkspaceID, error) { return id.WorkspaceIDFrom(s) }

// ParseIntegrationID parses a string into an integration ID.
func ParseIntegrationID(s string) (id.IntegrationID, error) { return id.IntegrationIDFrom(s) }

// ParseUserIDs parses a comma-joined or repeated list of user IDs.
func ParseUserIDs(ss []string) (user.IDList, error) {
	out := make(user.IDList, 0, len(ss))
	for _, s := range ss {
		uid, err := id.UserIDFrom(s)
		if err != nil {
			return nil, err
		}
		out = append(out, uid)
	}
	return out, nil
}

// ParseLang converts an optional BCP-47 string to a language tag pointer. It returns
// nil when the field is omitted/empty so update requests don't overwrite an existing
// language with "und"; a tag is only built when a value is actually provided.
func ParseLang(s *string) *language.Tag {
	if s == nil || *s == "" {
		return nil
	}
	t := language.Make(*s)
	return &t
}

// ParseTheme converts an optional theme string to a *user.Theme.
func ParseTheme(s *string) *user.Theme {
	if s == nil {
		return nil
	}
	th := user.ThemeDefault
	switch *s {
	case "dark":
		th = user.ThemeDark
	case "light":
		th = user.ThemeLight
	}
	return &th
}

// ParseRole converts a role string to role.RoleType.
func ParseRole(s string) role.RoleType {
	switch s {
	case "reader":
		return role.RoleReader
	case "writer":
		return role.RoleWriter
	case "maintainer":
		return role.RoleMaintainer
	case "owner":
		return role.RoleOwner
	}
	return role.RoleType("")
}

// RoleString converts role.RoleType to its API string.
func RoleString(r role.RoleType) string { return string(r) }
