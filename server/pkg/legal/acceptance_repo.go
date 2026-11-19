package legal

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// AcceptanceRepo stores policy_acceptances, unique on (user, type, version).
type AcceptanceRepo interface {
	// FindByUser returns every record of u in no particular order.
	FindByUser(ctx context.Context, u id.UserID) ([]Acceptance, error)
	// FindUsersAfter returns up to limit distinct users that hold a record,
	// in ID order, starting after the given user (nil starts at the first).
	FindUsersAfter(ctx context.Context, after *id.UserID, limit int) ([]id.UserID, error)
	// InsertIfAbsent stores each record whose (user, type, version) is not
	// stored yet and keeps the stored one otherwise, so a retry is safe.
	InsertIfAbsent(ctx context.Context, as []Acceptance) error
	// Remove deletes each stored record whose user, type, version and
	// AcceptedAt match one of as, so a record another call stored stays.
	Remove(ctx context.Context, as []Acceptance) error
	RemoveByUser(ctx context.Context, u id.UserID) error
}
