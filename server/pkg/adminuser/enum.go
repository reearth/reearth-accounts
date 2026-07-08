package adminuser

import (
	"errors"
	"slices"
	"strings"
)

var (
	// StatusPending is the initial status of a newly signed-up admin user
	// awaiting approval from an existing approved admin.
	StatusPending = Status("pending")
	// StatusApproved is the status of an admin user who has been approved and
	// can access admin features.
	StatusApproved = Status("approved")
	// StatusRejected is the status of an admin user whose access has been
	// rejected or revoked.
	StatusRejected = Status("rejected")

	statuses = []Status{
		StatusPending,
		StatusApproved,
		StatusRejected,
	}

	ErrInvalidStatus = errors.New("invalid status")
)

var (
	// RoleSystemAdmin is a role with full administrative privileges over the
	// admin application.
	RoleSystemAdmin = Role("system_admin")
	// RoleViewer is a role with read-only access to the admin application.
	RoleViewer = Role("viewer")

	roles = []Role{
		RoleSystemAdmin,
		RoleViewer,
	}

	ErrInvalidRole = errors.New("invalid role")
)

type Status string

func (s Status) Valid() bool {
	return slices.Contains(statuses, s)
}

func (s Status) String() string {
	return string(s)
}

func StatusFrom(s string) (Status, error) {
	status := Status(strings.ToLower(s))
	if status.Valid() {
		return status, nil
	}
	return status, ErrInvalidStatus
}

type Role string

func (r Role) Valid() bool {
	return slices.Contains(roles, r)
}

func (r Role) String() string {
	return string(r)
}

func RoleFrom(s string) (Role, error) {
	role := Role(strings.ToLower(s))
	if role.Valid() {
		return role, nil
	}
	return role, ErrInvalidRole
}
