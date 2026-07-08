package useruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/user"
)

// GetUserUseCase fetches a single user by ID for the admin console.
type GetUserUseCase struct {
	userRepo user.Repo
}

// NewGetUserUseCase is a Wire provider for GetUserUseCase.
func NewGetUserUseCase(userRepo user.Repo) *GetUserUseCase {
	return &GetUserUseCase{userRepo: userRepo}
}

// Execute returns the user with the given ID, or rerror.ErrNotFound if absent.
func (uc *GetUserUseCase) Execute(ctx context.Context, id user.ID) (*user.User, error) {
	return uc.userRepo.FindByID(ctx, id)
}
