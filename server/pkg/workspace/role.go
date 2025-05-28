package workspace

import (
	"errors"
	"strings"

	"golang.org/x/exp/slices"
)

var (
	// RoleOwner is a role who can have full controll of projects and workspaces
	RoleOwner = Role("owner")
	// RoleMaintainer is a role who can manage projects
	RoleMaintainer = Role("maintainer")
	// RoleWriter is a role who can read and write projects
	RoleWriter = Role("writer")
	// RoleReader is a role who can read projects
	RoleReader = Role("reader")

	roles = []Role{
		RoleOwner,
		RoleMaintainer,
		RoleWriter,
		RoleReader,
	}

	ErrInvalidRole = errors.New("invalid role")
)

type Role string

func (r Role) Valid() bool {
	return slices.Contains(roles, r)
}

func RoleFrom(r string) (Role, error) {
	role := Role(strings.ToLower(r))
	if role.Valid() {
		return role, nil
	}
	return role, ErrInvalidRole
}

func (r Role) Includes(role Role) bool {
	if !r.Valid() {
		return false
	}

	for i, r2 := range roles {
		if r == r2 {
			for _, r3 := range roles[i:] {
				if role == r3 {
					return true
				}
			}
		}
	}
	return false
}
