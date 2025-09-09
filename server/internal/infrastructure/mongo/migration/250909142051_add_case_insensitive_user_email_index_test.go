package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/mongo"
)

func TestAddCaseInsensitiveUserEmailIndex_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	
	// Connect to test database
	client, err := mongo.Connect(ctx, nil) 
	if err != nil {
		t.Skipf("Could not connect to MongoDB: %v", err)
	}
	defer client.Disconnect(ctx)

	testDB := client.Database("test_user_email_migration")
	defer testDB.Drop(ctx)

	mongoxClient := mongox.NewClientWithDatabase(testDB)

	// Run the migration to create the index
	err = AddCaseInsensitiveUserEmailIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := testDB.Collection("user")

	// Insert first user with lowercase email
	user1 := mongodoc.UserDocument{
		ID:    "user1",
		Name:  "Test User 1",
		Alias: "testuser1",
		Email: "test@example.com",
	}
	
	_, err = col.InsertOne(ctx, user1)
	assert.NoError(t, err, "First user should insert successfully")

	// Try to insert second user with uppercase email - should fail
	user2 := mongodoc.UserDocument{
		ID:    "user2", 
		Name:  "Test User 2",
		Alias: "testuser2",
		Email: "TEST@EXAMPLE.COM", // Same as first but uppercase
	}

	_, err = col.InsertOne(ctx, user2)
	assert.Error(t, err, "Second user with case-different email should fail")
	assert.True(t, mongo.IsDuplicateKeyError(err), "Should be duplicate key error")
}