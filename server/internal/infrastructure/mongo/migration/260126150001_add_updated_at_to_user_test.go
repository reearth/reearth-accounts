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

func TestApplyUserUpdatedAtSchema(t *testing.T) {

	t.Skip("skipped by request")

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	userCollection := db.Collection("user")

	// Create test users with ObjectId for timestamp extraction
	user1ID := primitive.NewObjectID()
	user2ID := primitive.NewObjectID()

	testUsers := []interface{}{
		bson.M{
			"_id":       user1ID,
			"id":        "user1",
			"name":      "User One",
			"email":     "user1@example.com",
			"alias":     "user1",
			"workspace": "workspace1",
			"subs":      []string{},
			"password":  []byte("password1"),
			"metadata":  bson.M{},
			// No updatedat field
		},
		bson.M{
			"_id":       user2ID,
			"id":        "user2",
			"name":      "User Two",
			"email":     "user2@example.com",
			"alias":     "user2",
			"workspace": "workspace2",
			"subs":      []string{},
			"password":  []byte("password2"),
			"metadata":  bson.M{},
			// No updatedat field
		},
		bson.M{
			"_id":       primitive.NewObjectID(),
			"id":        "user3",
			"name":      "User Three",
			"email":     "user3@example.com",
			"alias":     "user3",
			"workspace": "workspace3",
			"subs":      []string{},
			"password":  []byte("password3"),
			"metadata":  bson.M{},
			"updatedat": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			// Already has updatedat
		},
	}

	_, err := userCollection.InsertMany(ctx, testUsers)
	assert.NoError(t, err)

	// Run the migration
	err = ApplyUserUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	// Test case 1: user1 should have updatedat from ObjectId timestamp
	var user1 bson.M
	err = userCollection.FindOne(ctx, bson.M{"id": "user1"}).Decode(&user1)
	assert.NoError(t, err)
	assert.NotNil(t, user1["updatedat"])
	updatedAt1, ok := user1["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, user1ID.Timestamp().UTC(), updatedAt1.Time().UTC())

	// Test case 2: user2 should have updatedat from ObjectId timestamp
	var user2 bson.M
	err = userCollection.FindOne(ctx, bson.M{"id": "user2"}).Decode(&user2)
	assert.NoError(t, err)
	assert.NotNil(t, user2["updatedat"])
	updatedAt2, ok := user2["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, user2ID.Timestamp().UTC(), updatedAt2.Time().UTC())

	// Test case 3: user3 already had updatedat, should remain unchanged
	var user3 bson.M
	err = userCollection.FindOne(ctx, bson.M{"id": "user3"}).Decode(&user3)
	assert.NoError(t, err)
	assert.NotNil(t, user3["updatedat"])
	updatedAt3, ok := user3["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime.UTC(), updatedAt3.Time().UTC())

	// Test case 4: Verify migration is idempotent
	err = ApplyUserUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	var user1After bson.M
	err = userCollection.FindOne(ctx, bson.M{"id": "user1"}).Decode(&user1After)
	assert.NoError(t, err)
	updatedAt1After, ok := user1After["updatedat"].(primitive.DateTime)
	assert.True(t, ok)
	assert.Equal(t, updatedAt1.Time().UTC(), updatedAt1After.Time().UTC(), "updatedat should not change on second run")
}
