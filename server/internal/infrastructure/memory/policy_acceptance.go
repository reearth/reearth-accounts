package memory

import (
	"context"
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
