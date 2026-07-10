package adminuseruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// ApproveAdminUserUseCase approves a (typically pending) admin user.
type ApproveAdminUserUseCase struct {
	repo adminuser.Repo
}

// NewApproveAdminUserUseCase is a Wire provider for ApproveAdminUserUseCase.
func NewApproveAdminUserUseCase(repo adminuser.Repo) *ApproveAdminUserUseCase {
	return &ApproveAdminUserUseCase{repo: repo}
}

// Execute approves the target admin user, recording the operator as approver.
// An admin cannot approve their own account.
func (uc *ApproveAdminUserUseCase) Execute(ctx context.Context, operatorID, targetID adminuser.ID) (*adminuser.AdminUser, error) {
	if operatorID == targetID {
		return nil, ErrCannotModifySelf
	}

	target, err := uc.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Approve is idempotent: for an already-approved user AdminUser.Approve is a
	// no-op (it preserves the original approver/timestamp), so skip the redundant
	// write and return the record as-is.
	//
	// NOTE: this is a read-modify-write, not atomic. Two admins approving the
	// same pending user at the exact same moment could both read it as pending
	// and both write, so the last write wins and records that approver. This is
	// benign — the recorded approver is still one of the two legitimate admins
	// and no invariant is violated (unlike reject, this can't leave zero approved
	// admins) — so atomic enforcement is not worth the cost here.
	if target.IsApproved() {
		return target, nil
	}

	target.Approve(operatorID)
	// A freshly approved admin defaults to viewer (least-privilege). A record
	// that already carries a role — e.g. a previously-assigned admin who was
	// rejected and is now being re-approved — keeps it; we never clobber or
	// downgrade an existing role here.
	if target.Role() == "" {
		if err := target.SetRole(adminuser.RoleViewer); err != nil {
			return nil, err
		}
	}
	if err := uc.repo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}
