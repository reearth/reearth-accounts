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

	// Demoting an approved system_admin must be blocked if it is the last one.
	// The existence check and the save are performed as a single atomic
	// operation by the repo (SaveGuardingLastSystemAdmin) so two concurrent
	// demotions of the last two admins can't both pass an independent check.
	demotingLastSystemAdmin := target.IsApproved() && target.Role() == adminuser.RoleSystemAdmin && in.Role != adminuser.RoleSystemAdmin

	if err := target.SetRole(in.Role); err != nil {
		return nil, err
	}

	ok, err := uc.adminUserRepo.SaveGuardingLastSystemAdmin(ctx, target, demotingLastSystemAdmin)
	if err != nil {
		return nil, err
	}
	if !ok {
		return nil, ErrLastSystemAdmin
	}
	return target, nil
}
