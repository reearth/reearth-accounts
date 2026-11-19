package legal

import (
	"context"
	"fmt"
	"time"
)

const maxApplyAttempts = 64

type ApplyResult struct {
	Applied []Type
	Ignored []Type
}

// Apply writes a validated manifest one type at a time. Per type, one
// compare-and-set of the head decides; the archive is written afterwards
// from the head, so a failed archive write is repaired by the next apply of
// that type.
func Apply(ctx context.Context, r VersionRepo, m Manifest, now func() time.Time) (ApplyResult, error) {
	var res ApplyResult
	for _, e := range m.Entries {
		won, err := applyEntry(ctx, r, e, m.Timestamp, now)
		if won {
			res.Applied = append(res.Applied, e.Type)
		} else if err == nil {
			res.Ignored = append(res.Ignored, e.Type)
		}
		if err != nil {
			return res, err
		}
	}
	return res, nil
}

func applyEntry(ctx context.Context, r VersionRepo, e Entry, payloadAt time.Time, now func() time.Time) (bool, error) {
	for range maxApplyAttempts {
		cur, err := r.FindHead(ctx, e.Type)
		if err != nil {
			return false, err
		}
		next, outcome := PlanApply(cur, e, payloadAt, now())
		if outcome != OutcomeIgnored {
			won, err := r.CompareAndSwapHead(ctx, next)
			if err != nil {
				return false, err
			}
			if !won {
				continue
			}
		}
		return outcome == OutcomeWon, r.ArchiveVersions(ctx, next)
	}
	return false, fmt.Errorf("legal: %s: head compare-and-set lost %d times", e.Type, maxApplyAttempts)
}
