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

func TestConvertInvalidWorkspaceAlias(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	workspaceCollection := db.Collection("workspace")

	testWorkspaces := []interface{}{
		bson.M{
			"_id":   "workspace1",
			"id":    "workspace1",
			"name":  "Test Workspace One",
			"alias": "invalid_name",
		},
		bson.M{
			"_id":   "workspace2",
			"id":    "workspace2",
			"name":  "Test Workspace Two",
			"alias": "invalid@name",
		},
		bson.M{
			"_id":   "workspace3",
			"id":    "workspace3",
			"name":  "Test Workspace Three",
			"alias": "invalid--name",
		},
		bson.M{
			"_id":   "workspace4",
			"id":    "workspace4",
			"name":  "Test Workspace Four",
			"alias": "-invalid",
		},
		bson.M{
			"_id":   "workspace5",
			"id":    "workspace5",
			"name":  "Test Workspace Five",
			"alias": "invalid-",
		},
		bson.M{
			"_id":   "workspace6",
			"id":    "workspace6",
			"name":  "Test Workspace Six",
			"alias": "",
		},
		bson.M{
			"_id":   "workspace7",
			"id":    "workspace7",
			"name":  "Test Workspace Seven",
			"alias": "ab",
		},
		bson.M{
			"_id":   "workspace8",
			"id":    "workspace8",
			"name":  "Test Workspace Eight",
			"alias": "validname",
		},
		bson.M{
			"_id":   "workspace9",
			"id":    "workspace9",
			"name":  "Test Workspace Nine",
			"alias": "Valid-Name",
		},
	}

	_, err := workspaceCollection.InsertMany(ctx, testWorkspaces)
	assert.NoError(t, err)

	// Run the migration
	err = ConvertInvalidUserWorkspaceAlias(ctx, mongoxClient)
	assert.NoError(t, err)

	// Check that invalid aliases were sanitized
	nameRegex := regexp.MustCompile(`^[a-zA-Z0-9][a-zA-Z0-9-]{3,30}[a-zA-Z0-9]$`)

	invalidAliasTests := []struct {
		workspaceID       string
		originalAlias     string
		expectedSanitized string
	}{
		{"workspace1", "invalid_name", "invalid-name"},
		{"workspace2", "invalid@name", "invalid-name"},
		{"workspace3", "invalid--name", "invalid-name"},
		{"workspace4", "-invalid", "invalid"},
		{"workspace5", "invalid-", "invalid"},
		{"workspace6", "", "aaaaa"},
		{"workspace7", "ab", "abaaa"},
	}

	for _, test := range invalidAliasTests {
		var result bson.M
		fErr := workspaceCollection.FindOne(ctx, bson.M{"id": test.workspaceID}).Decode(&result)
		assert.NoError(t, fErr)

		alias, exists := result["alias"]
		assert.True(t, exists, "Alias should exist for workspace %s", test.workspaceID)
		aliasStr := alias.(string)
		assert.NotEmpty(t, aliasStr, "Alias should not be empty for workspace %s", test.workspaceID)
		assert.True(t, nameRegex.MatchString(aliasStr), "Alias should match regex for workspace %s: %s", test.workspaceID, aliasStr)
		assert.Equal(t, test.expectedSanitized, aliasStr, "Alias should be sanitized correctly for workspace %s", test.workspaceID)
	}

	// Check that valid aliases remained unchanged
	validAliasTests := []struct {
		workspaceID string
		alias       string
	}{
		{"workspace8", "validname"},
		{"workspace9", "Valid-Name"},
	}

	for _, test := range validAliasTests {
		var result bson.M
		fErr := workspaceCollection.FindOne(ctx, bson.M{"id": test.workspaceID}).Decode(&result)
		assert.NoError(t, fErr)
		assert.Equal(t, test.alias, result["alias"], "Valid alias should not be changed for workspace %s", test.workspaceID)
	}
}
