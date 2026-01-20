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

func TestApplyWorkspaceUpdatedAtSchema(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	workspaceCollection := db.Collection("workspace")

	// Create test workspaces with ObjectId for timestamp extraction
	workspace1ID := primitive.NewObjectID()
	workspace2ID := primitive.NewObjectID()

	testWorkspaces := []interface{}{
		bson.M{
			"_id":      workspace1ID,
			"id":       "workspace1",
			"name":     "Workspace One",
			"email":    "workspace1@example.com",
			"alias":    "workspace1",
			"members":  bson.M{},
			"metadata": bson.M{},
			"personal": false,
			// No updatedat field
		},
		bson.M{
			"_id":      workspace2ID,
			"id":       "workspace2",
			"name":     "Workspace Two",
			"email":    "workspace2@example.com",
			"alias":    "workspace2",
			"members":  bson.M{},
			"metadata": bson.M{},
			"personal": false,
			// No updatedat field
		},
		bson.M{
			"_id":       primitive.NewObjectID(),
			"id":        "workspace3",
			"name":      "Workspace Three",
			"email":     "workspace3@example.com",
			"alias":     "workspace3",
			"members":   bson.M{},
			"metadata":  bson.M{},
			"personal":  true,
			"updatedat": time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC),
			// Already has updatedat
		},
	}

	_, err := workspaceCollection.InsertMany(ctx, testWorkspaces)
	assert.NoError(t, err)

	// Run the migration
	err = ApplyWorkspaceUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	// Test case 1: workspace1 should have updatedat from ObjectId timestamp
	var workspace1 bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "workspace1"}).Decode(&workspace1)
	assert.NoError(t, err)
	assert.NotNil(t, workspace1["updatedat"])
	updatedAt1, ok := workspace1["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, workspace1ID.Timestamp().UTC(), updatedAt1.Time().UTC())

	// Test case 2: workspace2 should have updatedat from ObjectId timestamp
	var workspace2 bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "workspace2"}).Decode(&workspace2)
	assert.NoError(t, err)
	assert.NotNil(t, workspace2["updatedat"])
	updatedAt2, ok := workspace2["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	assert.Equal(t, workspace2ID.Timestamp().UTC(), updatedAt2.Time().UTC())

	// Test case 3: workspace3 already had updatedat, should remain unchanged
	var workspace3 bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "workspace3"}).Decode(&workspace3)
	assert.NoError(t, err)
	assert.NotNil(t, workspace3["updatedat"])
	updatedAt3, ok := workspace3["updatedat"].(primitive.DateTime)
	assert.True(t, ok, "updatedat should be a DateTime")
	expectedTime := time.Date(2023, 1, 1, 0, 0, 0, 0, time.UTC)
	assert.Equal(t, expectedTime.UTC(), updatedAt3.Time().UTC())

	// Test case 4: Verify migration is idempotent
	err = ApplyWorkspaceUpdatedAtSchema(ctx, mongoxClient)
	assert.NoError(t, err)

	var workspace1After bson.M
	err = workspaceCollection.FindOne(ctx, bson.M{"id": "workspace1"}).Decode(&workspace1After)
	assert.NoError(t, err)
	updatedAt1After, ok := workspace1After["updatedat"].(primitive.DateTime)
	assert.True(t, ok)
	assert.Equal(t, updatedAt1.Time().UTC(), updatedAt1After.Time().UTC(), "updatedat should not change on second run")
}
