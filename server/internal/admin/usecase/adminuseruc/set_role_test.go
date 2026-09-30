package adminuseruc

import (
	"context"
	"sync"
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

	got, err := uc.Execute(ctx, SetRoleInput{TargetID: target.ID(), Role: adminuser.RoleViewer})
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

	_, err := uc.Execute(ctx, SetRoleInput{TargetID: target.ID(), Role: adminuser.RoleViewer})
	assert.ErrorIs(t, err, ErrLastSystemAdmin)
}

func TestSetRole_DemoteRejectedSystemAdmin_OK(t *testing.T) {
	ctx := context.Background()
	// target is a rejected system_admin, so it isn't counted and demotion is allowed.
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := rejectedWithRole("target@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewSetRoleUseCase(repo)

	got, err := uc.Execute(ctx, SetRoleInput{TargetID: target.ID(), Role: adminuser.RoleViewer})
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

	got, err := uc.Execute(ctx, SetRoleInput{TargetID: target.ID(), Role: adminuser.RoleSystemAdmin})
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleSystemAdmin, got.Role())
}

func TestSetRole_InvalidRole(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	target := approvedWithRole("target@eukarya.io", adminuser.RoleViewer)
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, SetRoleInput{TargetID: target.ID(), Role: adminuser.Role("bogus")})
	assert.ErrorIs(t, err, adminuser.ErrInvalidRole)
}

// An invalid role is reported as ErrInvalidRole even for the last system_admin.
func TestSetRole_InvalidRole_LastSystemAdmin(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, SetRoleInput{TargetID: operator.ID(), Role: adminuser.Role("bogus")})
	assert.ErrorIs(t, err, adminuser.ErrInvalidRole)
}

// Two approved system_admins demoted concurrently must not both succeed: the
// existence check and the save now happen atomically per-repo (see
// adminuser.Repo.SaveGuardingLastSystemAdmin), so exactly one demotion must be
// rejected with ErrLastSystemAdmin, leaving one approved system_admin behind.
func TestSetRole_ConcurrentDemoteLastTwoSystemAdmins(t *testing.T) {
	ctx := context.Background()
	a := approvedWithRole("a@eukarya.io", adminuser.RoleSystemAdmin)
	b := approvedWithRole("b@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(a, b)
	uc := NewSetRoleUseCase(repo)

	var wg sync.WaitGroup
	errs := make([]error, 2)
	targets := []*adminuser.AdminUser{a, b}
	for i := range targets {
		wg.Add(1)
		go func(i int) {
			defer wg.Done()
			_, errs[i] = uc.Execute(ctx, SetRoleInput{TargetID: targets[i].ID(), Role: adminuser.RoleViewer})
		}(i)
	}
	wg.Wait()

	blocked, succeeded := 0, 0
	for _, err := range errs {
		switch {
		case err == nil:
			succeeded++
		case assert.ErrorIs(t, err, ErrLastSystemAdmin):
			blocked++
		}
	}
	assert.Equal(t, 1, succeeded, "exactly one demotion must succeed")
	assert.Equal(t, 1, blocked, "exactly one demotion must be blocked")

	hasOther, err := repo.ExistsApprovedSystemAdminExcept(ctx, adminuser.NewID())
	require.NoError(t, err)
	assert.True(t, hasOther, "at least one approved system_admin must remain")
}

func TestSetRole_NotFound(t *testing.T) {
	ctx := context.Background()
	operator := approvedWithRole("op@eukarya.io", adminuser.RoleSystemAdmin)
	repo := memory.NewAdminUserWith(operator)
	uc := NewSetRoleUseCase(repo)

	_, err := uc.Execute(ctx, SetRoleInput{TargetID: adminuser.NewID(), Role: adminuser.RoleViewer})
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}
