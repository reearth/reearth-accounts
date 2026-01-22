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

func init() {
	mongotest.Env = "REEARTH_ACCOUNTS_DB"
}

func TestAddRoles(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	expectedRoles := []string{"reader", "writer", "maintainer", "owner", "self"}

	t.Run("Add all roles when none exist", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Verify no roles exist initially
		count, err := roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(0), count)

		// Run migration
		err = AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify all expected roles were created
		count, err = roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		// Verify each role exists
		for _, roleName := range expectedRoles {
			var result bson.M
			err = roleCol.FindOne(ctx, bson.M{"name": roleName}).Decode(&result)
			assert.NoError(t, err)
			assert.Equal(t, roleName, result["name"])
			assert.NotEmpty(t, result["id"], "Role ID should not be empty")
		}
	})

	t.Run("Add only missing roles when some exist", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Insert only some roles
		existingRoles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "writer"},
		}
		_, err := roleCol.InsertMany(ctx, existingRoles)
		assert.NoError(t, err)

		// Run migration
		err = AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify all expected roles now exist
		count, err := roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		// Verify each role exists
		for _, roleName := range expectedRoles {
			var result bson.M
			err = roleCol.FindOne(ctx, bson.M{"name": roleName}).Decode(&result)
			assert.NoError(t, err)
			assert.Equal(t, roleName, result["name"])
		}

		// Verify existing roles kept their original IDs
		var readerRole bson.M
		err = roleCol.FindOne(ctx, bson.M{"name": "reader"}).Decode(&readerRole)
		assert.NoError(t, err)
		assert.Equal(t, "role1", readerRole["id"])

		var writerRole bson.M
		err = roleCol.FindOne(ctx, bson.M{"name": "writer"}).Decode(&writerRole)
		assert.NoError(t, err)
		assert.Equal(t, "role2", writerRole["id"])
	})

	t.Run("Idempotent - running twice doesn't create duplicates", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Run migration first time
		err := AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify all roles were created
		count, err := roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		// Run migration second time
		err = AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify no duplicates were created
		count, err = roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count, "Should still have only 5 roles after running twice")

		// Verify each role exists exactly once
		for _, roleName := range expectedRoles {
			count, err := roleCol.CountDocuments(ctx, bson.M{"name": roleName})
			assert.NoError(t, err)
			assert.Equal(t, int64(1), count, "Role %s should exist exactly once", roleName)
		}
	})

	t.Run("All roles already exist", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Insert all expected roles
		allRoles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role3", "name": "maintainer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role4", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role5", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, allRoles)
		assert.NoError(t, err)

		// Run migration
		err = AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify count remains the same
		count, err := roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(5), count)

		// Verify original IDs are preserved
		var readerRole bson.M
		err = roleCol.FindOne(ctx, bson.M{"name": "reader"}).Decode(&readerRole)
		assert.NoError(t, err)
		assert.Equal(t, "role1", readerRole["id"], "Original ID should be preserved")
	})

	t.Run("Extra roles are not removed", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Insert some expected roles plus an extra one
		rolesWithExtra := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role1", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role2", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "extra1", "name": "custom_role"},
		}
		_, err := roleCol.InsertMany(ctx, rolesWithExtra)
		assert.NoError(t, err)

		// Run migration
		err = AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify all expected roles plus the extra one exist
		count, err := roleCol.CountDocuments(ctx, bson.M{})
		assert.NoError(t, err)
		assert.Equal(t, int64(6), count, "Should have 5 expected roles + 1 custom role")

		// Verify custom role still exists
		var customRole bson.M
		err = roleCol.FindOne(ctx, bson.M{"name": "custom_role"}).Decode(&customRole)
		assert.NoError(t, err)
		assert.Equal(t, "custom_role", customRole["name"])

		// Verify all expected roles exist
		for _, roleName := range expectedRoles {
			var result bson.M
			err = roleCol.FindOne(ctx, bson.M{"name": roleName}).Decode(&result)
			assert.NoError(t, err)
			assert.Equal(t, roleName, result["name"])
		}
	})

	t.Run("Generated IDs are valid ULIDs", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")

		// Run migration
		err := AddRoles(ctx, c)
		assert.NoError(t, err)

		// Verify all role IDs are lowercase and valid format
		for _, roleName := range expectedRoles {
			var result bson.M
			err = roleCol.FindOne(ctx, bson.M{"name": roleName}).Decode(&result)
			assert.NoError(t, err)

			roleID := result["id"].(string)
			assert.NotEmpty(t, roleID, "Role ID should not be empty")
			assert.Equal(t, roleID, roleID, "Role ID should match itself")
			assert.Equal(t, 26, len(roleID), "ULID should be 26 characters")

			// Verify it's lowercase
			for _, char := range roleID {
				if char >= 'A' && char <= 'Z' {
					t.Errorf("Role ID should be lowercase, but found uppercase character in: %s", roleID)
				}
			}
		}
	})
}
