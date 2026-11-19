package legal

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"maps"
	"slices"
	"time"
)

// TypeHead holds every version received for one type, so one compare-and-set
// of it applies a payload. Revision is the CAS token; 0 means never stored.
type TypeHead struct {
	Applied   Applied
	AppliedAt time.Time
	Revision  int64
	Type      Type
	Versions  map[Version]HeadVersion
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

// HeadVersion is one received version. WithdrawnAt is set when it was
// withdrawn before it took effect.
type HeadVersion struct {
	VersionRecord
	WithdrawnAt *time.Time
}

func (v HeadVersion) withdrawn() bool { return v.WithdrawnAt != nil }

func (v HeadVersion) inEffect(now time.Time) bool { return !v.EffectiveAt.After(now) }

type VersionRecord struct {
	Ref
	EffectiveAt time.Time
	Required    bool
	Title       LocalizedText
	URL         LocalizedText
}

func (e Entry) record() VersionRecord {
	return VersionRecord(e)
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
	hv, ok := h.Versions[v]
	return ok && hv.withdrawn()
}

// SortedVersions returns the head's versions in ascending version order.
func (h TypeHead) SortedVersions() []HeadVersion {
	vs := slices.Collect(maps.Values(h.Versions))
	slices.SortFunc(vs, func(a, b HeadVersion) int { return a.Version.Compare(b.Version) })
	return vs
}

// At returns the current version (not withdrawn, the latest effective date
// that has passed) and the upcoming version (not withdrawn, date not passed).
func (h TypeHead) At(now time.Time) (current, upcoming *VersionRecord) {
	for _, v := range h.Versions {
		if v.withdrawn() {
			continue
		}
		rec := v.VersionRecord
		switch {
		case !v.inEffect(now):
			if upcoming == nil || rec.EffectiveAt.Before(upcoming.EffectiveAt) {
				upcoming = &rec
			}
		case current == nil || later(rec, *current):
			current = &rec
		}
	}
	return current, upcoming
}

func later(a, b VersionRecord) bool {
	if !a.EffectiveAt.Equal(b.EffectiveAt) {
		return a.EffectiveAt.After(b.EffectiveAt)
	}
	return a.Version.Compare(b.Version) > 0
}

type Outcome int

const (
	// OutcomeIgnored leaves the head as it is.
	OutcomeIgnored Outcome = iota
	// OutcomeRecorded adds a version from a payload that lost to the head.
	OutcomeRecorded
	// OutcomeWon makes the entry the type's newest decision.
	OutcomeWon
)

// PlanApply decides one entry of a payload stamped payloadAt against the
// stored head.
//
// A payload newer than the head, or with the head's timestamp and an entry
// that beats the applied one (the higher version, then the larger digest),
// wins:
//   - A version whose date has passed never changes.
//   - A withdrawn version named again follows renamedWithdrawn.
//   - Any other named version takes the entry's content.
//   - Every other version not yet in effect is withdrawn.
//
// Any other payload only adds its version when the head has not seen it,
// as stalePayloadVersion decides.
func PlanApply(cur TypeHead, e Entry, payloadAt, now time.Time) (TypeHead, Outcome) {
	applied := e.applied()
	won := cur.Revision == 0 || payloadAt.After(cur.AppliedAt) || payloadAt.Equal(cur.AppliedAt) && applied.beats(cur.Applied)
	_, seen := cur.Versions[e.Version]
	if !won && seen {
		return cur, OutcomeIgnored
	}

	next := cur
	next.Revision++
	next.Versions = maps.Clone(cur.Versions)
	if next.Versions == nil {
		next.Versions = map[Version]HeadVersion{}
	}
	if !won {
		next.Versions[e.Version] = stalePayloadVersion(e, now)
		return next, OutcomeRecorded
	}

	next.Applied, next.AppliedAt = applied, payloadAt
	v, ok := next.Versions[e.Version]
	switch {
	case !ok:
		v = HeadVersion{VersionRecord: e.record()}
	case v.withdrawn():
		v = renamedWithdrawn(v, e)
	case !v.inEffect(now):
		v.VersionRecord = e.record()
	}
	next.Versions[e.Version] = v
	for ver, o := range next.Versions {
		if ver != e.Version && !o.withdrawn() && !o.inEffect(now) {
			next.Versions[ver] = withdraw(o, now)
		}
	}
	return next, OutcomeWon
}

// renamedWithdrawn keeps a withdrawn version withdrawn when the newest
// payload names it again, so that a withdrawn version never takes effect.
func renamedWithdrawn(v HeadVersion, _ Entry) HeadVersion { return v }

// stalePayloadVersion adds a version the head has not seen from a payload
// that lost to it. A version already in effect is added in effect and
// becomes current when its date is the latest that has passed. A version
// not yet in effect was superseded by the newer payload, so it is added
// withdrawn.
func stalePayloadVersion(e Entry, now time.Time) HeadVersion {
	v := HeadVersion{VersionRecord: e.record()}
	if !v.inEffect(now) {
		v = withdraw(v, now)
	}
	return v
}

func withdraw(v HeadVersion, now time.Time) HeadVersion {
	at := now.UTC().Truncate(time.Millisecond)
	v.WithdrawnAt = &at
	return v
}
