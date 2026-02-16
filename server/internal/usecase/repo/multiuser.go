package repo

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

type MultiUser []user.Repo

func NewMultiUser(users ...user.Repo) MultiUser {
	return MultiUser(users)
}

var _ user.Repo = MultiUser{}

func (u MultiUser) FindAll(ctx context.Context) (user.List, error) {
	res := user.List{}
	for _, r := range u {
		if r, err := r.FindAll(ctx); err != nil {
			return nil, err
		} else {
			res = append(res, r...)
		}
	}
	return res, nil
}

func (u MultiUser) FindByID(ctx context.Context, id user.ID) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindByID(ctx, id)
	})
}

func (u MultiUser) FindByIDs(ctx context.Context, ids user.IDList) (user.List, error) {
	res := user.List{}
	for _, r := range u {
		if r, err := r.FindByIDs(ctx, ids); err != nil {
			return nil, err
		} else {
			res = append(res, r...)
		}
	}
	return res, nil
}

func (u MultiUser) FindByIDsWithPagination(ctx context.Context, ids user.IDList, alias *string, pagination *usecasex.Pagination) (user.List, *usecasex.PageInfo, error) {
	res := user.List{}
	var pi *usecasex.PageInfo
	for _, r := range u {
		if users, pageInfo, err := r.FindByIDsWithPagination(ctx, ids, alias, pagination); err != nil {
			if !errors.Is(err, rerror.ErrNotFound) {
				return nil, nil, err
			}
		} else {
			res = append(res, users...)
			pi = pageInfo
		}
	}
	return res, pi, nil
}

func (u MultiUser) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindBySub(ctx, sub)
	})
}

func (u MultiUser) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindByEmail(ctx, email)
	})
}

func (u MultiUser) FindByName(ctx context.Context, name string) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindByName(ctx, name)
	})
}

func (u MultiUser) FindByAlias(ctx context.Context, alias string) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindByAlias(ctx, alias)
	})
}

func (u MultiUser) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return u.findOne(func(r user.Repo) (*user.User, error) {
		return r.FindByNameOrEmail(ctx, nameOrEmail)
	})
}

func (u MultiUser) SearchByKeyword(ctx context.Context, keyword string) (user.List, error) {
	res := user.List{}
	for _, r := range u {
		if r, err := r.SearchByKeyword(ctx, keyword); err != nil {
			return nil, err
		} else {
			res = append(res, r...)
		}
	}
	return res, nil
}

func (u MultiUser) FindByVerification(ctx context.Context, v string) (*user.User, error) {
	return u.first2(func(r user.Repo) (*user.User, error) {
		return r.FindByVerification(ctx, v)
	})
}

func (u MultiUser) FindByPasswordResetRequest(ctx context.Context, p string) (*user.User, error) {
	return u.first2(func(r user.Repo) (*user.User, error) {
		return r.FindByPasswordResetRequest(ctx, p)
	})
}

func (u MultiUser) FindBySubOrCreate(ctx context.Context, v *user.User, s string) (*user.User, error) {
	return u.first2(func(r user.Repo) (*user.User, error) {
		return r.FindBySubOrCreate(ctx, v, s)
	})
}

func (u MultiUser) Create(ctx context.Context, usr *user.User) error {
	return u.first(func(r user.Repo) error {
		return r.Create(ctx, usr)
	})
}

func (u MultiUser) Save(ctx context.Context, usr *user.User) error {
	return u.first(func(r user.Repo) error {
		return r.Save(ctx, usr)
	})
}

func (u MultiUser) Remove(ctx context.Context, id user.ID) error {
	return u.first(func(r user.Repo) error {
		return r.Remove(ctx, id)
	})
}

func (u MultiUser) findOne(f func(user.Repo) (*user.User, error)) (*user.User, error) {
	for _, r := range u {
		if res, err := f(r); err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		} else if res != nil {
			return res, nil
		}
	}
	return nil, nil
}

func (u MultiUser) first(f func(user.Repo) error) error {
	if len(u) == 0 {
		return errors.New("no repo")
	}
	return f(u[0])
}

func (u MultiUser) first2(f func(user.Repo) (*user.User, error)) (*user.User, error) {
	if len(u) == 0 {
		return nil, errors.New("no repo")
	}
	return f(u[0])
}
