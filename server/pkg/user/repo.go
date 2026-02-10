package user

import (
	"context"

	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

var ErrDuplicatedUser = rerror.NewE(i18n.T("duplicated user"))

//go:generate mockgen -source=./user.go -destination=./mock_user.go -package user
type Repo interface {
	Query
	FindByVerification(context.Context, string) (*User, error)
	FindByPasswordResetRequest(context.Context, string) (*User, error)
	FindBySubOrCreate(context.Context, *User, string) (*User, error)
	Create(context.Context, *User) error
	Save(context.Context, *User) error
	Remove(context.Context, ID) error
}

type Query interface {
	FindAll(context.Context) (List, error)
	FindByAlias(context.Context, string) (*User, error)
	FindByEmail(context.Context, string) (*User, error)
	FindByID(context.Context, ID) (*User, error)
	FindByIDs(context.Context, IDList) (List, error)
	FindByIDsWithPagination(context.Context, IDList, *string, *usecasex.Pagination) (List, *usecasex.PageInfo, error)
	FindByName(context.Context, string) (*User, error)
	FindByNameOrEmail(context.Context, string) (*User, error)
	FindBySub(context.Context, string) (*User, error)
	SearchByKeyword(context.Context, string) (List, error)
}
