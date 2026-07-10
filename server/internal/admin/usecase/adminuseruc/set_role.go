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
	// Validate the requested role before anything else so a bad input is
	// reported as ErrInvalidRole regardless of target state (otherwise the
	// demotion guard below could mask it with ErrLastSystemAdmin).
	if !role.Valid() {
		return nil, adminuser.ErrInvalidRole
	}

	target, err := uc.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Demoting an approved system_admin must never drop the count of approved
	// system_admins to zero (otherwise nobody could ever assign roles again).
	// Only approved system_admins count toward the minimum, so demoting a
	// rejected/pending system_admin is always allowed: such a target isn't part
	// of the approved set the count is taken over.
	//
	// NOTE: this is a check-then-act guard, not atomic. Two system_admins
	// demoting each other at the exact same moment could both observe two
	// system_admins and both succeed, leaving zero. Given the admin set is a tiny
	// closed group (Eukarya employees) this race is acceptable for V1; enforcing
	// it atomically would require backend-specific locking and is deferred until
	// it's actually needed.
	//
	// The repo List filter only supports Status (not Role) today, so we count
	// system_admins in-memory from the approved set, paging through it until
	// exhaustion to avoid silently truncating a large set. We only need to know
	// whether another approved system_admin exists besides the target, so we exit
	// early once a second one is found. This is acceptable for the tiny closed
	// admin set; a follow-up can use an efficient role filter once the repo
	// supports it.
	if target.IsApproved() && target.Role() == adminuser.RoleSystemAdmin && role != adminuser.RoleSystemAdmin {
		approved := adminuser.StatusApproved
		const pageSize = 100
		count := 0
		for offset := 0; ; offset += pageSize {
			list, _, err := uc.repo.List(ctx, adminuser.ListFilter{
				Status:     &approved,
				Pagination: usecasex.OffsetPagination{Offset: int64(offset), Limit: pageSize}.Wrap(),
			})
			if err != nil {
				return nil, err
			}
			for _, u := range list {
				if u.Role() == adminuser.RoleSystemAdmin {
					count++
					// count includes the target itself; >= 2 means another
					// approved system_admin exists, so the demotion is safe and
					// we can stop scanning entirely.
					if count >= 2 {
						break
					}
				}
			}
			if count >= 2 || len(list) < pageSize {
				break
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
