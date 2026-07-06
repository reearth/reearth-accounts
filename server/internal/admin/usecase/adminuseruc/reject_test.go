package adminuseruc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
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

func TestReject_LastApprovedAdminBlocked(t *testing.T) {
	ctx := context.Background()
	operator := approved("op@eukarya.io")
	// target is the only approved admin in the repo; rejecting it would leave
	// zero approved admins.
	target := approved("solo@eukarya.io")
	repo := memory.NewAdminUserWith(target)
	uc := NewRejectAdminUserUseCase(repo)

	_, err := uc.Execute(ctx, operator.ID(), target.ID())
	assert.ErrorIs(t, err, ErrLastApprovedAdmin)
}
