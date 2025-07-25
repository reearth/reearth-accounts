package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/pkg/workspace"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

// go test -v -run TestRemoveMultipleWorkspaceOwners ./internal/infrastructure/mongo/migration/...

func init() {
	mongotest.Env = "REEARTH_ACCOUNTS_DB"
}

func TestRemoveMultipleWorkspaceOwners(t *testing.T) {
	db := mongotest.Connect(t)(t)
	client := mongox.NewClientWithDatabase(db)
	ctx := context.Background()

	workspaceCollection := client.WithCollection("workspace").Client()

	t.Run("workspace with single owner should not change", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace with single owner
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws1",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited
					},
					"user2": bson.M{
						"role":      string(workspace.RoleMaintainer),
						"invitedby": "user1",
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify workspace is unchanged (single owner)
		var ws1 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws1"}).Decode(&ws1)
		require.NoError(t, err)

		members := ws1["members"].(bson.M)
		user1 := members["user1"].(bson.M)
		user2 := members["user2"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1["role"])
		assert.Equal(t, string(workspace.RoleMaintainer), user2["role"])
	})

	t.Run("workspace with multiple owners - demote non-self-invited", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace with multiple owners
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws2",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited, should remain owner
					},
					"user2": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // invited by user1, should become maintainer
					},
					"user3": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user3", // self-invited, should remain owner
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify changes
		var ws2 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws2"}).Decode(&ws2)
		require.NoError(t, err)

		members := ws2["members"].(bson.M)
		user1 := members["user1"].(bson.M)
		user2 := members["user2"].(bson.M)
		user3 := members["user3"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1["role"], "user1 should remain owner (self-invited)")
		assert.Equal(t, string(workspace.RoleMaintainer), user2["role"], "user2 should become maintainer (not self-invited)")
		assert.Equal(t, string(workspace.RoleOwner), user3["role"], "user3 should remain owner (self-invited)")
	})

	t.Run("workspace with multiple owners - all non-self-invited become maintainers", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace with multiple owners, only one self-invited
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws3",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited, should remain owner
					},
					"user2": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // invited by user1, should become maintainer
					},
					"user3": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // invited by user1, should become maintainer
					},
					"user4": bson.M{
						"role":      string(workspace.RoleMaintainer),
						"invitedby": "user1", // already maintainer, no change
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify changes
		var ws3 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws3"}).Decode(&ws3)
		require.NoError(t, err)

		members := ws3["members"].(bson.M)
		user1 := members["user1"].(bson.M)
		user2 := members["user2"].(bson.M)
		user3 := members["user3"].(bson.M)
		user4 := members["user4"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1["role"], "user1 should remain owner (self-invited)")
		assert.Equal(t, string(workspace.RoleMaintainer), user2["role"], "user2 should become maintainer (not self-invited)")
		assert.Equal(t, string(workspace.RoleMaintainer), user3["role"], "user3 should become maintainer (not self-invited)")
		assert.Equal(t, string(workspace.RoleMaintainer), user4["role"], "user4 should remain maintainer")
	})

	t.Run("workspace with no owners should not change", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace with no owners
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws4",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleMaintainer),
						"invitedby": "user1",
					},
					"user2": bson.M{
						"role":      string(workspace.RoleMaintainer),
						"invitedby": "user1",
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify no changes (no owners to process)
		var ws4 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws4"}).Decode(&ws4)
		require.NoError(t, err)

		members := ws4["members"].(bson.M)
		user1 := members["user1"].(bson.M)
		user2 := members["user2"].(bson.M)

		assert.Equal(t, string(workspace.RoleMaintainer), user1["role"])
		assert.Equal(t, string(workspace.RoleMaintainer), user2["role"])
	})

	t.Run("multiple workspaces with mixed scenarios", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert multiple workspaces with different scenarios
		workspaceData := []interface{}{
			// Workspace with single owner - no changes
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws5",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1",
					},
				},
			},
			// Workspace with multiple owners - changes needed
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws6",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited
					},
					"user2": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // not self-invited
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify ws5 (single owner) is unchanged
		var ws5 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws5"}).Decode(&ws5)
		require.NoError(t, err)

		members5 := ws5["members"].(bson.M)
		user1_ws5 := members5["user1"].(bson.M)
		assert.Equal(t, string(workspace.RoleOwner), user1_ws5["role"])

		// Verify ws6 (multiple owners) has changes
		var ws6 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws6"}).Decode(&ws6)
		require.NoError(t, err)

		members6 := ws6["members"].(bson.M)
		user1_ws6 := members6["user1"].(bson.M)
		user2_ws6 := members6["user2"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1_ws6["role"], "user1 should remain owner")
		assert.Equal(t, string(workspace.RoleMaintainer), user2_ws6["role"], "user2 should become maintainer")
	})

	t.Run("handles empty collection gracefully", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Execute migration on empty collection
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify collection is still empty
		count, err := workspaceCollection.CountDocuments(ctx, bson.M{})
		require.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})

	t.Run("workspace with all owners self-invited should not change", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace where all owners are self-invited
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws_all_self",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited
					},
					"user2": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user2", // self-invited
					},
					"user3": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user3", // self-invited
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Execute migration
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify all owners remain owners since they are self-invited
		var ws bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws_all_self"}).Decode(&ws)
		require.NoError(t, err)

		members := ws["members"].(bson.M)
		for userID := range members {
			member := members[userID].(bson.M)
			assert.Equal(t, string(workspace.RoleOwner), member["role"], "User %s should remain owner", userID)
		}
	})

	t.Run("idempotent - running migration twice has no additional effect", func(t *testing.T) {
		// Clear collection
		_, err := workspaceCollection.DeleteMany(ctx, bson.M{})
		require.NoError(t, err)

		// Setup: Insert workspace with multiple owners
		workspaceData := []interface{}{
			bson.M{
				"_id": primitive.NewObjectID(),
				"id":  "ws_idempotent",
				"members": bson.M{
					"user1": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // self-invited
					},
					"user2": bson.M{
						"role":      string(workspace.RoleOwner),
						"invitedby": "user1", // not self-invited
					},
				},
			},
		}

		_, err = workspaceCollection.InsertMany(ctx, workspaceData)
		require.NoError(t, err)

		// Run migration first time
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify changes after first run
		var ws1 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws_idempotent"}).Decode(&ws1)
		require.NoError(t, err)

		members1 := ws1["members"].(bson.M)
		user1_first := members1["user1"].(bson.M)
		user2_first := members1["user2"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1_first["role"])
		assert.Equal(t, string(workspace.RoleMaintainer), user2_first["role"])

		// Run migration second time
		err = RemoveMultipleWorkspaceOwners(ctx, client)
		require.NoError(t, err)

		// Verify no additional changes occurred
		var ws2 bson.M
		err = workspaceCollection.FindOne(ctx, bson.M{"id": "ws_idempotent"}).Decode(&ws2)
		require.NoError(t, err)

		members2 := ws2["members"].(bson.M)
		user1_second := members2["user1"].(bson.M)
		user2_second := members2["user2"].(bson.M)

		assert.Equal(t, string(workspace.RoleOwner), user1_second["role"])
		assert.Equal(t, string(workspace.RoleMaintainer), user2_second["role"])

		// Verify only one document exists
		count, err := workspaceCollection.CountDocuments(ctx, bson.M{})
		require.NoError(t, err)
		assert.Equal(t, int64(1), count)
	})
}
