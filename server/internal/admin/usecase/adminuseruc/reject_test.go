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

func TestReject_NotFound(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	repo := memory.NewAdminUserWith(operator)
	uc := NewRejectAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), adminuser.NewID())
	assert.ErrorIs(t, err, rerror.ErrNotFound)
}

func TestReject_LastApprovedAdminBlocked(t *testing.T) {
	ctx := context.Background()
	// The target is the only approved admin in the repo; the operator is still
	// pending. Rejecting the sole approved admin would leave zero approved
	// admins, so it must be blocked.
	operator := pending("op@eukarya.io")
	target := approved("solo@eukarya.io")
	repo := memory.NewAdminUserWith(operator, target)
	uc := NewRejectAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), target.ID())
	assert.ErrorIs(t, err, ErrLastApprovedAdmin)
}
