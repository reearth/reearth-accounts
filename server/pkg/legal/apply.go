package legal

import (
	"context"
	"fmt"
	"time"
)

// maxApplyAttempts bounds the compare-and-set loop. Each lost attempt means
// another payload won, so concurrent deliveries settle far below it.
const maxApplyAttempts = 64

type ApplyResult struct {
	Applied []Type // the head changed
	Ignored []Type // an older payload or a replay
}

// Apply writes a validated manifest one type at a time. Per type, it first
// wins the head compare-and-set and only then upserts the version record the
// decision names, so no record exists that no head decision named.
func Apply(ctx context.Context, r VersionRepo, m Manifest, now func() time.Time) (ApplyResult, error) {
	var res ApplyResult
	for _, e := range m.Entries {
		applied, err := applyEntry(ctx, r, e, m.Timestamp, now)
		if err != nil {
			return res, err
		}
		if applied {
			res.Applied = append(res.Applied, e.Type)
		} else {
			res.Ignored = append(res.Ignored, e.Type)
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
		d := PlanApply(cur, e, payloadAt, now())
		if d.Next != nil {
			won, err := r.CompareAndSwapHead(ctx, *d.Next)
			if err != nil {
				return false, err
			}
			if !won {
				continue
			}
		}
		if d.Write {
			if err := r.UpsertVersion(ctx, NewVersionRecord(e, payloadAt), now()); err != nil {
				return false, err
			}
		}
		return d.Next != nil, nil
	}
	return false, fmt.Errorf("legal: %s: head compare-and-set lost %d times", e.Type, maxApplyAttempts)
}
