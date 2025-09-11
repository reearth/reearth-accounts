package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

func TestAddCaseInsensitiveUserWorkspaceIndex_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to test database
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	err := AddCaseInsensitiveUserWorkspaceIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := db.Collection("user")

	// Insert first user with lowercase workspace
	user1 := mongodoc.UserDocument{
		ID:        "user1",
		Name:      "Test User 1",
		Alias:     "testuser1",
		Email:     "test1@example.com",
		Workspace: "workspace123",
	}
	
	_, err = col.InsertOne(ctx, user1)
	assert.NoError(t, err, "First user should insert successfully")

	// Try to insert second user with uppercase workspace - should fail
	user2 := mongodoc.UserDocument{
		ID:        "user2", 
		Name:      "Test User 2",
		Alias:     "testuser2",
		Email:     "test2@example.com",
		Workspace: "WORKSPACE123", // Same as first but uppercase
	}

	_, err = col.InsertOne(ctx, user2)
	assert.Error(t, err, "Second user with case-different workspace should fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "Should be duplicate key error")
}
