package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

var ErrDuplicatedUser = rerror.NewE(i18n.T("duplicated user"))

type User interface {
	UserQuery
	FindByVerification(context.Context, string) (*user.User, error)
	FindByPasswordResetRequest(context.Context, string) (*user.User, error)
	FindBySubOrCreate(context.Context, *user.User, string) (*user.User, error)
	Create(context.Context, *user.User) error
	Save(context.Context, *user.User) error
	Remove(context.Context, user.ID) error
}

type UserQuery interface {
	FindAll(context.Context) (user.List, error)
	FindByID(context.Context, user.ID) (*user.User, error)
	FindByIDs(context.Context, user.IDList) (user.List, error)
	FindBySub(context.Context, string) (*user.User, error)
	FindByEmail(context.Context, string) (*user.User, error)
	FindByName(context.Context, string) (*user.User, error)
	FindByNameOrEmail(context.Context, string) (*user.User, error)
	SearchByKeyword(context.Context, string) (user.List, error)
}
