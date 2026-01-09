package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestFixPermittableWorkspaceRoles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("Remove workspace roles for non-member workspaces", func(t *testing.T) {
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
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert workspace WITHOUT user1 as member
		testWorkspace := bson.M{
			"_id":          primitive.NewObjectID(),
			"id":           "workspace1",
			"alias":        "test-workspace",
			"name":         "Test Workspace",
			"email":        "test@example.com",
			"members":      bson.M{}, // No members
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Insert permittable with stale workspace_roles (user was removed from workspace)
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role3"}, // self role
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"}, // stale entry
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles is now empty
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles := result["workspace_roles"]
		assert.Nil(t, workspaceRoles)
	})

	t.Run("Update workspace role when role changed", func(t *testing.T) {
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

		// Insert workspace with user1 as writer (changed from owner)
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user1": bson.M{
					"role":      "writer", // Current role
					"invitedby": "user1",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Insert permittable with old role (owner)
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role4"}, // self role
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"}, // old owner role
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles has updated role
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role2", wsRole["role_id"]) // updated to writer role
	})

	t.Run("Add missing workspace roles", func(t *testing.T) {
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

		// Insert workspace with user1 as member
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

		// Insert permittable WITHOUT workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role2"}, // self role
			// workspace_roles is missing
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles was added
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("Handle multiple workspaces with mixed membership", func(t *testing.T) {
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

		// Insert workspaces - user1 is member of ws1 but not ws2
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
				"_id":          primitive.NewObjectID(),
				"id":           "workspace2",
				"alias":        "workspace-2",
				"name":         "Workspace 2",
				"email":        "ws2@example.com",
				"members":      bson.M{}, // user1 is NOT a member
				"integrations": bson.M{},
				"personal":     false,
			},
		}
		_, err = workspaceCol.InsertMany(ctx, workspaces)
		assert.NoError(t, err)

		// Insert permittable with stale workspace_roles (includes ws2 where user is no longer a member)
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role3"}, // self role
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"}, // valid
				{"workspace_id": "workspace2", "role_id": "role2"}, // stale - user removed
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify only valid workspace_roles remain
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("No update when workspace_roles already correct", func(t *testing.T) {
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

		// Insert workspace
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

		// Insert permittable with correct workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role2"}, // self role
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"}, // correct
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles unchanged
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("Handle permittable without any workspace membership", func(t *testing.T) {
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

		// Insert workspace WITHOUT user1
		testWorkspace := bson.M{
			"_id":   primitive.NewObjectID(),
			"id":    "workspace1",
			"alias": "test-workspace",
			"name":  "Test Workspace",
			"email": "test@example.com",
			"members": bson.M{
				"user2": bson.M{ // different user
					"role":      "owner",
					"invitedby": "user2",
					"disabled":  false,
				},
			},
			"integrations": bson.M{},
			"personal":     false,
		}
		_, err = workspaceCol.InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Insert permittable for user1 with stale workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role2"},
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"}, // stale
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles is now empty
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles := result["workspace_roles"]
		assert.Nil(t, workspaceRoles)
	})

	t.Run("Handle disabled workspace members", func(t *testing.T) {
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
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert workspaces - user1 is active in ws1, disabled in ws2
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
						"disabled":  false, // active
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
						"role":      "writer",
						"invitedby": "user2",
						"disabled":  true, // disabled
					},
				},
				"integrations": bson.M{},
				"personal":     false,
			},
		}
		_, err = workspaceCol.InsertMany(ctx, workspaces)
		assert.NoError(t, err)

		// Insert permittable with workspace_roles for both workspaces
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role3"},
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace1", "role_id": "role1"},
				{"workspace_id": "workspace2", "role_id": "role2"},
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify only active workspace role remains (ws1), disabled workspace (ws2) should be removed
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1, "Should only have workspace role for active membership")

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})

	t.Run("Handle mixed active and disabled memberships across multiple workspaces", func(t *testing.T) {
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

		// Insert workspaces with various membership states
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
						"disabled":  false, // active
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
						"role":      "writer",
						"invitedby": "user2",
						"disabled":  true, // disabled
					},
				},
				"integrations": bson.M{},
				"personal":     false,
			},
			bson.M{
				"_id":   primitive.NewObjectID(),
				"id":    "workspace3",
				"alias": "workspace-3",
				"name":  "Workspace 3",
				"email": "ws3@example.com",
				"members": bson.M{
					"user1": bson.M{
						"role":      "reader",
						"invitedby": "user3",
						"disabled":  false, // active
					},
				},
				"integrations": bson.M{},
				"personal":     false,
			},
		}
		_, err = workspaceCol.InsertMany(ctx, workspaces)
		assert.NoError(t, err)

		// Insert permittable without workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role4"},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify only active workspace roles are added (ws1 and ws3, not ws2)
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 2, "Should have workspace roles for active memberships only")

		// Verify the workspace roles
		workspaceIDs := make(map[string]string)
		for _, wr := range workspaceRoles {
			wsRole := wr.(bson.M)
			workspaceID := wsRole["workspace_id"].(string)
			roleID := wsRole["role_id"].(string)
			workspaceIDs[workspaceID] = roleID
		}

		assert.Equal(t, "role1", workspaceIDs["workspace1"]) // owner - active
		assert.Equal(t, "role3", workspaceIDs["workspace3"]) // reader - active
		_, hasWs2 := workspaceIDs["workspace2"]
		assert.False(t, hasWs2, "Should not have workspace2 role (disabled)")
	})

	t.Run("Idempotent - running twice produces same result", func(t *testing.T) {
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

		// Insert workspace
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

		// Insert permittable with incorrect workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role2"},
			"workspace_roles": []bson.M{
				{"workspace_id": "workspace_old", "role_id": "role1"}, // stale
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration first time
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Run migration second time
		err = FixPermittableWorkspaceRoles(ctx, c)
		assert.NoError(t, err)

		// Verify result is correct
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "workspace1", wsRole["workspace_id"])
		assert.Equal(t, "role1", wsRole["role_id"])
	})
}

