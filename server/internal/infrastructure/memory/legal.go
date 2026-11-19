package memory

import (
	"context"
	"maps"
	"slices"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type PolicyVersion struct {
	heads    map[legal.Type]legal.TypeHead
	lock     sync.Mutex
	versions map[legal.Ref]legal.VersionRecord
}

func NewPolicyVersion() *PolicyVersion {
	return &PolicyVersion{
		heads:    map[legal.Type]legal.TypeHead{},
		versions: map[legal.Ref]legal.VersionRecord{},
	}
}

func (r *PolicyVersion) CompareAndSwapHead(_ context.Context, next legal.TypeHead) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	if r.heads[next.Type].Revision != next.Revision-1 {
		return false, nil
	}
	next.Withdrawn = slices.Clone(next.Withdrawn)
	r.heads[next.Type] = next
	return true, nil
}

func (r *PolicyVersion) FindHead(_ context.Context, t legal.Type) (legal.TypeHead, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	h, ok := r.heads[t]
	if !ok {
		return legal.TypeHead{Type: t}, nil
	}
	h.Withdrawn = slices.Clone(h.Withdrawn)
	return h, nil
}

func (r *PolicyVersion) FindHeads(_ context.Context) ([]legal.TypeHead, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	res := make([]legal.TypeHead, 0, len(r.heads))
	for _, h := range r.heads {
		h.Withdrawn = slices.Clone(h.Withdrawn)
		res = append(res, h)
	}
	return res, nil
}

func (r *PolicyVersion) FindVersions(_ context.Context) ([]legal.VersionRecord, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	return slices.Collect(maps.Values(r.versions)), nil
}

func (r *PolicyVersion) UpsertVersion(_ context.Context, v legal.VersionRecord, now time.Time) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	if cur, ok := r.versions[v.Ref]; ok && !cur.CanOverwrite(v.Stamp, now) {
		return nil
	}
	r.versions[v.Ref] = v
	return nil
}
