package adminuseruc

import (
	"context"

	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
)

// SetRoleUseCase assigns a role to an admin user.
type SetRoleUseCase struct {
	adminUserRepo adminuser.Repo
}

// NewSetRoleUseCase is a Wire provider for SetRoleUseCase.
func NewSetRoleUseCase(adminUserRepo adminuser.Repo) *SetRoleUseCase {
	return &SetRoleUseCase{adminUserRepo: adminUserRepo}
}

// SetRoleInput is the input for SetRoleUseCase.Execute.
type SetRoleInput struct {
	TargetID adminuser.ID
	Role     adminuser.Role
}

// Execute assigns a role to the target admin user. Self-role changes are allowed
// (RBAC is enforced in the middleware), but the last approved system_admin
// cannot be demoted.
func (uc *SetRoleUseCase) Execute(ctx context.Context, in SetRoleInput) (*adminuser.AdminUser, error) {
	// Validate before loading the target so a bad input maps to ErrInvalidRole.
	if !in.Role.Valid() {
		return nil, adminuser.ErrInvalidRole
	}

	target, err := uc.adminUserRepo.FindByID(ctx, in.TargetID)
	if err != nil {
		return nil, err
	}

	// Demoting an approved system_admin is blocked if it is the last one
	// (check-then-act, not atomic; acceptable for the tiny closed admin set).
	if target.IsApproved() && target.Role() == adminuser.RoleSystemAdmin && in.Role != adminuser.RoleSystemAdmin {
		hasOther, err := uc.adminUserRepo.ExistsApprovedSystemAdminExcept(ctx, target.ID())
		if err != nil {
			return nil, err
		}
		if !hasOther {
			return nil, ErrLastSystemAdmin
		}
	}

	if err := target.SetRole(in.Role); err != nil {
		return nil, err
	}
	if err := uc.adminUserRepo.Save(ctx, target); err != nil {
		return nil, err
	}
	return target, nil
}
