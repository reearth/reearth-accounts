package legal

import (
	"slices"
	"time"
)

// Catalog is a read snapshot of every type's head.
type Catalog struct {
	heads map[Type]TypeHead
}

func NewCatalog(heads []TypeHead) Catalog {
	c := Catalog{heads: make(map[Type]TypeHead, len(heads))}
	for _, h := range heads {
		c.heads[h.Type] = h
	}
	return c
}

// Current is the version of t in effect at now, or nil before any version
// of t takes effect.
func (c Catalog) Current(t Type, now time.Time) *VersionRecord {
	cur, _ := c.heads[t].At(now)
	return cur
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

// Upcoming is the version of t that takes effect after now, if any.
func (c Catalog) Upcoming(t Type, now time.Time) *VersionRecord {
	_, up := c.heads[t].At(now)
	return up
}

// Withdrawn reports whether ref is withdrawn. A withdrawn version is never
// current, though the newest payload can republish it as upcoming.
func (c Catalog) Withdrawn(ref Ref) bool {
	return c.heads[ref.Type].IsWithdrawn(ref.Version)
}
