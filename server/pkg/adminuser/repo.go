package adminuser

import (
	"context"

	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
)

var (
	ErrDuplicatedAdminUser = rerror.NewE(i18n.T("duplicated admin user"))
	// ErrCursorPaginationUnsupported is returned by List when cursor-based
	// pagination is requested. Admin user listing is offset-based only
	// (page / per_page), consistently across all backends.
	ErrCursorPaginationUnsupported = rerror.NewE(i18n.T("cursor pagination is not supported for admin users"))
)

// ListFilter narrows and paginates a List query.
type ListFilter struct {
	Status     *Status
	Role       *Role
	Pagination *usecasex.Pagination
}

//go:generate mockgen -source=./repo.go -destination=./mock_adminuser.go -package adminuser
type Repo interface {
	FindByEmail(context.Context, string) (*AdminUser, error)
	FindByID(context.Context, ID) (*AdminUser, error)
	FindByIDs(context.Context, IDList) (List, error)
	List(context.Context, ListFilter) (List, *usecasex.PageInfo, error)
	// ExistsApprovedSystemAdminExcept reports whether an approved system_admin
	// other than excludeID exists. Used to guard against demoting the last one.
	ExistsApprovedSystemAdminExcept(ctx context.Context, excludeID ID) (bool, error)
	Save(context.Context, *AdminUser) error
}
