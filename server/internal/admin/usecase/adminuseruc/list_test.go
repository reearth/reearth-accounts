package adminuseruc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/adminuser"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func pending(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusPending).MustBuild()
}

func approved(email string) *adminuser.AdminUser {
	return adminuser.New().NewID().Name(email).Email(email).Status(adminuser.StatusApproved).MustBuild()
}

func TestList_All(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAdminUserWith(pending("p@eukarya.io"), approved("a@eukarya.io"))
	uc := NewListAdminUsersUseCase(repo)

	got, pi, err := uc.Execute(ctx, adminuser.ListFilter{})
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)
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
