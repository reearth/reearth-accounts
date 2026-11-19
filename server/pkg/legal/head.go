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

// FreezeMargin is how long before its effective date a version stops
// changing when a payload is planned. It covers the time between planning
// and writing the head, and clock skew between instances.
const FreezeMargin = time.Minute

// HeadVersion is one received version. WithdrawnAt is set when it was
// withdrawn before it took effect. SetAt and SetDigest identify the payload
// entry its content came from. Revision is the head revision that last
// changed it.
type HeadVersion struct {
	VersionRecord
	Revision    int64
	SetAt       time.Time
	SetDigest   []byte
	WithdrawnAt *time.Time
}

func (v HeadVersion) withdrawn() bool { return v.WithdrawnAt != nil }

func (v HeadVersion) inEffect(now time.Time) bool { return !v.EffectiveAt.After(now) }

func (v HeadVersion) frozen(at time.Time) bool { return v.inEffect(at.Add(FreezeMargin)) }

// takes reports whether an entry of a payload stamped payloadAt replaces the
// content. A frozen version never changes, since readers may already see it
// in effect; the freeze skips a withdrawn one, which never took effect.
// Otherwise a newer payload does, and so does the same-timestamp entry with
// the larger digest, so either order ends the same.
func (v HeadVersion) takes(payloadAt time.Time, digest []byte, now time.Time) bool {
	if !v.withdrawn() && v.frozen(now) {
		return false
	}
	if payloadAt.Equal(v.SetAt) {
		return bytes.Compare(digest, v.SetDigest) > 0
	}
	return payloadAt.After(v.SetAt)
}

func (v HeadVersion) setBy(e Entry, payloadAt time.Time, a Applied) HeadVersion {
	v.VersionRecord, v.SetAt, v.SetDigest = e.record(), payloadAt, a.Digest
	return v
}

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

// ArchiveDue returns, in ascending version order, the versions whose
// archived revision is missing from archived or older than the revision that
// last changed them.
func (h TypeHead) ArchiveDue(archived map[Version]int64) []HeadVersion {
	var due []HeadVersion
	for _, v := range h.SortedVersions() {
		if rev, ok := archived[v.Version]; !ok || rev < v.Revision {
			due = append(due, v)
		}
	}
	return due
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
// stored head. A version is frozen from FreezeMargin before its date.
//
// A payload newer than the head, or with the head's timestamp and an entry
// that beats the applied one (the higher version, then the larger digest),
// wins:
//   - A withdrawn version named again follows renamedWithdrawn.
//   - Any other named version takes the entry's content unless it is frozen.
//   - Every other version not withdrawn and not frozen is withdrawn.
//
// Any other payload adds its version when the head has not seen it, as
// losingPayloadVersion decides, and otherwise only replaces the content of a
// version it wins a same-timestamp tie for that is withdrawn or not frozen.
func PlanApply(cur TypeHead, e Entry, payloadAt, now time.Time) (TypeHead, Outcome) {
	applied := e.applied()
	won := cur.Revision == 0 || payloadAt.After(cur.AppliedAt) || payloadAt.Equal(cur.AppliedAt) && applied.beats(cur.Applied)
	v, seen := cur.Versions[e.Version]
	tie := seen && payloadAt.Equal(v.SetAt) && v.takes(payloadAt, applied.Digest, now)
	if !won && seen && !tie {
		return cur, OutcomeIgnored
	}

	next := cur
	next.Revision++
	next.Versions = maps.Clone(cur.Versions)
	if next.Versions == nil {
		next.Versions = map[Version]HeadVersion{}
	}
	set := func(v HeadVersion) {
		v.Revision = next.Revision
		next.Versions[v.Version] = v
	}
	if !won {
		if seen {
			set(v.setBy(e, payloadAt, applied))
		} else {
			set(losingPayloadVersion(e, payloadAt, applied, payloadAt.Before(cur.AppliedAt), now))
		}
		return next, OutcomeRecorded
	}

	next.Applied, next.AppliedAt = applied, payloadAt
	switch {
	case !seen:
		set(HeadVersion{}.setBy(e, payloadAt, applied))
	case v.withdrawn():
		set(renamedWithdrawn(v, e, payloadAt, applied, now))
	case v.takes(payloadAt, applied.Digest, now):
		set(v.setBy(e, payloadAt, applied))
	}
	for ver, o := range next.Versions {
		if ver != e.Version && !o.withdrawn() && !o.frozen(now) {
			set(withdraw(o, now))
		}
	}
	return next, OutcomeWon
}

// renamedWithdrawn republishes a withdrawn version the newest payload names
// again, as upcoming with the entry's content, so that it ends the same as
// when the payloads arrive in the other order. A version that would be frozen
// stays withdrawn, so a republish never puts a version into effect at once.
func renamedWithdrawn(v HeadVersion, e Entry, payloadAt time.Time, a Applied, now time.Time) HeadVersion {
	next := v.setBy(e, payloadAt, a)
	if next.frozen(now) {
		return v
	}
	next.WithdrawnAt = nil
	return next
}

// losingPayloadVersion adds a version the head has not seen from a payload
// that lost to it. An older payload never puts a version into effect, so its
// version is added withdrawn whatever its date. A same-timestamp loser is
// judged as the winner judges the versions it does not name.
func losingPayloadVersion(e Entry, payloadAt time.Time, a Applied, older bool, now time.Time) HeadVersion {
	v := HeadVersion{}.setBy(e, payloadAt, a)
	if older || !v.frozen(now) {
		v = withdraw(v, now)
	}
	return v
}

func withdraw(v HeadVersion, now time.Time) HeadVersion {
	at := now.UTC().Truncate(time.Millisecond)
	v.WithdrawnAt = &at
	return v
}
