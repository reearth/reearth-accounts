package repo

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type MultiUser []User

func NewMultiUser(users ...User) MultiUser {
	return MultiUser(users)
}

var _ User = MultiUser{}

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
	return u.findOne(func(r User) (*user.User, error) {
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

func (u MultiUser) FindBySub(ctx context.Context, sub string) (*user.User, error) {
	return u.findOne(func(r User) (*user.User, error) {
		return r.FindBySub(ctx, sub)
	})
}

func (u MultiUser) FindByEmail(ctx context.Context, email string) (*user.User, error) {
	return u.findOne(func(r User) (*user.User, error) {
		return r.FindByEmail(ctx, email)
	})
}

func (u MultiUser) FindByName(ctx context.Context, name string) (*user.User, error) {
	return u.findOne(func(r User) (*user.User, error) {
		return r.FindByName(ctx, name)
	})
}

func (u MultiUser) FindByNameOrEmail(ctx context.Context, nameOrEmail string) (*user.User, error) {
	return u.findOne(func(r User) (*user.User, error) {
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
	return u.first2(func(r User) (*user.User, error) {
		return r.FindByVerification(ctx, v)
	})
}

func (u MultiUser) FindByPasswordResetRequest(ctx context.Context, p string) (*user.User, error) {
	return u.first2(func(r User) (*user.User, error) {
		return r.FindByPasswordResetRequest(ctx, p)
	})
}

func (u MultiUser) FindBySubOrCreate(ctx context.Context, v *user.User, s string) (*user.User, error) {
	return u.first2(func(r User) (*user.User, error) {
		return r.FindBySubOrCreate(ctx, v, s)
	})
}

func (u MultiUser) Create(ctx context.Context, user *user.User) error {
	return u.first(func(r User) error {
		return r.Create(ctx, user)
	})
}

func (u MultiUser) Save(ctx context.Context, user *user.User) error {
	return u.first(func(r User) error {
		return r.Save(ctx, user)
	})
}

func (u MultiUser) Remove(ctx context.Context, id user.ID) error {
	return u.first(func(r User) error {
		return r.Remove(ctx, id)
	})
}

func (u MultiUser) findOne(f func(User) (*user.User, error)) (*user.User, error) {
	for _, r := range u {
		if res, err := f(r); err != nil && !errors.Is(err, rerror.ErrNotFound) {
			return nil, err
		} else if res != nil {
			return res, nil
		}
	}
	return nil, nil
}

func (u MultiUser) first(f func(User) error) error {
	if len(u) == 0 {
		return errors.New("no repo")
	}
	return f(u[0])
}

func (u MultiUser) first2(f func(User) (*user.User, error)) (*user.User, error) {
	if len(u) == 0 {
		return nil, errors.New("no repo")
	}
	return f(u[0])
}
