package useruc

import (
	"context"

	adminrbac "github.com/reearth/reearth-accounts/server/internal/admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/pkg/pagination"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

// ErrOperationDenied is returned when the operator lacks the required admin permission.
var ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))

// ErrInvalidOperator is returned when no authenticated operator is supplied.
// In practice the auth middleware always provides one; this is a defensive
// guard against a nil dereference from future call sites.
var ErrInvalidOperator = rerror.NewE(i18n.T("invalid operator"))

// ListUsersInput carries pagination parameters for the list-users request.
type ListUsersInput struct {
	Page     int64
	PageSize int64
} // @name ListUsersRequest

// ListUsersOutput is the response for listing users.
type ListUsersOutput struct {
	Items    []*UserItem `json:"items"`
	Total    int64       `json:"total"`
	Page     int64       `json:"page"`
	PageSize int64       `json:"page_size"`
} // @name ListUsersResponse

// UserItem represents a single user in the admin API response.
type UserItem struct {
	ID    string `json:"id"`
	Name  string `json:"name"`
	Email string `json:"email"`
	Alias string `json:"alias"`
} // @name User

func toUserItem(u *user.User) *UserItem {
	if u == nil {
		return nil
	}
	return &UserItem{
		ID:    u.ID().String(),
		Name:  u.Name(),
		Email: u.Email(),
		Alias: u.Alias(),
	}
}

// ListUsersUseCase lists all users for the admin console.
type ListUsersUseCase struct {
	userRepo user.Repo
	authz    *authz.Checker
}

// NewListUsersUseCase is a Wire provider for ListUsersUseCase.
func NewListUsersUseCase(userRepo user.Repo, checker *authz.Checker) *ListUsersUseCase {
	return &ListUsersUseCase{userRepo: userRepo, authz: checker}
}

// Execute returns a paginated list of users after verifying the operator's admin permission.
func (uc *ListUsersUseCase) Execute(ctx context.Context, operator *user.User, input ListUsersInput) (*ListUsersOutput, error) {
	if operator == nil {
		return nil, ErrInvalidOperator
	}

	allowed, err := uc.authz.Allowed(ctx, operator.ID(), adminrbac.ResourceUser, adminrbac.ActionList)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrOperationDenied
	}

	p := pagination.ToPagination(input.Page, input.PageSize)
	users, pageInfo, err := uc.userRepo.FindAllWithPagination(ctx, nil, p)
	if err != nil {
		return nil, err
	}

	items := make([]*UserItem, 0, len(users))
	for _, u := range users {
		items = append(items, toUserItem(u))
	}

	page := input.Page
	if page < 1 {
		page = 1
	}
	pageSize := input.PageSize
	if pageSize < 1 {
		pageSize = 50
	}
	if pageSize > 100 {
		pageSize = 100
	}

	var total int64
	if pageInfo != nil {
		total = pageInfo.TotalCount
	}

	return &ListUsersOutput{
		Items:    items,
		Total:    total,
		Page:     page,
		PageSize: pageSize,
	}, nil
}
