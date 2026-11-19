package legal

import (
	"slices"
	"time"
)

// TypeHead is the decision state of one type, written by one compare-and-set
// per payload. Revision is the CAS token; 0 means never stored.
type TypeHead struct {
	AppliedAt time.Time
	Revision  int64
	Type      Type
	Upcoming  *Scheduled
	Withdrawn []Withdrawal
}

// Scheduled is the upcoming version as of the payload that named it.
type Scheduled struct {
	EffectiveAt time.Time
	Version     Version
}

// Withdrawal records when a version stopped being upcoming. It counts only
// for a version whose effective date is after At: a version already in effect
// is never withdrawn.
type Withdrawal struct {
	At      time.Time
	Version Version
}

func (h TypeHead) WithdrawalOf(v Version) (Withdrawal, bool) {
	i := slices.IndexFunc(h.Withdrawn, func(w Withdrawal) bool { return w.Version == v })
	if i < 0 {
		return Withdrawal{}, false
	}
	return h.Withdrawn[i], true
}

// VersionRecord is one saved version of one type. Stamp is the timestamp of
// the payload that last wrote it.
type VersionRecord struct {
	Ref
	EffectiveAt time.Time
	Required    bool
	Stamp       time.Time
	Title       LocalizedText
	URL         LocalizedText
}

func NewVersionRecord(e Entry, stamp time.Time) VersionRecord {
	return VersionRecord{
		Ref:         e.Ref,
		EffectiveAt: e.EffectiveAt,
		Required:    e.Required,
		Stamp:       stamp,
		Title:       e.Title,
		URL:         e.URL,
	}
}

func (r VersionRecord) InEffect(now time.Time) bool { return !r.EffectiveAt.After(now) }

// CanOverwrite reports whether a write stamped stamp may replace r: r must not
// be newer and must not be in effect.
func (r VersionRecord) CanOverwrite(stamp, now time.Time) bool {
	return !r.Stamp.After(stamp) && !r.InEffect(now)
}

// Decision is what one payload entry does to one type. Next nil keeps the
// stored head. Write upserts the entry's version record, which happens only
// after Next (if any) has won its compare-and-set.
type Decision struct {
	Next  *TypeHead
	Write bool
}

// PlanApply decides one entry of a payload stamped payloadAt against the
// stored head. It reads no version record, so a record that is not written
// yet cannot change the decision.
//
//   - An older payload changes nothing.
//   - A payload with the head's own timestamp is a replay: the head stays and
//     the record is rewritten only when the head names the entry.
//   - A withdrawn version stays withdrawn. Naming it again is ignored, and the
//     upcoming version is withdrawn as for any entry that is not upcoming.
//   - The entry becomes the upcoming version when its date has not passed.
//     The previous upcoming version, if it is another one, is withdrawn;
//     a withdrawal does not count for a version already in effect.
func PlanApply(cur TypeHead, e Entry, payloadAt, now time.Time) Decision {
	switch {
	case payloadAt.Before(cur.AppliedAt):
		return Decision{}
	case payloadAt.Equal(cur.AppliedAt) && cur.Revision > 0:
		return Decision{Write: names(cur, e, now)}
	}

	next := cur
	next.Revision = cur.Revision + 1
	next.AppliedAt = payloadAt
	next.Upcoming = nil
	next.Withdrawn = slices.Clone(cur.Withdrawn)

	_, withdrawn := cur.WithdrawalOf(e.Version)
	upcoming := !withdrawn && e.EffectiveAt.After(now)
	frozen := false
	if u := cur.Upcoming; u != nil {
		switch {
		case u.Version == e.Version && !u.EffectiveAt.After(now):
			frozen = true
			upcoming = false
		case u.Version != e.Version:
			next.Withdrawn = append(next.Withdrawn, Withdrawal{At: now, Version: u.Version})
		}
	}
	if upcoming {
		next.Upcoming = &Scheduled{EffectiveAt: e.EffectiveAt, Version: e.Version}
	}
	return Decision{Next: &next, Write: !withdrawn && !frozen}
}

// names reports whether the head, unchanged since a payload with its own
// timestamp, decided to write e.
func names(h TypeHead, e Entry, now time.Time) bool {
	if u := h.Upcoming; u != nil {
		return u.Version == e.Version && u.EffectiveAt.Equal(e.EffectiveAt)
	}
	_, withdrawn := h.WithdrawalOf(e.Version)
	return !withdrawn && !e.EffectiveAt.After(now)
}
