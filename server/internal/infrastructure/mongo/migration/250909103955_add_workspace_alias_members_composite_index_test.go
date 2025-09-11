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

func TestAddWorkspaceAliasMembersCompositeUniqueIndex(t *testing.T) {
	if testing.Short() {
		t.Skip("skipping integration test")
	}

	ctx := context.Background()
	
	// Use proper test database connection
	db := mongo.Connect(t)(t)
	mongoxClient := mongox.NewClientWithDatabase(db)

	// Run the migration to create the composite index
	err := AddWorkspaceAliasMembersCompositeUniqueIndex(ctx, mongoxClient)
	assert.NoError(t, err)

	col := db.Collection("workspace")

	// Create sample members map
	members1 := map[string]mongodoc.WorkspaceMemberDocument{
		"user1": {Role: "owner", InvitedBy: "user1", Disabled: false},
		"user2": {Role: "maintainer", InvitedBy: "user1", Disabled: false},
	}

	members2 := map[string]mongodoc.WorkspaceMemberDocument{
		"user3": {Role: "owner", InvitedBy: "user3", Disabled: false},
		"user4": {Role: "maintainer", InvitedBy: "user3", Disabled: false},
	}

	// Insert first workspace with lowercase alias and specific members
	workspace1 := mongodoc.WorkspaceDocument{
		ID:      "workspace1",
		Name:    "Test Workspace 1",
		Alias:   "myworkspace",
		Email:   "test1@example.com",
		Members: members1,
	}
	
	_, err = col.InsertOne(ctx, workspace1)
	assert.NoError(t, err, "First workspace should insert successfully")

	// Try to insert workspace with same alias (different case) but different members - should succeed
	workspace2 := mongodoc.WorkspaceDocument{
		ID:      "workspace2",
		Name:    "Test Workspace 2", 
		Alias:   "MYWORKSPACE", // Same alias but different case
		Email:   "test2@example.com",
		Members: members2, // Different members
	}

	_, err = col.InsertOne(ctx, workspace2)
	assert.NoError(t, err, "Workspace with same alias but different members should succeed")

	// Try to insert workspace with exactly same alias and same members - should fail
	workspace3 := mongodoc.WorkspaceDocument{
		ID:      "workspace3",
		Name:    "Test Workspace 3",
		Alias:   "myworkspace", // Same alias as workspace1
		Email:   "test3@example.com", 
		Members: members1, // Same members as workspace1
	}

	_, err = col.InsertOne(ctx, workspace3)
	assert.Error(t, err, "Workspace with same alias and same members should fail")
	assert.True(t, mongodriver.IsDuplicateKeyError(err), "Should be duplicate key error for composite index")
}
