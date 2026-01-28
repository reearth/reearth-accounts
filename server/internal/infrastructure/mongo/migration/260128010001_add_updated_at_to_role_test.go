package migration

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/bson/primitive"
)

func TestApplyRoleUpdatedAtSchema(t *testing.T) {

	t.Skip("skipped by request")

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	roleCollection := db.Collection("role")

	// Create test roles with ObjectId for timestamp extraction
	role1ID := primitive.NewObjectID()
	role2ID := primitive.NewObjectID()

	testRoles := []interface{}{
		bson.M{
			"_id":  role1ID,
			"id":   "role1",
			"name": "Role One",
			// No updatedat field
		},
		bson.M{
			"_id":  role2ID,
			"id":   "role2",
			"name": "Role Two",
			// No updatedat field
		},
		bson.M{
			"_id":       primitive.NewObjectID(),
			"id":        "role3",
			"name":      "Role Three",
			"updatedat": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			// Already has updatedat
		},
	}

	_, err := roleCollection.InsertMany(ctx, testRoles)
	assert.NoError(t, err)

	// Run the migration
	err = ApplyRoleUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	// Test case 1: role1 should have updatedat from ObjectId timestamp
	var role1 bson.M
	err = roleCollection.FindOne(ctx, bson.M{"id": "role1"}).Decode(&role1)
	assert.NoError(t, err)
	assert.NotNil(t, role1["updatedat"])
	updatedAt1, ok := role1["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, role1ID.Timestamp().UTC(), updatedAt1.Time().UTC())

	// Test case 2: role2 should have updatedat from ObjectId timestamp
	var role2 bson.M
	err = roleCollection.FindOne(ctx, bson.M{"id": "role2"}).Decode(&role2)
	assert.NoError(t, err)
	assert.NotNil(t, role2["updatedat"])
	updatedAt2, ok := role2["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, role2ID.Timestamp().UTC(), updatedAt2.Time().UTC())

	// Test case 3: role3 already had updatedat, should remain unchanged
	var role3 bson.M
	err = roleCollection.FindOne(ctx, bson.M{"id": "role3"}).Decode(&role3)
	assert.NoError(t, err)
	assert.NotNil(t, role3["updatedat"])
	updatedAt3, ok := role3["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime.UTC(), updatedAt3.Time().UTC())

	// Test case 4: Verify migration is idempotent
	err = ApplyRoleUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	var role1After bson.M
	err = roleCollection.FindOne(ctx, bson.M{"id": "role1"}).Decode(&role1After)
	assert.NoError(t, err)
	updatedAt1After, ok := role1After["updatedat"].(primitive.DateTime)
	assert.True(t, ok)
	assert.Equal(t, updatedAt1.Time().UTC(), updatedAt1After.Time().UTC(), "updatedat should not change on second run")
}
