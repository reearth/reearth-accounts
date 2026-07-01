package memory

import (
	"context"
	"sort"
	"sync"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type AdminUser struct {
	lock sync.Mutex
	data map[adminuser.ID]*adminuser.AdminUser
}

func NewAdminUser() adminuser.Repo {
	return &AdminUser{
		data: map[adminuser.ID]*adminuser.AdminUser{},
	}
}

func NewAdminUserWith(items ...*adminuser.AdminUser) adminuser.Repo {
	r := &AdminUser{data: map[adminuser.ID]*adminuser.AdminUser{}}
	ctx := context.Background()
	for _, i := range items {
		_ = r.Save(ctx, i)
	}
	return r
}

func (r *AdminUser) FindByEmail(ctx context.Context, email string) (*adminuser.AdminUser, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	e := adminuser.NormalizeEmail(email)
	for _, v := range r.data {
		if v.Email() == e {
			return v, nil
		}
	}
	return nil, rerror.ErrNotFound
}

func (r *AdminUser) FindByID(ctx context.Context, id adminuser.ID) (*adminuser.AdminUser, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	if v, ok := r.data[id]; ok {
		return v, nil
	}
	return nil, rerror.ErrNotFound
}

func (r *AdminUser) FindByIDs(ctx context.Context, ids adminuser.IDList) (adminuser.List, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	res := make(adminuser.List, 0, len(ids))
	for _, id := range ids {
		if v, ok := r.data[id]; ok {
			res = append(res, v)
		}
	}
	return res, nil
}

func (r *AdminUser) List(ctx context.Context, f adminuser.ListFilter) (adminuser.List, *usecasex.PageInfo, error) {
	r.lock.Lock()
	defer r.lock.Unlock()

	all := make(adminuser.List, 0, len(r.data))
	for _, v := range r.data {
		if f.Status != nil && v.Status() != *f.Status {
			continue
		}
		all = append(all, v)
	}

	// sort by creation time (ascending), then by ID for stable ordering
	sort.SliceStable(all, func(i, j int) bool {
		ti, tj := all[i].CreatedAt(), all[j].CreatedAt()
		if ti.Equal(tj) {
			return all[i].ID().Compare(all[j].ID()) < 0
		}
		return ti.Before(tj)
	})

	total := int64(len(all))

	if f.Pagination == nil || f.Pagination.Offset == nil {
		return all, usecasex.NewPageInfo(total, nil, nil, false, false), nil
	}

	offset := f.Pagination.Offset.Offset
	limit := f.Pagination.Offset.Limit
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	page := all[offset:end]
	hasNext := end < total
	hasPrev := offset > 0
	return page, usecasex.NewPageInfo(total, nil, nil, hasNext, hasPrev), nil
}

func (r *AdminUser) Save(ctx context.Context, u *adminuser.AdminUser) error {
	if u == nil {
		return nil
	}
	r.lock.Lock()
	defer r.lock.Unlock()

	r.data[u.ID()] = u
	return nil
}
