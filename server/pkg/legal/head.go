package legal

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"slices"
	"time"
)

// TypeHead holds every decision about one type, so one compare-and-set of
// it applies a payload. Revision is the CAS token; 0 means never stored.
type TypeHead struct {
	Applied   Applied
	AppliedAt time.Time
	Current   *VersionRecord
	Revision  int64
	Type      Type
	Upcoming  *VersionRecord
	Withdrawn []Withdrawal
}

// Applied identifies the entry of the payload stamped AppliedAt, so that a
// second payload with the same timestamp is resolved the same in either
// order.
type Applied struct {
	Digest  []byte
	Version Version
}

func (a Applied) beats(o Applied) bool {
	if c := a.Version.Compare(o.Version); c != 0 {
		return c > 0
	}
	return bytes.Compare(a.Digest, o.Digest) > 0
}

// Withdrawal records when a version stopped being upcoming. Only a version
// that never took effect is withdrawn.
type Withdrawal struct {
	At      time.Time
	Version Version
}

// VersionRecord is the content of one version of one type.
type VersionRecord struct {
	Ref
	EffectiveAt time.Time
	Required    bool
	Title       LocalizedText
	URL         LocalizedText
}

func (e Entry) record() VersionRecord {
	return VersionRecord{Ref: e.Ref, EffectiveAt: e.EffectiveAt, Required: e.Required, Title: e.Title, URL: e.URL}
}

func (e Entry) applied() Applied {
	// json.Marshal sorts map keys, so equal entries give equal bytes.
	b, _ := json.Marshal(struct {
		EffectiveAt time.Time
		Required    bool
		Title       LocalizedText
		URL         LocalizedText
		Version     string
	}{e.EffectiveAt.UTC(), e.Required, e.Title, e.URL, e.Version.String()})
	sum := sha256.Sum256(b)
	return Applied{Digest: sum[:], Version: e.Version}
}

func (h TypeHead) IsWithdrawn(v Version) bool {
	return slices.ContainsFunc(h.Withdrawn, func(w Withdrawal) bool { return w.Version == v })
}

// At returns the current and upcoming versions at now. An upcoming version
// whose date has passed is current.
func (h TypeHead) At(now time.Time) (current, upcoming *VersionRecord) {
	if u := h.Upcoming; u != nil && !u.EffectiveAt.After(now) {
		return u, nil
	}
	return h.Current, h.Upcoming
}

// PlanApply decides one entry of a payload stamped payloadAt against the
// stored head. It returns false when the head stays as it is.
//
//   - An older payload changes nothing. A payload with the head's own
//     timestamp changes the head only when its entry beats the applied one:
//     the higher version, then the larger digest.
//   - An upcoming version whose date has passed becomes current first.
//   - A withdrawn version stays withdrawn; naming it again is ignored.
//   - A version that has taken effect never changes.
//   - An entry not yet in effect becomes the upcoming version, and replaces
//     the content of the same upcoming version.
//   - An entry already in effect becomes current when its date is after the
//     current version's date.
//   - The previous upcoming version is withdrawn unless the entry keeps it
//     upcoming or makes it current.
func PlanApply(cur TypeHead, e Entry, payloadAt, now time.Time) (TypeHead, bool) {
	applied := e.applied()
	if cur.Revision > 0 && (payloadAt.Before(cur.AppliedAt) || payloadAt.Equal(cur.AppliedAt) && !applied.beats(cur.Applied)) {
		return cur, false
	}

	next := cur
	next.Revision++
	next.AppliedAt = payloadAt
	next.Applied = applied
	next.Current, next.Upcoming = cur.At(now)
	next.Withdrawn = slices.Clone(cur.Withdrawn)

	rec := e.record()
	prev := next.Upcoming
	next.Upcoming = nil
	switch {
	case next.IsWithdrawn(e.Version), next.Current != nil && next.Current.Version == e.Version:
	case e.EffectiveAt.After(now):
		next.Upcoming = &rec
	case next.Current == nil || later(rec, *next.Current):
		next.Current = &rec
	}
	if prev != nil && !next.names(prev.Version) {
		next.Withdrawn = append(next.Withdrawn, Withdrawal{At: now.Truncate(time.Millisecond), Version: prev.Version})
	}
	return next, true
}

func (h TypeHead) names(v Version) bool {
	return h.Current != nil && h.Current.Version == v || h.Upcoming != nil && h.Upcoming.Version == v
}

// later orders versions in effect: the later date, then the higher version.
func later(a, b VersionRecord) bool {
	if !a.EffectiveAt.Equal(b.EffectiveAt) {
		return a.EffectiveAt.After(b.EffectiveAt)
	}
	return a.Version.Compare(b.Version) > 0
}
