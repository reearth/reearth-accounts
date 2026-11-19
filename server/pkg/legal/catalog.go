package legal

import (
	"slices"
	"time"
)

// Catalog is a read snapshot of every type's head and version records.
type Catalog struct {
	heads    map[Type]TypeHead
	versions map[Type]map[Version]VersionRecord
}

func NewCatalog(heads []TypeHead, records []VersionRecord) Catalog {
	c := Catalog{
		heads:    make(map[Type]TypeHead, len(heads)),
		versions: make(map[Type]map[Version]VersionRecord),
	}
	for _, h := range heads {
		c.heads[h.Type] = h
	}
	for _, r := range records {
		if c.versions[r.Type] == nil {
			c.versions[r.Type] = make(map[Version]VersionRecord)
		}
		c.versions[r.Type][r.Version] = r
	}
	return c
}

// Current is the version not withdrawn with the latest effective date that
// has passed, or nil before any version of t takes effect.
func (c Catalog) Current(t Type, now time.Time) *VersionRecord {
	var cur *VersionRecord
	for _, r := range c.versions[t] {
		if !r.InEffect(now) || c.Withdrawn(r.Ref) {
			continue
		}
		if cur == nil || r.EffectiveAt.After(cur.EffectiveAt) ||
			(r.EffectiveAt.Equal(cur.EffectiveAt) && r.Version.Compare(cur.Version) > 0) {
			r := r
			cur = &r
		}
	}
	return cur
}

// Upcoming is the head's upcoming version when its record exists and its
// date has not passed. PlanApply never names a withdrawn version upcoming.
func (c Catalog) Upcoming(t Type, now time.Time) *VersionRecord {
	u := c.heads[t].Upcoming
	if u == nil {
		return nil
	}
	r, ok := c.versions[t][u.Version]
	if !ok || r.InEffect(now) {
		return nil
	}
	return &r
}

// Types lists the types that have a head, sorted.
func (c Catalog) Types() []Type {
	ts := make([]Type, 0, len(c.heads))
	for t := range c.heads {
		ts = append(ts, t)
	}
	slices.Sort(ts)
	return ts
}

// Withdrawn reports whether ref was withdrawn before it took effect. Once
// withdrawn, it never takes effect.
func (c Catalog) Withdrawn(ref Ref) bool {
	w, ok := c.heads[ref.Type].WithdrawalOf(ref.Version)
	if !ok {
		return false
	}
	r, ok := c.versions[ref.Type][ref.Version]
	return !ok || r.EffectiveAt.After(w.At)
}
