//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/permittable"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestPermittable_Save_WorkspaceRolesBulkInsert exercises the
// PermittableWorkspaceRolesInsertBulk path (SCA-04): saving a permittable
// with several workspace roles, then round-tripping it through a real
// Postgres to catch anything a static sqlc type-check can't (jsonb key
// casing, jsonb_to_recordset column matching, etc).
func TestPermittable_Save_WorkspaceRolesBulkInsert(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewPermittable(NewClient(pool))

	uid := id.NewUserID()
	rid := id.NewRoleID()
	wid1 := id.NewWorkspaceID()
	wid2 := id.NewWorkspaceID()
	wid3 := id.NewWorkspaceID()

	p := permittable.New().NewID().UserID(uid).RoleIDs([]id.RoleID{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(wid1, rid),
			permittable.NewWorkspaceRole(wid2, rid),
			permittable.NewWorkspaceRole(wid3, rid),
		}).MustBuild()

	require.NoError(t, r.Save(ctx, *p))

	got, err := r.FindByUserID(ctx, uid)
	require.NoError(t, err)

	gotWorkspaceIDs := make(map[id.WorkspaceID]struct{}, len(got.WorkspaceRoles()))
	for _, wr := range got.WorkspaceRoles() {
		gotWorkspaceIDs[wr.ID()] = struct{}{}
	}
	assert.Len(t, got.WorkspaceRoles(), 3)
	assert.Contains(t, gotWorkspaceIDs, wid1)
	assert.Contains(t, gotWorkspaceIDs, wid2)
	assert.Contains(t, gotWorkspaceIDs, wid3)

	// Save again with a changed role set (same user, same permittable id kept
	// by ON CONFLICT): exercises the delete-then-bulk-insert path with a
	// different row count than the original insert.
	p2 := permittable.New().NewID().UserID(uid).RoleIDs([]id.RoleID{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(wid1, rid),
		}).MustBuild()
	require.NoError(t, r.Save(ctx, *p2))

	reloaded, err := r.FindByUserID(ctx, uid)
	require.NoError(t, err)
	assert.Len(t, reloaded.WorkspaceRoles(), 1)
	assert.Equal(t, wid1, reloaded.WorkspaceRoles()[0].ID())
}

// TestPermittable_SaveMany_WorkspaceRolesBulkInsert covers the same path via
// SaveMany, batching several permittables in one transaction.
func TestPermittable_SaveMany_WorkspaceRolesBulkInsert(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewPermittable(NewClient(pool))

	rid := id.NewRoleID()
	uid1, uid2 := id.NewUserID(), id.NewUserID()
	wid1, wid2 := id.NewWorkspaceID(), id.NewWorkspaceID()

	p1 := permittable.New().NewID().UserID(uid1).RoleIDs([]id.RoleID{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{permittable.NewWorkspaceRole(wid1, rid)}).MustBuild()
	p2 := permittable.New().NewID().UserID(uid2).RoleIDs([]id.RoleID{rid}).
		WorkspaceRoles([]permittable.WorkspaceRole{
			permittable.NewWorkspaceRole(wid1, rid),
			permittable.NewWorkspaceRole(wid2, rid),
		}).MustBuild()

	require.NoError(t, r.SaveMany(ctx, permittable.List{p1, p2}))

	got1, err := r.FindByUserID(ctx, uid1)
	require.NoError(t, err)
	assert.Len(t, got1.WorkspaceRoles(), 1)

	got2, err := r.FindByUserID(ctx, uid2)
	require.NoError(t, err)
	assert.Len(t, got2.WorkspaceRoles(), 2)
}
