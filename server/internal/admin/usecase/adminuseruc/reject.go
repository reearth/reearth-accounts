package adminuseruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// RejectAdminUserUseCase rejects a pending admin user or revokes an approved one.
type RejectAdminUserUseCase struct {
	repo adminuser.Repo
}

// NewRejectAdminUserUseCase is a Wire provider for RejectAdminUserUseCase.
func NewRejectAdminUserUseCase(repo adminuser.Repo) *RejectAdminUserUseCase {
	return &RejectAdminUserUseCase{repo: repo}
}

// Execute rejects/revokes the target admin user. An admin cannot reject their
// own account, and the last remaining approved admin cannot be rejected.
func (uc *RejectAdminUserUseCase) Execute(ctx context.Context, operatorID, targetID adminuser.ID) (*adminuser.AdminUser, error) {
	if operatorID == targetID {
		return nil, ErrCannotModifySelf
	}

	target, err := uc.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Revoking an approved admin must never drop the count of approved admins
	// to zero (otherwise nobody could ever approve again).
	if target.IsApproved() {
		approved := adminuser.StatusApproved
		_, pi, err := uc.repo.List(ctx, adminuser.ListFilter{Status: &approved})
		if err != nil {
			return nil, err
		}
		if pi != nil && pi.TotalCount <= 1 {
			return nil, ErrLastApprovedAdmin
		}
	}

	target.Reject()
	if err := uc.repo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}
