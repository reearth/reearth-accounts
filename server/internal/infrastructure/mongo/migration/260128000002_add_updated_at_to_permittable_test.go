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

func TestApplyPermittableUpdatedAtSchema(t *testing.T) {

	t.Skip("skipped by request")

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	permittableCollection := db.Collection("permittable")

	// Create test permittables with ObjectId for timestamp extraction
	permittable1ID := primitive.NewObjectID()
	permittable2ID := primitive.NewObjectID()

	testPermittables := []any{
		bson.M{
			"_id":    permittable1ID,
			"id":     "permittable1",
			"userid": "user1",
			"roleids": []string{},
			// No updatedat field
		},
		bson.M{
			"_id":    permittable2ID,
			"id":     "permittable2",
			"userid": "user2",
			"roleids": []string{},
			// No updatedat field
		},
		bson.M{
			"_id":       primitive.NewObjectID(),
			"id":        "permittable3",
			"userid":    "user3",
			"roleids":   []string{},
			"updatedat": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			// Already has updatedat
		},
	}

	_, err := permittableCollection.InsertMany(ctx, testPermittables)
	assert.NoError(t, err)

	// Run the migration
	err = ApplyPermittableUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	// Test case 1: permittable1 should have updatedat from ObjectId timestamp
	var permittable1 bson.M
	err = permittableCollection.FindOne(ctx, bson.M{"id": "permittable1"}).Decode(&permittable1)
	assert.NoError(t, err)
	assert.NotNil(t, permittable1["updatedat"])
	updatedAt1, ok := permittable1["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, permittable1ID.Timestamp().UTC(), updatedAt1.Time().UTC())

	// Test case 2: permittable2 should have updatedat from ObjectId timestamp
	var permittable2 bson.M
	err = permittableCollection.FindOne(ctx, bson.M{"id": "permittable2"}).Decode(&permittable2)
	assert.NoError(t, err)
	assert.NotNil(t, permittable2["updatedat"])
	updatedAt2, ok := permittable2["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, permittable2ID.Timestamp().UTC(), updatedAt2.Time().UTC())

	// Test case 3: permittable3 already had updatedat, should remain unchanged
	var permittable3 bson.M
	err = permittableCollection.FindOne(ctx, bson.M{"id": "permittable3"}).Decode(&permittable3)
	assert.NoError(t, err)
	assert.NotNil(t, permittable3["updatedat"])
	updatedAt3, ok := permittable3["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime.UTC(), updatedAt3.Time().UTC())

	// Test case 4: Verify migration is idempotent
	err = ApplyPermittableUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	var permittable1After bson.M
	err = permittableCollection.FindOne(ctx, bson.M{"id": "permittable1"}).Decode(&permittable1After)
	assert.NoError(t, err)
	updatedAt1After, ok := permittable1After["updatedat"].(primitive.DateTime)
	assert.True(t, ok)
	assert.Equal(t, updatedAt1.Time().UTC(), updatedAt1After.Time().UTC(), "updatedat should not change on second run")
}
