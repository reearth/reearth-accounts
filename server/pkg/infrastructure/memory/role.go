package memory

import (
	"context"
	"sync"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearthx/rerror"
)

type Role struct {
	lock sync.Mutex
	data map[id.RoleID]*role.Role
}

func NewRole() *Role {
	return &Role{
		data: map[id.RoleID]*role.Role{},
	}
}

func NewRoleWith(items ...*role.Role) *Role {
	r := NewRole()
	ctx := context.Background()
	for _, i := range items {
		_ = r.Save(ctx, *i)
	}
	return r
}

func (r *Role) FindAll(ctx context.Context) (role.List, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	res := make(role.List, 0, len(r.data))
	for _, v := range r.data {
		res = append(res, v)
	}
	return res, nil
}

func (r *Role) FindByID(ctx context.Context, id id.RoleID) (*role.Role, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	res, ok := r.data[id]
	if ok {
		return res, nil
	}
	return nil, rerror.ErrNotFound
}

func (r *Role) FindByIDs(ctx context.Context, ids id.RoleIDList) (role.List, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	res := make(role.List, 0, len(ids))
	for _, id := range ids {
		if v, ok := r.data[id]; ok {
			res = append(res, v)
		}
	}
	return res, nil
}

func (r *Role) Save(ctx context.Context, rl role.Role) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	r.data[rl.ID()] = &rl
	return nil
}

func (r *Role) Remove(ctx context.Context, id id.RoleID) error {
	r.lock.Lock()
	defer r.lock.Unlock()

	delete(r.data, id)
	return nil
}
