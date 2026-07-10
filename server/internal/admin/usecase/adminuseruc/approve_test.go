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

func TestApprove(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	target := pending("new@eukarya.io")
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewApproveAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID())
	require.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.Equal(t, operator.ID(), got.ApprovedBy())

	reloaded, err := repo.FindByID(ctx, target.ID())
	require.NoError(t, err)
	assert.True(t, reloaded.IsApproved())
}

func TestApprove_DefaultsRoleToViewer(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	target := pending("new@eukarya.io") // no role set
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewApproveAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, got.Role())

	reloaded, err := repo.FindByID(ctx, target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleViewer, reloaded.Role())
}

func TestApprove_KeepsExistingRole(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	target := pending("new@eukarya.io")
	require.NoError(t, target.SetRole(adminuser.RoleSystemAdmin))
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewApproveAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID())
	require.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.Equal(t, adminuser.RoleSystemAdmin, got.Role()) // not downgraded to viewer
}

func TestApprove_AlreadyApprovedDoesNotChangeRole(t *testing.T) {
	ctx := context.Background()
	firstApprover := adminuser.NewID()
	target := pending("t@eukarya.io")
	require.NoError(t, target.SetRole(adminuser.RoleSystemAdmin))
	target.Approve(firstApprover) // pending -> approved with system_admin
	repo := memory.NewAdminUserWith(target)
	uc := NewApproveAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, adminuser.NewID(), target.ID())
	require.NoError(t, err)
	assert.Equal(t, adminuser.RoleSystemAdmin, got.Role()) // idempotent path untouched
}

func TestApprove_AlreadyApprovedIsIdempotent(t *testing.T) {
	ctx := context.Background()
	firstApprover := adminuser.NewID()
	target := pending("t@eukarya.io")
	target.Approve(firstApprover) // pending -> approved by firstApprover
	repo := memory.NewAdminUserWith(target)
	uc := NewApproveAdminUserUseCase(repo)

	// a different operator re-approves: no-op, original approver preserved
	got, err := uc.Execute(ctx, adminuser.NewID(), target.ID())
	require.NoError(t, err)
	assert.True(t, got.IsApproved())
	assert.Equal(t, firstApprover, got.ApprovedBy())
}

func TestApprove_CannotApproveSelf(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	repo := memory.NewAdminUserWith(operator)
	uc := NewApproveAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), operator.ID())
	assert.ErrorIs(t, err, ErrCannotModifySelf)
}

func TestApprove_NotFound(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	repo := memory.NewAdminUserWith(operator)
	uc := NewApproveAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), adminuser.NewID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}
