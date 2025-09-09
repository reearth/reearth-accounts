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

func TestAddCaseInsensitiveUserSubsIndex_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to test database
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	err := AddCaseInsensitiveUserSubsIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	// Run the migration to create the index
	err = AddCaseInsensitiveUserSubsIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := db.Collection("user")

	// Insert first user with lowercase sub
	user1 := mongodoc.UserDocument{
		ID:    "user1",
		Name:  "Test User 1",
		Alias: "testuser1",
		Email: "test1@example.com",
		Subs:  []string{"auth0|abc123"},
	}
	
	_, err = col.InsertOne(ctx, user1)
	assert.NoError(t, err, "First user should insert successfully")

	// Try to insert second user with uppercase sub - should fail
	user2 := mongodoc.UserDocument{
		ID:    "user2", 
		Name:  "Test User 2",
		Alias: "testuser2",
		Email: "test2@example.com",
		Subs:  []string{"AUTH0|ABC123"}, // Same as first but uppercase
	}

	_, err = col.InsertOne(ctx, user2)
	assert.Error(t, err, "Second user with case-different sub should fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "Should be duplicate key error")
}