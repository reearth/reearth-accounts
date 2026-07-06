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

func pending(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusPending).MustBuild()
}

func approved(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusApproved).MustBuild()
}

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

	// persisted
	reloaded, err := repo.FindByID(ctx, target.ID())
	require.NoError(t, err)
	assert.True(t, reloaded.IsApproved())
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

func TestReject_Pending(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	target := pending("new@eukarya.io")
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewRejectAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), target.ID())
	require.NoError(t, err)
	assert.True(t, got.IsRejected())
}

func TestReject_RevokeApproved(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	other := approved("other@eukarya.io")
	repo := memory.NewAdminUserWith(operator, other)
	uc := NewRejectAdminUserUseCase(repo)

	got, err := uc.Execute(ctx, operator.ID(), other.ID())
	require.NoError(t, err)
	assert.True(t, got.IsRejected())
}

func TestReject_CannotRejectSelf(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	repo := memory.NewAdminUserWith(operator)
	uc := NewRejectAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), operator.ID())
	assert.ErrorIs(t, err, ErrCannotModifySelf)
}

func TestReject_LastApprovedAdminBlocked(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	// only one approved admin (the operator); revoking the sole other approved
	// admin is fine, but here target is the only approved besides operator...
	// construct: operator + one approved target; total approved = 2, so ok.
	// To hit the guard, make target the ONLY approved admin.
	target := approved("solo@eukarya.io")
	repo := memory.NewAdminUserWith(target) // operator not persisted; only target approved
	uc := NewRejectAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), target.ID())
	assert.ErrorIs(t, err, ErrLastApprovedAdmin)
}

func TestList_StatusFilter(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAdminUserWith(pending("p@eukarya.io"), approved("a@eukarya.io"))
	uc := NewListAdminUsersUseCase(repo)

	st := adminuser.StatusPending
	got, pi, err := uc.Execute(ctx, adminuser.ListFilter{Status: &st})
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, int64(1), pi.TotalCount)
	assert.True(t, got[0].IsPending())
}
