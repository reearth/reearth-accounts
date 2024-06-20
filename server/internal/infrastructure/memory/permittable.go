package memory

import (
	"context"
	"sync"

	"golang.org/x/exp/slices"

	"github.com/reearth/reearth-account/pkg/id"
	"github.com/reearth/reearth-account/pkg/permittable"
	"github.com/reearth/reearthx/account/accountdomain/user"
	"github.com/reearth/reearthx/rerror"
)

type Permittable struct {
	lock sync.Mutex
	data map[id.PermittableID]*permittable.Permittable
}

func NewPermittable() *Permittable {
	return &Permittable{
		data: map[id.PermittableID]*permittable.Permittable{},
	}
}

func NewPermittableWith(items ...*permittable.Permittable) *Permittable {
	p := NewPermittable()
	ctx := context.Background()
	for _, i := range items {
		_ = p.Save(ctx, *i)
	}
	return p
}

func (p *Permittable) FindByUserID(ctx context.Context, userID user.ID) (*permittable.Permittable, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	for _, perm := range p.data {
		if perm.UserID() == userID {
			return perm, nil
		}
	}
	return nil, rerror.ErrNotFound
}

func (p *Permittable) FindByUserIDs(ctx context.Context, userIDs user.IDList) (permittable.List, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	results := make(permittable.List, 0, len(userIDs))
	for _, userID := range userIDs {
		for _, perm := range p.data {
			if perm.UserID() == userID {
				results = append(results, perm)
				break
			}
		}
	}

	if len(results) == 0 {
		return nil, rerror.ErrNotFound
	}

	return results, nil
}

func (p *Permittable) FindByRoleID(ctx context.Context, roleID id.RoleID) (permittable.List, error) {
	p.lock.Lock()
	defer p.lock.Unlock()

	results := make(permittable.List, 0, len(p.data))
	for _, perm := range p.data {
		if slices.Contains(perm.RoleIDs(), roleID) {
			results = append(results, perm)
		}
	}

	if len(results) == 0 {
		return nil, rerror.ErrNotFound
	}

	return results, nil
}

func (r *Permittable) Save(ctx context.Context, p permittable.Permittable) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.data[p.ID()] = &p
	return nil
}
