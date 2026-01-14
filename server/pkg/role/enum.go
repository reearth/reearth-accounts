package role

import (
	"errors"
	"slices"
	"strings"
)

var (
	// RoleOwner is a role who can have full controll of projects and workspaces
	RoleOwner = RoleType("owner")
	// RoleMaintainer is a role who can manage projects
	RoleMaintainer = RoleType("maintainer")
	// RoleWriter is a role who can read and write projects
	RoleWriter = RoleType("writer")
	// RoleReader is a role who can read projects
	RoleReader = RoleType("reader")

	roleTypes = []RoleType{
		RoleOwner,
		RoleMaintainer,
		RoleWriter,
		RoleReader,
	}

	ErrInvalidRole = errors.New("invalid role")
)

type RoleType string

func (r RoleType) Valid() bool {
	return slices.Contains(roleTypes, r)
}

func (r RoleType) String() string {
	return string(r)
}

func RoleFrom(r string) (RoleType, error) {
	role := RoleType(strings.ToLower(r))
	if role.Valid() {
		return role, nil
	}
	return role, ErrInvalidRole
}

func (r RoleType) Includes(role RoleType) bool {
	if !r.Valid() {
		return false
	}

	for i, r2 := range roleTypes {
		if r == r2 {
			for _, r3 := range roleTypes[i:] {
				if role == r3 {
					return true
				}
			}
		}
	}
	return false
}
