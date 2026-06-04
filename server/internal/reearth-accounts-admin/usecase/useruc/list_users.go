package useruc

import (
	"context"

	adminrbac "github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/rbac"
	"github.com/reearth/reearth-accounts/server/internal/reearth-accounts-admin/usecase/authz"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/i18n"
	"github.com/reearth/reearthx/rerror"
)

// ErrOperationDenied is returned when the operator lacks the required admin permission.
var ErrOperationDenied = rerror.NewE(i18n.T("operation denied"))

// ListUsersOutput is the response for listing users.
type ListUsersOutput struct {
	Items []*UserDTO `json:"items"`
} // @name ListUsersResponse

// ListUsersUseCase lists all users for the admin console.
type ListUsersUseCase struct {
	userRepo user.Repo
	authz    *authz.Checker
}

// NewListUsersUseCase is a Wire provider for ListUsersUseCase.
func NewListUsersUseCase(userRepo user.Repo, checker *authz.Checker) *ListUsersUseCase {
	return &ListUsersUseCase{userRepo: userRepo, authz: checker}
}

// Execute returns all users after verifying the operator's admin permission.
func (uc *ListUsersUseCase) Execute(ctx context.Context, operator *user.User) (*ListUsersOutput, error) {
	allowed, err := uc.authz.Allowed(ctx, operator.ID(), adminrbac.ResourceUser, adminrbac.ActionList)
	if err != nil {
		return nil, err
	}
	if !allowed {
		return nil, ErrOperationDenied
	}

	users, err := uc.userRepo.FindAll(ctx)
	if err != nil {
		return nil, err
	}

	items := make([]*UserDTO, 0, len(users))
	for _, u := range users {
		items = append(items, toUserDTO(u))
	}
	return &ListUsersOutput{Items: items}, nil
}
