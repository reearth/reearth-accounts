package outbox

import (
	"context"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
)

// Repo stores mail_outbox. Every method is one conditional write per record
// and needs no transaction.
type Repo interface {
	// ClaimDue moves one due pending record, the earliest NextAttemptAt
	// first, to sending under c and returns it. It returns nil when none is
	// due. Two callers never claim the same record.
	ClaimDue(ctx context.Context, now time.Time, c Claim) (*Record, error)
	// Complete writes t only while token holds the record and its claim has
	// not expired at now. It returns false when the claim no longer holds.
	Complete(ctx context.Context, k Key, token string, t Transition, now time.Time) (bool, error)
	// Enqueue stores each record whose key is absent and leaves existing
	// records unchanged.
	Enqueue(ctx context.Context, rs []Record) error
	FindByKey(ctx context.Context, k Key) (*Record, error)
	RemoveByUser(ctx context.Context, u id.UserID) error
	// Sweep turns expired claims unknown, then stale pending records failed,
	// then deletes records retired under ttl.
	Sweep(ctx context.Context, now time.Time, ttl time.Duration) (SweepResult, error)
}
