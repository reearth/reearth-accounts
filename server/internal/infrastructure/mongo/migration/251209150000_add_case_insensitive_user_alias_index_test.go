package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

func TestAddCaseInsensitiveUserAliasIndex_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Use proper test database connection
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	// Run the migration to create the index
	err := AddCaseInsensitiveUserAliasIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := db.Collection("user")

	// Insert first user with lowercase alias
	user1 := mongodoc.UserDocument{
		ID:    "user1",
		Name:  "Test User 1",
		Alias: "testuser",
		Email: "test1@example.com",
	}

	_, err = col.InsertOne(ctx, user1)
	assert.NoError(t, err, "First user should insert successfully")

	// Try to insert second user with uppercase alias - should fail
	user2 := mongodoc.UserDocument{
		ID:    "user2",
		Name:  "Test User 2",
		Alias: "TESTUSER", // Same as first but uppercase
		Email: "test2@example.com",
	}

	_, err = col.InsertOne(ctx, user2)
	assert.Error(t, err, "Second user with case-different alias should fail")

	// Verify it's a duplicate key error
	if mongodriver.IsDuplicateKeyError(err) {
		t.Logf("Correctly got duplicate key error: %v", err)
	} else {
		t.Errorf("Expected duplicate key error, got: %v", err)
	}

	// Try with mixed case - should also fail
	user3 := mongodoc.UserDocument{
		ID:    "user3",
		Name:  "Test User 3",
		Alias: "TestUser", // Mixed case version
		Email: "test3@example.com",
	}

	_, err = col.InsertOne(ctx, user3)
	assert.Error(t, err, "Third user with mixed case alias should also fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "Should be duplicate key error")
}
