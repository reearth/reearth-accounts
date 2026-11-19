package legal

import (
	"context"
	"time"
)

// VersionRepo stores policy_types (one head per type) and policy_versions
// (one record per type and version). Every write is a single-document or
// single-statement conditional write.
type VersionRepo interface {
	// CompareAndSwapHead stores next when the stored revision is
	// next.Revision-1 (an absent head counts as 0). It returns false when
	// another writer won.
	CompareAndSwapHead(ctx context.Context, next TypeHead) (bool, error)
	// FindHead returns TypeHead{Type: t} when t has no head.
	FindHead(ctx context.Context, t Type) (TypeHead, error)
	FindHeads(ctx context.Context) ([]TypeHead, error)
	FindVersions(ctx context.Context) ([]VersionRecord, error)
	// UpsertVersion inserts r, or replaces the stored record only when
	// stored.CanOverwrite(r.Stamp, now).
	UpsertVersion(ctx context.Context, r VersionRecord, now time.Time) error
}
