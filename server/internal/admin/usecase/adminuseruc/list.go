package adminuseruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/usecasex"
)

// ListAdminUsersUseCase lists admin users, optionally filtered by status, in
// creation order with offset pagination.
type ListAdminUsersUseCase struct {
	repo adminuser.Repo
}

// NewListAdminUsersUseCase is a Wire provider for ListAdminUsersUseCase.
func NewListAdminUsersUseCase(repo adminuser.Repo) *ListAdminUsersUseCase {
	return &ListAdminUsersUseCase{repo: repo}
}

// Execute returns the matching admin users and pagination info.
func (uc *ListAdminUsersUseCase) Execute(ctx context.Context, filter adminuser.ListFilter) (adminuser.List, *usecasex.PageInfo, error) {
	return uc.repo.List(ctx, filter)
}
