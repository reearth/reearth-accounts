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

func TestFixPermittableRoleIDs(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	t.Run("Remove workspace roles from roleids", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert test roles (workspace roles + self)
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_maintainer", "name": "maintainer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_writer", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_reader", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with workspace roles in roleids (incorrect state)
		existingPermittable := bson.M{
			"_id":    primitive.NewObjectID(),
			"id":     "permittable1",
			"userid": "user1",
			"roleids": []string{
				"role_owner",      // should be removed
				"role_maintainer", // should be removed
				"role_self",       // should be kept
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify workspace roles were removed and self remains
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Add self role if missing", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert test roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with only workspace role (no self)
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role_owner"}, // only workspace role, no self
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify self role was added
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Remove all workspace roles and add self", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert all workspace roles and self
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_maintainer", "name": "maintainer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_writer", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_reader", "name": "reader"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with all workspace roles but no self
		existingPermittable := bson.M{
			"_id":    primitive.NewObjectID(),
			"id":     "permittable1",
			"userid": "user1",
			"roleids": []string{
				"role_owner",
				"role_maintainer",
				"role_writer",
				"role_reader",
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify all workspace roles removed and self added
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Keep non-workspace roles", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles including custom role
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_custom", "name": "custom_role"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with workspace role, self, and custom role
		existingPermittable := bson.M{
			"_id":    primitive.NewObjectID(),
			"id":     "permittable1",
			"userid": "user1",
			"roleids": []string{
				"role_owner",  // should be removed
				"role_self",   // should be kept
				"role_custom", // should be kept
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify custom role and self are kept
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 2)

		roleIDStrs := make([]string, len(roleids))
		for i, r := range roleids {
			roleIDStrs[i] = r.(string)
		}
		assert.Contains(t, roleIDStrs, "role_self")
		assert.Contains(t, roleIDStrs, "role_custom")
		assert.NotContains(t, roleIDStrs, "role_owner")
	})

	t.Run("No change when already correct", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with correct roleids (only self)
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role_self"}, // already correct
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify roleids unchanged
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Handle empty roleids", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with empty roleids
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{}, // empty
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify self role was added
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Handle multiple permittables", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_writer", "name": "writer"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert multiple permittables with different states
		permittables := []interface{}{
			bson.M{
				"_id":     primitive.NewObjectID(),
				"id":      "permittable1",
				"userid":  "user1",
				"roleids": []string{"role_owner", "role_self"}, // owner should be removed
			},
			bson.M{
				"_id":     primitive.NewObjectID(),
				"id":      "permittable2",
				"userid":  "user2",
				"roleids": []string{"role_writer"}, // writer removed, self added
			},
			bson.M{
				"_id":     primitive.NewObjectID(),
				"id":      "permittable3",
				"userid":  "user3",
				"roleids": []string{"role_self"}, // already correct
			},
		}
		_, err = permittableCol.InsertMany(ctx, permittables)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify user1
		var result1 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result1)
		assert.NoError(t, err)
		roleids1, _ := result1["roleids"].(primitive.A)
		assert.Len(t, roleids1, 1)
		assert.Equal(t, "role_self", roleids1[0])

		// Verify user2
		var result2 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user2"}).Decode(&result2)
		assert.NoError(t, err)
		roleids2, _ := result2["roleids"].(primitive.A)
		assert.Len(t, roleids2, 1)
		assert.Equal(t, "role_self", roleids2[0])

		// Verify user3
		var result3 bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user3"}).Decode(&result3)
		assert.NoError(t, err)
		roleids3, _ := result3["roleids"].(primitive.A)
		assert.Len(t, roleids3, 1)
		assert.Equal(t, "role_self", roleids3[0])
	})

	t.Run("Idempotent - running twice produces same result", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with workspace role
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role_owner", "role_self"},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration first time
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Run migration second time
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify result is correct
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])
	})

	t.Run("Handle missing self role in roles collection", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles WITHOUT self role
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_writer", "name": "writer"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with workspace roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role_owner", "role_writer"},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration - should not error
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify workspace roles were removed (self not added since it doesn't exist)
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 0)
	})

	t.Run("Preserve workspace_roles field", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		c := mongox.NewClientWithDatabase(db)

		roleCol := db.Collection("role")
		permittableCol := db.Collection("permittable")

		// Insert roles
		roles := []interface{}{
			bson.M{"_id": primitive.NewObjectID(), "id": "role_owner", "name": "owner"},
			bson.M{"_id": primitive.NewObjectID(), "id": "role_self", "name": "self"},
		}
		_, err := roleCol.InsertMany(ctx, roles)
		assert.NoError(t, err)

		// Insert permittable with workspace_roles
		existingPermittable := bson.M{
			"_id":     primitive.NewObjectID(),
			"id":      "permittable1",
			"userid":  "user1",
			"roleids": []string{"role_owner"},
			"workspace_roles": []bson.M{
				{"workspace_id": "ws1", "role_id": "role_owner"},
			},
		}
		_, err = permittableCol.InsertOne(ctx, existingPermittable)
		assert.NoError(t, err)

		// Run migration
		err = FixPermittableRoleIDs(ctx, c)
		assert.NoError(t, err)

		// Verify workspace_roles preserved
		var result bson.M
		err = permittableCol.FindOne(ctx, bson.M{"userid": "user1"}).Decode(&result)
		assert.NoError(t, err)

		// Check roleids fixed
		roleids, ok := result["roleids"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, roleids, 1)
		assert.Equal(t, "role_self", roleids[0])

		// Check workspace_roles preserved
		workspaceRoles, ok := result["workspace_roles"].(primitive.A)
		assert.True(t, ok)
		assert.Len(t, workspaceRoles, 1)

		wsRole := workspaceRoles[0].(bson.M)
		assert.Equal(t, "ws1", wsRole["workspace_id"])
		assert.Equal(t, "role_owner", wsRole["role_id"])
	})
}
