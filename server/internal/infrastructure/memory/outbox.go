package memory

import (
	"context"
	"maps"
	"sync"
	"time"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/outbox"
	"github.com/reearth/reearthx/rerror"
)

type Outbox struct {
	data map[outbox.Key]outbox.Record
	lock sync.Mutex
}

func NewOutbox() *Outbox { return &Outbox{data: map[outbox.Key]outbox.Record{}} }

func (r *Outbox) ClaimDue(_ context.Context, now time.Time, c outbox.Claim) (*outbox.Record, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var next *outbox.Record
	for _, rec := range r.data {
		if rec.Due(now) && (next == nil || rec.NextAttemptAt.Before(next.NextAttemptAt)) {
			next = &rec
		}
	}
	if next == nil {
		return nil, nil
	}
	claimed := next.Claimed(c, now)
	r.data[claimed.Key] = claimed
	return clone(claimed), nil
}

func (r *Outbox) Complete(_ context.Context, k outbox.Key, token string, t outbox.Transition, now time.Time) (bool, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	rec, ok := r.data[k]
	if !ok || !rec.Holds(token, now) {
		return false, nil
	}
	r.data[k] = rec.Applied(t, now)
	return true, nil
}

func (r *Outbox) Enqueue(_ context.Context, rs []outbox.Record) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, rec := range rs {
		if _, ok := r.data[rec.Key]; !ok {
			r.data[rec.Key] = *clone(rec)
		}
	}
	return nil
}

func (r *Outbox) FindByKey(_ context.Context, k outbox.Key) (*outbox.Record, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	rec, ok := r.data[k]
	if !ok {
		return nil, rerror.ErrNotFound
	}
	return clone(rec), nil
}

func (r *Outbox) RemoveByUser(_ context.Context, u id.UserID) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for k, rec := range r.data {
		if uid, ok := rec.Recipient.User(); ok && uid == u {
			delete(r.data, k)
		}
	}
	return nil
}

func (r *Outbox) Sweep(_ context.Context, now time.Time, ttl time.Duration) (outbox.SweepResult, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	var res outbox.SweepResult
	for k, rec := range r.data {
		if rec.ClaimExpired(now) {
			r.data[k] = rec.Expired(now)
			res.Expired++
		}
	}
	for k, rec := range r.data {
		if rec.Stale(now, ttl) {
			r.data[k] = rec.TimedOut(now)
			res.TimedOut++
		}
	}
	for k, rec := range r.data {
		if rec.Retired(now, ttl) {
			delete(r.data, k)
			res.Deleted++
		}
	}
	return res, nil
}

func clone(rec outbox.Record) *outbox.Record {
	rec.Subject = maps.Clone(rec.Subject)
	if rec.Claim != nil {
		c := *rec.Claim
		rec.Claim = &c
	}
	if rec.EndedAt != nil {
		e := *rec.EndedAt
		rec.EndedAt = &e
	}
	return &rec
}

var _ outbox.Repo = (*Outbox)(nil)
