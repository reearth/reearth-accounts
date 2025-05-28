package memory

import (
	"context"
	"sync"
	"strings"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/util"
)

type User struct {
	data *util.SyncMap[user.ID, *user.User]
	err  error
}

func NewUser() *User {
	return &User{
		data: &util.SyncMap[user.ID, *user.User]{},
	}
}

func NewUserWith(users ...*user.User) *User {
	r := NewUser()
	for _, u := range users {
		r.data.Store(u.ID(), u)
	}
	return r
}

func (r *User) FindAll(ctx context.Context) (user.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	res := r.data.FindAll(func(key user.ID, value *user.User) bool {
		return true
	})

	return res, nil
}

func (r *User) FindByIDs(_ context.Context, ids user.IDList) (user.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	res := r.data.FindAll(func(key user.ID, value *user.User) bool {
		return ids.Has(key)
	})

	return res, nil
}

func (r *User) FindByID(_ context.Context, v user.ID) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return key == v
	}), rerror.ErrNotFound)
}

func (r *User) FindBySub(_ context.Context, auth0sub string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if auth0sub == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.ContainAuth(user.AuthFrom(auth0sub))
	}), rerror.ErrNotFound)
}

func (r *User) FindByPasswordResetRequest(_ context.Context, token string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if token == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.PasswordReset() != nil && value.PasswordReset().Token == token
	}), rerror.ErrNotFound)
}

func (r *User) FindByEmail(_ context.Context, email string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if email == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.Email() == email
	}), rerror.ErrNotFound)
}

func (r *User) FindByName(_ context.Context, name string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if name == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.Name() == name
	}), rerror.ErrNotFound)
}

func (r *User) FindByNameOrEmail(_ context.Context, nameOrEmail string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if nameOrEmail == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.Email() == nameOrEmail || value.Name() == nameOrEmail
	}), rerror.ErrNotFound)
}

func (r *User) SearchByKeyword(_ context.Context, keyword string) (user.List, error) {
	if r.err != nil {
		return nil, r.err
	}

	if len(keyword) < 3 {
		return nil, nil
	}

	keyword = strings.TrimSpace(strings.ToLower(keyword))

	return rerror.ErrIfNil(r.data.FindAll(func(key user.ID, value *user.User) bool {
		return strings.Contains(strings.ToLower(value.Email()), keyword) ||
			strings.Contains(strings.ToLower(value.Name()), keyword)
	}), rerror.ErrNotFound)
}

func (r *User) FindByVerification(_ context.Context, code string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	if code == "" {
		return nil, rerror.ErrInvalidParams
	}

	return rerror.ErrIfNil(r.data.Find(func(key user.ID, value *user.User) bool {
		return value.Verification() != nil && value.Verification().Code() == code

	}), rerror.ErrNotFound)
}

func (r *User) FindBySubOrCreate(_ context.Context, u *user.User, sub string) (*user.User, error) {
	if r.err != nil {
		return nil, r.err
	}

	u2 := r.data.Find(func(key user.ID, value *user.User) bool {
		return value.ContainAuth(user.AuthFrom(sub))
	})
	if u2 == nil {
		r.data.Store(u.ID(), u)
		return u, nil
	}
	return u2, nil
}

func (r *User) Create(_ context.Context, u *user.User) error {
	if r.err != nil {
		return r.err
	}

	if _, ok := r.data.Load(u.ID()); !ok {
		r.data.Store(u.ID(), u)
	} else {
		return repo.ErrDuplicatedUser
	}

	return nil
}

func (r *User) Save(_ context.Context, u *user.User) error {
	if r.err != nil {
		return r.err
	}

	r.data.Store(u.ID(), u)
	return nil
}

func (r *User) Remove(_ context.Context, user user.ID) error {
	if r.err != nil {
		return r.err
	}

	r.data.Delete(user)
	return nil
}

func SetUserError(r repo.User, err error) {
	r.(*User).err = err
}
