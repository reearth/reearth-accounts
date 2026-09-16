//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestWorkspace_Save_MembersBulkInsert exercises the WorkspaceMembersInsertBulk
// / WorkspaceIntegrationsInsertBulk path (SCA-03): saving a workspace with
// several members and integrations, then round-tripping it through a real
// Postgres to catch anything a static sqlc type-check can't (jsonb key casing,
// jsonb_to_recordset column matching, etc).
func TestWorkspace_Save_MembersBulkInsert(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewWorkspace(NewClient(pool))

	ownerID := id.NewUserID()
	writerID := id.NewUserID()
	readerID := id.NewUserID()
	iid := id.NewIntegrationID()

	ws := workspace.New().NewID().Name("bulk-insert-corp").Alias("bulk-insert-corp").
		Members(map[user.ID]workspace.Member{
			ownerID:  {Role: role.RoleOwner, InvitedBy: ownerID},
			writerID: {Role: role.RoleWriter, InvitedBy: ownerID},
			readerID: {Role: role.RoleReader, InvitedBy: ownerID, Disabled: true},
		}).
		Integrations(map[id.IntegrationID]workspace.Member{
			iid: {Role: role.RoleMaintainer, InvitedBy: ownerID},
		}).
		MustBuild()

	require.NoError(t, r.Create(ctx, ws))

	got, err := r.FindByID(ctx, ws.ID())
	require.NoError(t, err)

	assert.Equal(t, role.RoleOwner, got.Members().UserRole(ownerID))
	assert.Equal(t, role.RoleWriter, got.Members().UserRole(writerID))
	assert.Equal(t, role.RoleReader, got.Members().UserRole(readerID))
	if m := got.Members().User(readerID); assert.NotNil(t, m) {
		assert.True(t, m.Disabled)
	}
	assert.Equal(t, role.RoleMaintainer, got.Members().IntegrationRole(iid))

	// Save again with a changed member set: exercises the delete-then-bulk-
	// insert path with a different row count than the original insert.
	require.NoError(t, got.Members().Leave(readerID))
	require.NoError(t, r.Save(ctx, got))

	reloaded, err := r.FindByID(ctx, ws.ID())
	require.NoError(t, err)
	assert.Equal(t, role.RoleOwner, reloaded.Members().UserRole(ownerID))
	assert.Equal(t, role.RoleWriter, reloaded.Members().UserRole(writerID))
	assert.False(t, reloaded.Members().HasUser(readerID))
	assert.Equal(t, role.RoleMaintainer, reloaded.Members().IntegrationRole(iid))
}
