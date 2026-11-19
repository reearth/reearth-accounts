package memory

import (
	"context"
	"slices"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type PolicyNotice struct {
	data map[id.PolicyNoticeID]legal.Notice
	lock sync.Mutex
}

func NewPolicyNotice() *PolicyNotice {
	return &PolicyNotice{data: map[id.PolicyNoticeID]legal.Notice{}}
}

func (r *PolicyNotice) AdvanceFanout(_ context.Context, n id.PolicyNoticeID, lease legal.Lease, cursor string, done bool) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	stored, ok := r.data[n]
	if !ok || stored.Fanout.Lease == nil || stored.Fanout.Lease.Token != lease.Token {
		return false, nil
	}
	stored.Fanout = legal.Fanout{Cursor: cursor, Done: done}
	if !done {
		stored.Fanout.Lease = &lease
	}
	r.data[n] = stored
	return true, nil
}

func (r *PolicyNotice) ClaimFanout(_ context.Context, now time.Time, lease legal.Lease) (*legal.Notice, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var next *legal.Notice
	for _, n := range r.data {
		if n.Fanout.Done || (n.Fanout.Lease != nil && n.Fanout.Lease.Until.After(now)) {
			continue
		}
		if next == nil || n.CreatedAt.Before(next.CreatedAt) {
			next = &n
		}
	}
	if next == nil {
		return nil, nil
	}
	next.Fanout.Lease = &lease
	r.data[next.ID] = *next
	return next, nil
}

func (r *PolicyNotice) Create(_ context.Context, n legal.Notice) (legal.Notice, bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, stored := range r.data {
		if stored.Ref == n.Ref && stored.Audience == n.Audience {
			return stored, false, nil
		}
	}
	r.data[n.ID] = n
	return n, true, nil
}

func (r *PolicyNotice) FindByRef(_ context.Context, ref legal.Ref) ([]legal.Notice, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	res := []legal.Notice{}
	for _, n := range r.data {
		if n.Ref == ref {
			res = append(res, n)
		}
	}
	slices.SortFunc(res, func(a, b legal.Notice) int { return a.CreatedAt.Compare(b.CreatedAt) })
	return res, nil
}

func (r *PolicyNotice) ReleaseFanout(_ context.Context, n id.PolicyNoticeID, token string) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	stored, ok := r.data[n]
	if ok && stored.Fanout.Lease != nil && stored.Fanout.Lease.Token == token {
		stored.Fanout.Lease = nil
		r.data[n] = stored
	}
	return nil
}
