package adminuseruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/usecasex"
)

// SetRoleUseCase assigns a role to an admin user.
type SetRoleUseCase struct {
	repo adminuser.Repo
}

// NewSetRoleUseCase is a Wire provider for SetRoleUseCase.
func NewSetRoleUseCase(repo adminuser.Repo) *SetRoleUseCase {
	return &SetRoleUseCase{repo: repo}
}

// Execute assigns role to the target admin user. Unlike approve/reject, changing
// one's own role is allowed; the zero-system_admin guard below still prevents the
// last system_admin from demoting themselves.
func (uc *SetRoleUseCase) Execute(ctx context.Context, operatorID, targetID adminuser.ID, role adminuser.Role) (*adminuser.AdminUser, error) {
	target, err := uc.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Demoting a system_admin must never drop the count of system_admins to zero
	// (otherwise nobody could ever assign roles again).
	//
	// NOTE: this is a check-then-act guard, not atomic. Two system_admins
	// demoting each other at the exact same moment could both observe two
	// system_admins and both succeed, leaving zero. Given the admin set is a tiny
	// closed group (Eukarya employees) this race is acceptable for V1; enforcing
	// it atomically would require backend-specific locking and is deferred until
	// it's actually needed.
	//
	// The repo List filter only supports Status (not Role) today, so we count
	// system_admins in-memory from the approved set. This is acceptable for the
	// tiny closed admin set; a follow-up can use an efficient role filter once the
	// repo supports it.
	if target.Role() == adminuser.RoleSystemAdmin && role != adminuser.RoleSystemAdmin {
		approved := adminuser.StatusApproved
		list, _, err := uc.repo.List(ctx, adminuser.ListFilter{
			Status:     &approved,
			Pagination: usecasex.OffsetPagination{Offset: 0, Limit: 1000}.Wrap(),
		})
		if err != nil {
			return nil, err
		}
		count := 0
		for _, u := range list {
			if u.Role() == adminuser.RoleSystemAdmin {
				count++
			}
		}
		// count includes the target itself; <= 1 means the target is the only
		// approved system_admin.
		if count <= 1 {
			return nil, ErrLastSystemAdmin
		}
	}

	if err := target.SetRole(role); err != nil {
		return nil, err
	}
	if err := uc.repo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}
