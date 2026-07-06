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