func TestWorkspaceRolesEqual(t *testing.T) {
	t.Run("Equal slices", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws2", RoleID: "role2"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws2", RoleID: "role2"},
		}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Different lengths", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws2", RoleID: "role2"},
		}
		assert.False(t, workspaceRolesEqual(a, b))
	})

	t.Run("Different role IDs", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role2"},
		}
		assert.False(t, workspaceRolesEqual(a, b))
	})

	t.Run("Different workspace IDs", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws2", RoleID: "role1"},
		}
		assert.False(t, workspaceRolesEqual(a, b))
	})

	t.Run("Both empty", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{}
		b := []mongodoc.WorkspaceRoleDocument{}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Both nil", func(t *testing.T) {
		var a, b []mongodoc.WorkspaceRoleDocument
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("One nil one empty", func(t *testing.T) {
		var a []mongodoc.WorkspaceRoleDocument
		b := []mongodoc.WorkspaceRoleDocument{}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Same elements different order", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws2", RoleID: "role2"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws2", RoleID: "role2"},
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Duplicate entries in both slices", func(t *testing.T) {
		// Note: Duplicates shouldn't occur in valid data, but the function should handle them
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role1"},
		}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Duplicate workspace with different roles", func(t *testing.T) {
		// This is invalid data but tests the function's behavior
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role2"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role2"},
		}
		assert.True(t, workspaceRolesEqual(a, b))
	})

	t.Run("Duplicate workspace with different roles mismatched", func(t *testing.T) {
		a := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role2"},
		}
		b := []mongodoc.WorkspaceRoleDocument{
			{WorkspaceID: "ws1", RoleID: "role1"},
			{WorkspaceID: "ws1", RoleID: "role3"},
		}
		assert.False(t, workspaceRolesEqual(a, b))
	})
}
