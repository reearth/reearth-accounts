package repo

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

var ErrDuplicatedUser = rerror.NewE(i18n.T("duplicated user"))

type User interface {
	UserQuery
	FindByVerification(context.Context, string) (*user.User, error)
	FindByPasswordResetRequest(context.Context, string) (*user.User, error)
	FindBySubOrCreate(context.Context, *user.User, string) (*user.User, error)
	Create(context.Context, *user.User) error
	Save(context.Context, *user.User) error
	Remove(context.Context, id.UserID) error
}

type UserQuery interface {
	FindAll(context.Context) ([]*user.User, error)
	FindByID(context.Context, id.UserID) (*user.User, error)
	FindByIDs(context.Context, id.UserIDList) ([]*user.User, error)
	FindByIDsWithPagination(context.Context, id.UserIDList, *usecasex.Pagination, ...string) ([]*user.User, *usecasex.PageInfo, error)
	FindBySub(context.Context, string) (*user.User, error)
	FindByEmail(context.Context, string) (*user.User, error)
	FindByName(context.Context, string) (*user.User, error)
	FindByAlias(context.Context, string) (*user.User, error)
	FindByNameOrEmail(context.Context, string) (*user.User, error)
	SearchByKeyword(context.Context, string, ...string) ([]*user.User, error)
}
