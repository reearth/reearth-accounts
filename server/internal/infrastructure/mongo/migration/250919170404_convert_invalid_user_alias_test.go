package migration

import (
	"context"
	"regexp"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestConvertInvalidUserAlias(t *testing.T) {
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
			"name":  "Test User One",
			"alias": "invalid_name",
			"email": "test1@example.com",
		},
		bson.M{
			"_id":   "user2",
			"id":    "user2",
			"name":  "Test User Two",
			"alias": "invalid@name",
			"email": "test2@example.com",
		},
		bson.M{
			"_id":   "user3",
			"id":    "user3",
			"name":  "Test User Three",
			"alias": "invalid--name",
			"email": "test3@example.com",
		},
		bson.M{
			"_id":   "user4",
			"id":    "user4",
			"name":  "Test User Four",
			"alias": "-invalid",
			"email": "test4@example.com",
		},
		bson.M{
			"_id":   "user5",
			"id":    "user5",
			"name":  "Test User Five",
			"alias": "invalid-",
			"email": "test5@example.com",
		},
		bson.M{
			"_id":   "user6",
			"id":    "user6",
			"name":  "Test User Six",
			"alias": "",
			"email": "test6@example.com",
		},
		bson.M{
			"_id":   "user7",
			"id":    "user7",
			"name":  "Test User Seven",
			"alias": "ab",
			"email": "test7@example.com",
		},
		bson.M{
			"_id":   "user8",
			"id":    "user8",
			"name":  "Test User Eight",
			"alias": "validname",
			"email": "test8@example.com",
		},
		bson.M{
			"_id":   "user9",
			"id":    "user9",
			"name":  "Test User Nine",
			"alias": "Valid-Name",
			"email": "test9@example.com",
		},
	}

	_, err := userCollection.InsertMany(ctx, testUsers)
	assert.NoError(t, err)

	// Run the migration
	err = ConvertInvalidUserAlias(ctx, mongoxClient)
	assert.NoError(t, err)

	// Check that invalid aliases were sanitized
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{3,30}[a-zA-Z0-9]$`)

	invalidAliasTests := []struct {
		userID            string
		originalAlias     string
		expectedSanitized string
	}{
		{"user1", "invalid_name", "invalid-name"},
		{"user2", "invalid@name", "invalid-name"},
		{"user3", "invalid--name", "invalid-name"},
		{"user4", "-invalid", "invalid"},
		{"user5", "invalid-", "invalid"},
		{"user6", "", "aaaaa"},
		{"user7", "ab", "abaaa"},
	}

	for _, test := range invalidAliasTests {
		var result bson.M
		fErr := userCollection.FindOne(ctx, bson.M{"id": test.userID}).Decode(&result)
		assert.NoError(t, fErr)

		alias, exists := result["alias"]
		assert.True(t, exists, "Alias should exist for user %s", test.userID)
		aliasStr := alias.(string)
		assert.NotEmpty(t, aliasStr, "Alias should not be empty for user %s", test.userID)
		assert.True(t, nameRegex.MatchString(aliasStr), "Alias should match regex for user %s: %s", test.userID, aliasStr)
		assert.Equal(t, test.expectedSanitized, aliasStr, "Alias should be sanitized correctly for user %s", test.userID)
	}

	// Check that valid aliases remained unchanged
	validAliasTests := []struct {
		userID string
		alias  string
	}{
		{"user8", "validname"},
		{"user9", "Valid-Name"},
	}

	for _, test := range validAliasTests {
		var result bson.M
		err := userCollection.FindOne(ctx, bson.M{"id": test.userID}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, test.alias, result["alias"], "Valid alias should not be changed for user %s", test.userID)
	}
}
