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
