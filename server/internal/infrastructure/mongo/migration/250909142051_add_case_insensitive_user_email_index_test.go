package migration

import (
	"context"
	"testing"

	"github.com/labstack/gommon/log"
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
	err = client.Disconnect(ctx)
	if err != nil {
		  log.Errorf("failed to disconnect: %v", err)
	}

	testDB := client.Database("test_user_email_migration")
	err = testDB.Drop(ctx)
	if err != nil {
          t.Errorf("failed to drop test database: %v", err)
      }

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