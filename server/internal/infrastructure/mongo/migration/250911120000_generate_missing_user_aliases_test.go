package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/internal/infrastructure/mongo"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestGenerateMissingUserAliases(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	userCollection := db.Collection("user")
	
	testUsers := []interface{}{
		bson.M{
			"_id":   "user1",
			"id":    "user1",
			"name":  "User One",
			"email": "user1@example.com",
			"alias": "", 
		},
		bson.M{
			"_id":   "user2", 
			"id":    "user2",
			"name":  "User Two",
			"email": "user2@example.com",
			"alias": "", 
		},
		bson.M{
			"_id":   "user3",
			"id":    "user3", 
			"name":  "User Three",
			"email": "user3@example.com",
			"alias": "validalias", 
		},
	}

	_, err := userCollection.InsertMany(ctx, testUsers)
	assert.NoError(t, err)

	// Run the migration
	err = GenerateMissingUserAliases(ctx, mongoxClient)
	assert.NoError(t, err)

	// Check that user1 and user2 got new aliases
	updatedUsers := []string{"user1", "user2"}
	for _, id := range updatedUsers {
		var result bson.M
		err := userCollection.FindOne(ctx, bson.M{"id": id}).Decode(&result)
		assert.NoError(t, err)
		
		alias, exists := result["alias"]
		assert.True(t, exists, "Alias should exist for user %s", id)
		aliasStr := alias.(string)
		assert.NotEmpty(t, aliasStr, "Alias should not be empty for user %s", id)
		assert.Len(t, aliasStr, 10, "Generated alias should be 10 characters for user %s", id)
	}

	// Check that user3 kept its original alias
	var result bson.M
	err = userCollection.FindOne(ctx, bson.M{"id": "user3"}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "validalias", result["alias"], "Valid alias should not be changed")
}