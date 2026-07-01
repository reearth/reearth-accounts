package adminuser

import (
	"context"

	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

var ErrDuplicatedAdminUser = rerror.NewE(i18n.T("duplicated admin user"))

// ListFilter narrows and paginates a List query.
type ListFilter struct {
	Status     *Status
	Pagination *usecasex.Pagination
}

//go:generate mockgen -source=./repo.go -destination=./mock_adminuser.go -package adminuser
type Repo interface {
	FindByEmail(context.Context, string) (*AdminUser, error)
	FindByID(context.Context, ID) (*AdminUser, error)
	FindByIDs(context.Context, IDList) (List, error)
	List(context.Context, ListFilter) (List, *usecasex.PageInfo, error)
	Save(context.Context, *AdminUser) error
}
