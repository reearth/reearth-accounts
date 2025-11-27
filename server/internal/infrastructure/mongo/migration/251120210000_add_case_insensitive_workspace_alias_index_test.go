package migration

import (
	"context"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
)

func TestAddCaseInsensitiveWorkspaceIndexes_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()

	// Use proper test database connection
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	// Run the migration to create the index
	err := AddCaseInsensitiveWorkspaceAliasIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := db.Collection("workspace")

	// Insert first workspace with lowercase alias
	workspace1 := mongodoc.WorkspaceDocument{
		ID:    "workspace1",
		Name:  "Test Workspace 1",
		Alias: "myworkspace",
		Email: "test1@example.com",
	}

	_, err = col.InsertOne(ctx, workspace1)
	assert.NoError(t, err, "First workspace should insert successfully")

	// Try to insert second workspace with uppercase alias - should fail
	workspace2 := mongodoc.WorkspaceDocument{
		ID:    "workspace2",
		Name:  "Test Workspace 2",
		Alias: "MYWORKSPACE", // Same as first but uppercase
		Email: "test2@example.com",
	}

	_, err = col.InsertOne(ctx, workspace2)
	assert.Error(t, err, "Second workspace with case-different alias should fail")

	// Verify it's a duplicate key error
	if mongodriver.IsDuplicateKeyError(err) {
		t.Logf("Correctly got duplicate key error: %v", err)
	} else {
		t.Errorf("Expected duplicate key error, got: %v", err)
	}

	// Try with mixed case - should also fail
	workspace3 := mongodoc.WorkspaceDocument{
		ID:    "workspace3",
		Name:  "Test Workspace 3",
		Alias: "MyWorkSpace", // Mixed case version
		Email: "test3@example.com",
	}

	_, err = col.InsertOne(ctx, workspace3)
	assert.Error(t, err, "Third workspace with mixed case alias should also fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "Should be duplicate key error")
}

func TestAddCaseInsensitiveWorkspaceIndexes_DuplicateHandling(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)
	col := db.Collection("workspace")

	// Insert workspaces with case-insensitive duplicate aliases before migration
	workspaces := []mongodoc.WorkspaceDocument{
		{
			ID:    "workspace1",
			Name:  "First Workspace",
			Alias: "testworkspace",
			Email: "test1@example.com",
		},
		{
			ID:    "workspace2",
			Name:  "Second Workspace",
			Alias: "TESTWORKSPACE",
			Email: "test2@example.com",
		},
		{
			ID:    "workspace3",
			Name:  "Third Workspace",
			Alias: "TestWorkspace",
			Email: "test3@example.com",
		},
	}

	for _, ws := range workspaces {
		_, err := col.InsertOne(ctx, ws)
		assert.NoError(t, err)
	}

	// Run migration - should handle duplicates and create index
	err := AddCaseInsensitiveWorkspaceAliasIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	// Verify results
	var results []mongodoc.WorkspaceDocument
	cursor, err := col.Find(ctx, bson.M{})
	assert.NoError(t, err)
	err = cursor.All(ctx, &results)
	assert.NoError(t, err)
	assert.Len(t, results, 3)

	// Check aliases are unique case-insensitively
	aliasMap := make(map[string]string)
	for _, ws := range results {
		lowerAlias := strings.ToLower(ws.Alias)
		if existingID, exists := aliasMap[lowerAlias]; exists {
			t.Errorf("Duplicate case-insensitive alias '%s' for workspaces %s and %s",
				lowerAlias, existingID, ws.ID)
		}
		aliasMap[lowerAlias] = ws.ID
	}

	// First workspace should keep original alias
	var firstWorkspace mongodoc.WorkspaceDocument
	// Find by alias instead of _id, since _id may be ObjectID
	err = col.FindOne(ctx, bson.M{"alias": "testworkspace"}).Decode(&firstWorkspace)
	assert.NoError(t, err)
	assert.Equal(t, "testworkspace", firstWorkspace.Alias)

	// Other workspaces should have new aliases with suffix pattern
	cursor, err = col.Find(ctx, bson.M{"alias": bson.M{"$regex": "^testworkspace-"}})
	assert.NoError(t, err)
	var updatedWorkspaces []mongodoc.WorkspaceDocument
	err = cursor.All(ctx, &updatedWorkspaces)
	assert.NoError(t, err)
	assert.Len(t, updatedWorkspaces, 2)
	for _, ws := range updatedWorkspaces {
		assert.True(t, strings.HasPrefix(ws.Alias, "testworkspace-"))
	}

	// Index should prevent new duplicates
	newWorkspace := mongodoc.WorkspaceDocument{
		ID:    "workspace4",
		Name:  "Fourth Workspace",
		Alias: "TESTWORKSPACE",
		Email: "test4@example.com",
	}

	_, err = col.InsertOne(ctx, newWorkspace)
	assert.Error(t, err)
	assert.True(t, mongodriver.IsDuplicateKeyError(err))
}
