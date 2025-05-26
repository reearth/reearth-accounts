package memory

import (
	"context"
	"sync"

	"github.com/reearth/reearth-accounts/pkg/id"
	"github.com/reearth/reearth-accounts/pkg/user"
	"github.com/reearth/reearthx/rerror"
)

type User struct {
	lock sync.Mutex
	data map[id.UserID]*user.User
}

func NewUser() *User {
	return &User{
		data: map[id.UserID]*user.User{},
	}
}

func NewUserWith(items ...*user.User) *User {
	u := NewUser()
	ctx := context.Background()
	for _, i := range items {
		_ = u.Save(ctx, i)
	}

	return u
}

func (u *User) FindByID(ctx context.Context, id id.UserID) (*user.User, error) {
	u.lock.Lock()
	defer u.lock.Unlock()

	res, ok := u.data[id]
	if ok {
		return res, nil
	}
	return nil, rerror.ErrNotFound
}

func (u *User) Save(ctx context.Context, usr *user.User) error {
	u.lock.Lock()
	defer u.lock.Unlock()

	u.data[usr.ID()] = usr
	return nil
}
