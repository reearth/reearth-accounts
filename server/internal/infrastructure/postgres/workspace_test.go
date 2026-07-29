//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
	"github.com/samber/lo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestWorkspace_FindAll(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewWorkspace(NewClient(pool))

	wsAlpha := workspace.New().NewID().Name("alpha-corp").Alias("alpha-corp").MustBuild()
	wsBeta := workspace.New().NewID().Name("beta-inc").Alias("beta-inc").MustBuild()
	require.NoError(t, r.Create(ctx, wsAlpha))
	require.NoError(t, r.Create(ctx, wsBeta))

	wsBeta.Delete()
	require.NoError(t, r.Save(ctx, wsBeta))

	t.Run("status all returns everything regardless of soft-delete", func(t *testing.T) {
		list, pageInfo, err := r.FindAll(ctx, nil, workspace.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 2)
		assert.EqualValues(t, 2, pageInfo.TotalCount)
	})

	t.Run("status active excludes soft-deleted workspaces", func(t *testing.T) {
		list, pageInfo, err := r.FindAll(ctx, nil, workspace.StatusActive, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, wsAlpha.ID(), list[0].ID())
		assert.EqualValues(t, 1, pageInfo.TotalCount)
	})

	t.Run("status deleted returns only soft-deleted workspaces", func(t *testing.T) {
		list, pageInfo, err := r.FindAll(ctx, nil, workspace.StatusDeleted, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, wsBeta.ID(), list[0].ID())
		assert.EqualValues(t, 1, pageInfo.TotalCount)
	})

	t.Run("keyword matches name case-insensitively", func(t *testing.T) {
		list, _, err := r.FindAll(ctx, lo.ToPtr("ALPHA"), workspace.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, wsAlpha.ID(), list[0].ID())
	})

	t.Run("keyword matches alias", func(t *testing.T) {
		list, _, err := r.FindAll(ctx, lo.ToPtr("beta-inc"), workspace.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.Equal(t, wsBeta.ID(), list[0].ID())
	})

	t.Run("pagination limits the page size", func(t *testing.T) {
		list, pageInfo, err := r.FindAll(ctx, nil, workspace.StatusAll, usecasex.OffsetPagination{Offset: 0, Limit: 1}.Wrap())
		assert.NoError(t, err)
		assert.Len(t, list, 1)
		assert.EqualValues(t, 2, pageInfo.TotalCount)
	})
}
