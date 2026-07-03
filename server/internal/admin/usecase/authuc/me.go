package authuc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// GetMeUseCase loads a single admin user by ID (the current session's user).
type GetMeUseCase struct {
	repo adminuser.Repo
}

// NewGetMeUseCase is a Wire provider for GetMeUseCase.
func NewGetMeUseCase(repo adminuser.Repo) *GetMeUseCase {
	return &GetMeUseCase{repo: repo}
}

// Execute returns the admin user for the given ID.
func (uc *GetMeUseCase) Execute(ctx context.Context, id adminuser.ID) (*adminuser.AdminUser, error) {
	return uc.repo.FindByID(ctx, id)
}
