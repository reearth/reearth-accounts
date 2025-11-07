package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"go.mongodb.org/mongo-driver/bson"
)

func TestSyncPersonalWorkspaceAlias(t *testing.T) {
	ctx := context.Background()
	db := mongotest.Connect(t)(t)

	client := mongox.NewClientWithDatabase(db)
	userCol := client.WithCollection("user")
	workspaceCol := client.WithCollection("workspace")

	// Setup test data
	testUsers := []mongodoc.UserDocument{
		{
			ID:        "user1",
			Name:      "User One",
			Email:     "user1@example.com",
			Alias:     "userone",
			Workspace: "workspace1",
		},
		{
			ID:        "user2",
			Name:      "User Two",
			Email:     "user2@example.com",
			Alias:     "usertwo",
			Workspace: "workspace2",
		},
		{
			ID:        "user3",
			Name:      "User Three",
			Email:     "user3@example.com",
			Alias:     "userthree",
			Workspace: "workspace3",
		},
	}

	testWorkspaces := []mongodoc.WorkspaceDocument{
		{
			ID:       "workspace1",
			Name:     "Workspace One",
			Alias:    "oldaliasone", // Different from user alias
			Personal: true,
		},
		{
			ID:       "workspace2",
			Name:     "Workspace Two",
			Alias:    "usertwo", // Same as user alias - should not change
			Personal: true,
		},
		{
			ID:       "workspace3",
			Name:     "Workspace Three",
			Alias:    "teamworkspace", // Different but not personal
			Personal: false,
		},
		{
			ID:       "workspace4",
			Name:     "Workspace Four",
			Alias:    "orphanworkspace",
			Personal: true, // Personal but no matching user
		},
	}

	// Insert test users
	for _, user := range testUsers {
		_, err := userCol.Client().InsertOne(ctx, user)
		assert.NoError(t, err)
	}

	// Insert test workspaces
	for _, workspace := range testWorkspaces {
		_, err := workspaceCol.Client().InsertOne(ctx, workspace)
		assert.NoError(t, err)
	}

	// Run migration
	err := SyncPersonalWorkspaceAlias(ctx, client)
	assert.NoError(t, err)

	// Verify results
	var results []mongodoc.WorkspaceDocument
	cursor, err := workspaceCol.Client().Find(ctx, bson.M{})
	assert.NoError(t, err)
	err = cursor.All(ctx, &results)
	assert.NoError(t, err)

	for _, result := range results {
		switch result.ID {
		case "workspace1":
			// Should be updated to match user alias
			assert.Equal(t, "userone", result.Alias, "Personal workspace alias should match user alias")
		case "workspace2":
			// Should remain the same (already matched)
			assert.Equal(t, "usertwo", result.Alias, "Already matching alias should not change")
		case "workspace3":
			// Should not be updated (not personal)
			assert.Equal(t, "teamworkspace", result.Alias, "Non-personal workspace should not change")
		case "workspace4":
			// Should remain the same (no matching user)
			assert.Equal(t, "orphanworkspace", result.Alias, "Workspace without user should not change")
		}
	}
}

func TestSyncPersonalWorkspaceAlias_EmptyDatabase(t *testing.T) {
	ctx := context.Background()
	db := mongotest.Connect(t)(t)

	client := mongox.NewClientWithDatabase(db)

	// Run migration on empty database - should not error
	err := SyncPersonalWorkspaceAlias(ctx, client)
	assert.NoError(t, err)
}

func TestSyncPersonalWorkspaceAlias_NoPersonalWorkspaces(t *testing.T) {
	ctx := context.Background()
	db := mongotest.Connect(t)(t)

	client := mongox.NewClientWithDatabase(db)
	userCol := client.WithCollection("user")
	workspaceCol := client.WithCollection("workspace")

	// Setup test data with no personal workspaces
	testUser := mongodoc.UserDocument{
		ID:        "user1",
		Name:      "User One",
		Email:     "user1@example.com",
		Alias:     "userone",
		Workspace: "workspace1",
	}

	testWorkspace := mongodoc.WorkspaceDocument{
		ID:       "workspace1",
		Name:     "Workspace One",
		Alias:    "teamworkspace",
		Personal: false, // Not personal
	}

	_, err := userCol.Client().InsertOne(ctx, testUser)
	assert.NoError(t, err)
	_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
	assert.NoError(t, err)

	// Run migration
	err = SyncPersonalWorkspaceAlias(ctx, client)
	assert.NoError(t, err)

	// Verify workspace alias unchanged
	var result mongodoc.WorkspaceDocument
	err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace1"}).Decode(&result)
	assert.NoError(t, err)
	assert.Equal(t, "teamworkspace", result.Alias, "Non-personal workspace should not change")
}
