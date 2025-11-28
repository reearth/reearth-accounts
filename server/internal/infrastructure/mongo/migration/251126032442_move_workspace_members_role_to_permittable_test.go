package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestMoveWorkspaceMembersRoleToPermittable(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("Create new permittables with workspace roles", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role4", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert test workspace with members
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user1": bson.M{
					"role":      "owner",
					"invitedby": "user1",
					"disabled":  false,
				},
				"user2": bson.M{
					"role":      "writer",
					"invitedby": "user1",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify permittable for user1 was created
		var result1 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result1)
		assert.NoError(t, err)
		assert.Equal(t, "user1", result1["userid"])

		// Check workspace_roles
		workspaceRoles, ok := result1["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		// Verify the workspace role
		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"]) // owner role

		// Verify user_roles contains "self"
		userRoles, ok := result1["user_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, userRoles, 1)
		assert.Equal(t, "role4", userRoles[0]) // self role

		// Verify permittable for user2 was created
		var result2 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user2"}).Decode(&result2)
		assert.NoError(t, err)
		assert.Equal(t, "user2", result2["userid"])

		workspaceRoles2, ok := result2["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles2, 1)

		wsRole2 := workspaceRoles2[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole2["workspace_id"])
		assert.Equal(t, "role2", wsRole2["role_id"]) // writer role
	})

	t.Run("Update existing permittables with workspace roles", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "maintainer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert existing permittable with user_roles
		existingPermittable := bson.M{
			"_id":        primitive.NewObjectID(),
			"id":         "permittable1",
			"userid":     "user1",
			"user_roles": []string{"role3"}, // self role
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Insert test workspace
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user1": bson.M{
					"role":      "owner",
					"invitedby": "user1",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify permittable was updated
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		// Check that user_roles is preserved
		userRoles, ok := result["user_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, userRoles, 1)
		assert.Equal(t, "role3", userRoles[0])

		// Check workspace_roles was added
		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("Handle multiple workspaces for same user", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert two workspaces with the same user
		workspaces := []interface{}{
			bson.M{
				"_id":   primitive.NewObjectID(),
				"id":    "workspace1",
				"alias": "workspace-1",
				"name":  "Workspace 1",
				"email": "ws1@example.com",
				"members": bson.M{
					"user1": bson.M{
						"role":      "owner",
						"invitedby": "user1",
						"disabled":  false,
					},
				},
				"integrations": bson.M{},
				"personal":     false,
			},
			bson.M{
				"_id":   primitive.NewObjectID(),
				"id":    "workspace2",
				"alias": "workspace-2",
				"name":  "Workspace 2",
				"email": "ws2@example.com",
				"members": bson.M{
					"user1": bson.M{
						"role":      "reader",
						"invitedby": "user1",
						"disabled":  false,
					},
				},
				"integrations": bson.M{},
				"personal":     false,
			},
		}
		_, err = workspaceCol.InsertMany(ctx, workspaces)
		assert.NoError(t, err)

		// Run migration
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify permittable has both workspace roles
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 2)

		// Verify both workspace roles exist
		workspaceIDs := make(map[string]string)
		for _, wr := range workspaceRoles {
			wsRole := wr.(bson.M)
			workspaceID := wsRole["workspace_id"].(string)
			roleID := wsRole["role_id"].(string)
			workspaceIDs[workspaceID] = roleID
		}

		assert.Equal(t, "role1", workspaceIDs["workspace1"]) // owner
		assert.Equal(t, "role2", workspaceIDs["workspace2"]) // reader
	})

	t.Run("Skip members with non-existent roles", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert only some roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert workspace with members having valid and invalid roles
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user1": bson.M{
					"role":      "owner", // valid
					"invitedby": "user1",
					"disabled":  false,
				},
				"user2": bson.M{
					"role":      "invalid_role", // invalid
					"invitedby": "user1",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify permittable for user1 was created
		var result1 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result1)
		assert.NoError(t, err)

		workspaceRoles1, ok := result1["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles1, 1)

		// Verify permittable for user2 was NOT created (invalid role)
		var result2 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user2"}).Decode(&result2)
		assert.Error(t, err) // Should not find the document
	})

	t.Run("Idempotent - running twice doesn't create duplicates", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert test workspace
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user1": bson.M{
					"role":      "owner",
					"invitedby": "user1",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration first time
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Get result after first run
		var result1 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result1)
		assert.NoError(t, err)

		workspaceRoles1, ok := result1["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles1, 1)

		// Run migration second time
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify no duplicates were created
		var result2 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result2)
		assert.NoError(t, err)

		workspaceRoles2, ok := result2["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles2, 1, "Should still have only 1 workspace role after running twice")

		// Verify it's the same workspace role
		wsRole := workspaceRoles2[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("Handle empty workspace members", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		workspaceCol := db.Collection("workspace")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "owner"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert workspace with no members
		testWorkspace := bson.M{
			"_id":          primitive.NewObjectID(),
			"id":           "workspace1",
			"alias":        "test-workspace",
			"name":         "Test Workspace",
			"email":        "test@example.com",
			"members":      bson.M{}, // Empty
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration - should not error
		err = MoveWorkspaceMembersRoleToPermittable(ctx, c)
		assert.NoError(t, err)

		// Verify no permittables were created
		count, err := permittableCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)
	})
}
