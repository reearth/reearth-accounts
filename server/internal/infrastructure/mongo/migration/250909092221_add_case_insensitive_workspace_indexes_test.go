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

func TestAddCaseInsensitiveWorkspaceIndexes_CaseInsensitiveUniqueness(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	
	// Use proper test database connection
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	// Run the migration to create the index
	err := AddCaseInsensitiveWorkspaceIndexes(ctx, mongoxClient)
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