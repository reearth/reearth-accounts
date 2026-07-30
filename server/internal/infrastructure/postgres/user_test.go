//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearthx/usecasex"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestUser_FindAllWithPagination(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewUser(NewClient(pool))

	uAlpha := user.New().NewID().Name("alpha").Email("alpha@example.com").MustBuild()
	uBeta := user.New().NewID().Name("beta").Email("beta@example.com").MustBuild()
	require.NoError(t, r.Create(ctx, uAlpha))
	require.NoError(t, r.Create(ctx, uBeta))

	uBeta.Deactivate()
	require.NoError(t, r.Save(ctx, uBeta))

	t.Run("status all returns everything regardless of soft-delete", func(t *testing.T) {
		list, pageInfo, err := r.FindAllWithPagination(ctx, nil, user.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 2)
		assert.EqualValues(t, 2, pageInfo.TotalCount)
	})

	t.Run("status active excludes soft-deleted users", func(t *testing.T) {
		list, pageInfo, err := r.FindAllWithPagination(ctx, nil, user.StatusActive, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, uAlpha.ID(), list[0].ID())
		assert.EqualValues(t, 1, pageInfo.TotalCount)
	})

	t.Run("status deleted returns only soft-deleted users", func(t *testing.T) {
		list, pageInfo, err := r.FindAllWithPagination(ctx, nil, user.StatusDeleted, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, uBeta.ID(), list[0].ID())
		assert.EqualValues(t, 1, pageInfo.TotalCount)
	})

	t.Run("keyword matches name case-insensitively", func(t *testing.T) {
		list, _, err := r.FindAllWithPagination(ctx, lo.ToPtr("ALPHA"), user.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, uAlpha.ID(), list[0].ID())
	})

	t.Run("keyword matches email", func(t *testing.T) {
		list, _, err := r.FindAllWithPagination(ctx, lo.ToPtr("beta@example.com"), user.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, uBeta.ID(), list[0].ID())
	})

	t.Run("pagination limits the page size", func(t *testing.T) {
		list, pageInfo, err := r.FindAllWithPagination(ctx, nil, user.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 1}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.EqualValues(t, 2, pageInfo.TotalCount)
	})
}
