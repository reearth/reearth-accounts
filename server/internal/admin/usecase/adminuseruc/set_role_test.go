package adminuseruc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/reearth/reearthx/rerror"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func approvedWithRole(email string, role adminuser.Role) *adminuser.AdminUser {
	u := approved(email)
	if err := u.SetRole(role); err != nil {
		panic(err)
	}
	return u
}

func rejectedWithRole(email string, role adminuser.Role) *adminuser.AdminUser {
	u := adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusRejected).MustBuild()
	if err := u.SetRole(role); err != nil {
		panic(err)
	}
	return u
}

func TestSetRole_DemoteSystemAdmin_OK(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := approvedWithRole("target@eukarya.io", adminuser.RoleSystemAdmin)
	other := approvedWithRole("other@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator, target, other)
	uc := NewSetRoleUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID(), adminuser.RoleViewer)
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, got.Role())

	reloaded, err := repo.FindByID(ctx, target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, reloaded.Role())
}

func TestSetRole_DemoteLastSystemAdminBlocked(t *testing.T) {
	ctx := context.Background()
	// target is the only approved system_admin, so demoting it is blocked.
	target := approvedWithRole("solo@eukarya.io", adminuser.RoleSystemAdmin)
	viewer := approvedWithRole("viewer@eukarya.io", adminuser.RoleViewer)
	repo := memory.NewAdminUserWith(target, viewer)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, target.ID(), target.ID(), adminuser.RoleViewer)
	assert.ErrorIs(t, err, ErrLastSystemAdmin)
}

func TestSetRole_DemoteRejectedSystemAdmin_OK(t *testing.T) {
	ctx := context.Background()
	// target is a rejected system_admin, so it isn't counted and demotion is allowed.
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := rejectedWithRole("target@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewSetRoleUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID(), adminuser.RoleViewer)
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, got.Role())

	reloaded, err := repo.FindByID(ctx, target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, reloaded.Role())
}

func TestSetRole_PromoteViewer_OK(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := approvedWithRole("target@eukarya.io", adminuser.RoleViewer)
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewSetRoleUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID(), adminuser.RoleSystemAdmin)
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleSystemAdmin, got.Role())
}

func TestSetRole_InvalidRole(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := approvedWithRole("target@eukarya.io", adminuser.RoleViewer)
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), target.ID(), adminuser.Role("bogus"))
	assert.ErrorIs(t, err, adminuser.ErrInvalidRole)
}

// An invalid role is reported as ErrInvalidRole even for the last system_admin.
func TestSetRole_InvalidRole_LastSystemAdmin(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), operator.ID(), adminuser.Role("bogus"))
	assert.ErrorIs(t, err, adminuser.ErrInvalidRole)
}

func TestSetRole_NotFound(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), adminuser.NewID(), adminuser.RoleViewer)
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}
