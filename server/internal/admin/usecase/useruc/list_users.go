// Package useruc holds the admin user usecases. V1 is read-only (list). The
// "must be approved" gate is applied by the RequireApproved middleware, so these
// usecases carry no extra authorization.
package useruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
)

// ListUsersUseCase lists users for the admin console, optionally filtered by a
// name/alias/email keyword, with offset pagination.
type ListUsersUseCase struct {
	userRepo user.Repo
}

// NewListUsersUseCase is a Wire provider for ListUsersUseCase.
func NewListUsersUseCase(userRepo user.Repo) *ListUsersUseCase {
	return &ListUsersUseCase{userRepo: userRepo}
}

// Execute returns the matching users and pagination info.
func (uc *ListUsersUseCase) Execute(ctx context.Context, keyword *string, pagination *usecasex.Pagination) (user.List, *usecasex.PageInfo, error) {
	return uc.userRepo.FindAllWithPagination(ctx, keyword, pagination)
}
