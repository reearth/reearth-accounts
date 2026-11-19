package legal

import "context"

// VersionRepo stores policy_types (one head per type, which decides) and
// policy_versions (one archived record per type and version, which nothing
// reads to decide). Every write is a single-document or single-statement
// conditional write.
type VersionRepo interface {
	// ArchiveVersions upserts every version of h into policy_versions,
	// stamped with h.Revision. A stored record with a higher revision is kept.
	ArchiveVersions(ctx context.Context, h TypeHead) error
	// CompareAndSwapHead stores next when the stored revision is
	// next.Revision-1 (an absent head counts as 0). It returns false when
	// another writer won.
	CompareAndSwapHead(ctx context.Context, next TypeHead) (bool, error)
	// FindHead returns TypeHead{Type: t} when t has no head.
	FindHead(ctx context.Context, t Type) (TypeHead, error)
	FindHeads(ctx context.Context) ([]TypeHead, error)
	FindVersions(ctx context.Context) ([]VersionRecord, error)
}
