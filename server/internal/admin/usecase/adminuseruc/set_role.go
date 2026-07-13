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

// Execute assigns role to the target admin user. Self-role changes are allowed
// (RBAC is enforced in the middleware), but the last approved system_admin
// cannot be demoted.
func (uc *SetRoleUseCase) Execute(ctx context.Context, _, targetID adminuser.ID, role adminuser.Role) (*adminuser.AdminUser, error) {
	// Validate before loading the target so a bad input maps to ErrInvalidRole.
	if !role.Valid() {
		return nil, adminuser.ErrInvalidRole
	}

	target, err := uc.repo.FindByID(ctx, targetID)
	if err != nil {
		return nil, err
	}

	// Demoting an approved system_admin is blocked if it is the last one. Count
	// approved system_admins by paging the approved set until a second one is
	// found (check-then-act, not atomic; acceptable for the tiny admin set).
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
					// count includes the target; >= 2 means the demotion is safe.
					if count >= 2 {
						break
					}
				}
			}
			if count >= 2 || len(list) < pageSize {
				break
			}
		}
		// <= 1 means the target is the only approved system_admin.
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
