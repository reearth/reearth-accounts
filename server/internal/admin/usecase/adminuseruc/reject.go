package adminuseruc

import (
	"context"
	"errors"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/reearth/reearthx/usecasex"
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
	//
	// NOTE: this is a check-then-act guard, not atomic. Two approved admins
	// rejecting each other at the exact same moment could both observe
	// TotalCount == 2 and both succeed, leaving zero approved admins. Given the
	// admin set is a tiny closed group (Eukarya employees) this race is
	// acceptable for V1; enforcing it atomically would require backend-specific
	// locking (e.g. a Postgres advisory lock / serializable transaction or a
	// conditional update) and is deferred until it's actually needed.
	if target.IsApproved() {
		approved := adminuser.StatusApproved
		// Only the total count is needed; limit to a single row so repos don't
		// fetch the entire approved-admin list on every revoke.
		_, pi, err := uc.repo.List(ctx, adminuser.ListFilter{
			Status:     &approved,
			Pagination: usecasex.OffsetPagination{Offset: 0, Limit: 1}.Wrap(),
		})
		if err != nil {
			return nil, err
		}
		// PageInfo is part of the Repo.List contract (the ListAdminUsers handler
		// dereferences it unconditionally too). Fail fast on a nil rather than
		// silently skipping the guard, which would let the last approved admin be
		// rejected.
		if pi == nil {
			return nil, rerror.ErrInternalBy(errors.New("admin user list returned nil page info"))
		}
		if pi.TotalCount <= 1 {
			return nil, ErrLastApprovedAdmin
		}
	}

	target.Reject()
	if err := uc.repo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}
