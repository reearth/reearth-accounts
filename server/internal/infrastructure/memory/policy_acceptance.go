package memory

import (
	"context"
	"maps"
	"slices"
	"strings"
	"sync"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/legal"
)

type acceptanceKey struct {
	legal.Ref
	user id.UserID
}

type PolicyAcceptance struct {
	data map[acceptanceKey]legal.Acceptance
	lock sync.Mutex
}

func NewPolicyAcceptance() *PolicyAcceptance {
	return &PolicyAcceptance{data: map[acceptanceKey]legal.Acceptance{}}
}

func (r *PolicyAcceptance) FindByUser(_ context.Context, u id.UserID) ([]legal.Acceptance, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	res := []legal.Acceptance{}
	for k, a := range r.data {
		if k.user == u {
			res = append(res, a)
		}
	}
	return res, nil
}

func (r *PolicyAcceptance) FindUsersAfter(_ context.Context, after *id.UserID, limit int) ([]id.UserID, error) {
	r.lock.Lock()
	defer r.lock.Unlock()
	seen := map[id.UserID]struct{}{}
	for k := range r.data {
		if after == nil || k.user.String() > after.String() {
			seen[k.user] = struct{}{}
		}
	}
	users := slices.SortedFunc(maps.Keys(seen), func(a, b id.UserID) int { return strings.Compare(a.String(), b.String()) })
	return users[:min(limit, len(users))], nil
}

func (r *PolicyAcceptance) InsertIfAbsent(_ context.Context, as []legal.Acceptance) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, a := range as {
		k := acceptanceKey{Ref: a.Ref, user: a.UserID}
		if _, ok := r.data[k]; !ok {
			r.data[k] = a
		}
	}
	return nil
}

func (r *PolicyAcceptance) Remove(_ context.Context, as []legal.Acceptance) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for _, a := range as {
		k := acceptanceKey{Ref: a.Ref, user: a.UserID}
		if stored, ok := r.data[k]; ok && stored.AcceptedAt.Equal(a.AcceptedAt) {
			delete(r.data, k)
		}
	}
	return nil
}

func (r *PolicyAcceptance) RemoveByUser(_ context.Context, u id.UserID) error {
	r.lock.Lock()
	defer r.lock.Unlock()
	for k := range r.data {
		if k.user == u {
			delete(r.data, k)
		}
	}
	return nil
}
