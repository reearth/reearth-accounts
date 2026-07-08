package useruc

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/memory"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func testUser(name, alias, email string) *user.User {
	return user.New().NewID().Name(name).Alias(alias).Email(email).MustBuild()
}

func TestListUsers_All(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserWith(
		testUser("Alpha", "alpha", "alpha@example.com"),
		testUser("Beta", "beta", "beta@example.com"),
	)
	uc := NewListUsersUseCase(repo)

	got, pi, err := uc.Execute(ctx, nil, nil)
	require.NoError(t, err)
	assert.Equal(t, 2, len(got))
	assert.Equal(t, int64(2), pi.TotalCount)
}

func TestListUsers_Keyword(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserWith(
		testUser("Alpha", "alpha", "alpha@example.com"),
		testUser("Beta", "beta", "beta@example.com"),
	)
	uc := NewListUsersUseCase(repo)

	kw := "alph"
	got, pi, err := uc.Execute(ctx, &kw, nil)
	require.NoError(t, err)
	require.Equal(t, 1, len(got))
	assert.Equal(t, "Alpha", got[0].Name())
	assert.Equal(t, int64(1), pi.TotalCount)
}

func TestListUsers_Pagination(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewUserWith(
		testUser("A", "a", "a@example.com"),
		testUser("B", "b", "b@example.com"),
		testUser("C", "c", "c@example.com"),
	)
	uc := NewListUsersUseCase(repo)

	p := usecasex.OffsetPagination{Offset: 1, Limit: 1}.Wrap()
	got, pi, err := uc.Execute(ctx, nil, p)
	require.NoError(t, err)
	assert.Equal(t, 1, len(got))
	assert.Equal(t, int64(3), pi.TotalCount)
	assert.True(t, pi.HasNextPage)
	assert.True(t, pi.HasPreviousPage)
}
