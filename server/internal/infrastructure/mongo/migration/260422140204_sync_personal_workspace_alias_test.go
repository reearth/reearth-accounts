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

func init() {
	mongotest.Env = "REEARTH_DB"
}

func TestSyncPersonalWorkspaceAlias(t *testing.T) {
	t.Run("PersonalWorkspaceWithDifferentAlias", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User with alias "userone" and personal workspace with different alias
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
			Alias:    "oldaliasone",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)

		// Verify workspace alias was updated to match user alias
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace1"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "userone", result.Alias)
	})

	t.Run("PersonalWorkspaceWithMatchingAlias", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User and personal workspace with matching alias
		testUser := mongodoc.UserDocument{
			ID:        "user2",
			Name:      "User Two",
			Email:     "user2@example.com",
			Alias:     "usertwo",
			Workspace: "workspace2",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace2",
			Name:     "Workspace Two",
			Alias:    "usertwo",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)

		// Verify workspace alias remains unchanged
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace2"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "usertwo", result.Alias)
	})

	t.Run("NonPersonalWorkspace", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User and non-personal workspace
		testUser := mongodoc.UserDocument{
			ID:        "user3",
			Name:      "User Three",
			Email:     "user3@example.com",
			Alias:     "userthree",
			Workspace: "workspace3",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace3",
			Name:     "Workspace Three",
			Alias:    "teamworkspace",
			Personal: false,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)

		// Verify workspace alias remains unchanged (not personal)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace3"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "teamworkspace", result.Alias)
	})

	t.Run("PersonalWorkspaceWithoutUser", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		workspaceCol := client.WithCollection("workspace")

		// Setup: Personal workspace without matching user
		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace4",
			Name:     "Workspace Four",
			Alias:    "orphanworkspace",
			Personal: true,
		}

		_, err := workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)

		// Verify workspace alias remains unchanged (no matching user)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace4"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "orphanworkspace", result.Alias)
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)

		// Run migration on empty database - should not error
		err := SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)
	})
}
