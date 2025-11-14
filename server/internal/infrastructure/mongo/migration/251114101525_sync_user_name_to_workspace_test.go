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

func TestSyncUserNameToWorkspace(t *testing.T) {
	t.Run("PersonalWorkspaceWithEmailFormatName", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User with name and personal workspace with email-formatted name
		testUser := mongodoc.UserDocument{
			ID:        "user1",
			Name:      "John Doe",
			Email:     "john@example.com",
			Workspace: "workspace1",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace1",
			Name:     "john@example.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name was updated to match user name
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace1"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "John Doe", result.Name)
	})

	t.Run("PersonalWorkspaceWithNonEmailFormatName", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User and personal workspace with regular name (no @)
		testUser := mongodoc.UserDocument{
			ID:        "user2",
			Name:      "Jane Doe",
			Email:     "jane@example.com",
			Workspace: "workspace2",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace2",
			Name:     "My Workspace",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name remains unchanged (not email format)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace2"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "My Workspace", result.Name)
	})

	t.Run("NonPersonalWorkspaceWithEmailFormatName", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User and non-personal workspace with email-formatted name
		testUser := mongodoc.UserDocument{
			ID:        "user3",
			Name:      "Bob Smith",
			Email:     "bob@example.com",
			Workspace: "workspace3",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace3",
			Name:     "team@example.com",
			Personal: false,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name remains unchanged (not personal)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace3"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "team@example.com", result.Name)
	})

	t.Run("PersonalWorkspaceWithoutUser", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		workspaceCol := client.WithCollection("workspace")

		// Setup: Personal workspace with email format but no matching user
		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace4",
			Name:     "orphan@example.com",
			Personal: true,
		}

		_, err := workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name remains unchanged (no matching user)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace4"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "orphan@example.com", result.Name)
	})

	t.Run("UserWithEmptyName", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User with empty name
		testUser := mongodoc.UserDocument{
			ID:        "user5",
			Name:      "",
			Email:     "empty@example.com",
			Workspace: "workspace5",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace5",
			Name:     "empty@example.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name remains unchanged (user name is empty)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace5"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "empty@example.com", result.Name)
	})

	t.Run("WorkspaceNameMatchesUserName", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: Workspace name already matches user name (even with @ in name)
		testUser := mongodoc.UserDocument{
			ID:        "user6",
			Name:      "same@example.com",
			Email:     "same@example.com",
			Workspace: "workspace6",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace6",
			Name:     "same@example.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name remains unchanged (already matches)
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace6"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "same@example.com", result.Name)
	})

	t.Run("MultiplePersonalWorkspaces", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: Multiple users with personal workspaces
		testUser1 := mongodoc.UserDocument{
			ID:        "user7",
			Name:      "Alice Wonder",
			Email:     "alice@example.com",
			Workspace: "workspace7",
		}

		testWorkspace1 := mongodoc.WorkspaceDocument{
			ID:       "workspace7",
			Name:     "alice@example.com",
			Personal: true,
		}

		testUser2 := mongodoc.UserDocument{
			ID:        "user8",
			Name:      "Charlie Brown",
			Email:     "charlie@example.com",
			Workspace: "workspace8",
		}

		testWorkspace2 := mongodoc.WorkspaceDocument{
			ID:       "workspace8",
			Name:     "charlie@example.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser1)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace1)
		assert.NoError(t, err)
		_, err = userCol.Client().InsertOne(ctx, testUser2)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace2)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify both workspaces were updated
		var result1 mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace7"}).Decode(&result1)
		assert.NoError(t, err)
		assert.Equal(t, "Alice Wonder", result1.Name)

		var result2 mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace8"}).Decode(&result2)
		assert.NoError(t, err)
		assert.Equal(t, "Charlie Brown", result2.Name)
	})

	t.Run("EukaryaEmailDomain", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User with eukarya.io email domain
		testUser := mongodoc.UserDocument{
			ID:        "user9",
			Name:      "Eukarya User",
			Email:     "user@eukarya.io",
			Workspace: "workspace9",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace9",
			Name:     "user@eukarya.io",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name was updated
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace9"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "Eukarya User", result.Name)
	})

	t.Run("GmailDomain", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: User with gmail.com email domain
		testUser := mongodoc.UserDocument{
			ID:        "user10",
			Name:      "Gmail User",
			Email:     "someone@gmail.com",
			Workspace: "workspace10",
		}

		testWorkspace := mongodoc.WorkspaceDocument{
			ID:       "workspace10",
			Name:     "someone@gmail.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify workspace name was updated
		var result mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace10"}).Decode(&result)
		assert.NoError(t, err)
		assert.Equal(t, "Gmail User", result.Name)
	})

	t.Run("MixedEmailDomains", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Setup: Multiple users with different email domains
		testUser1 := mongodoc.UserDocument{
			ID:        "user11",
			Name:      "Eukarya Admin",
			Email:     "admin@eukarya.io",
			Workspace: "workspace11",
		}

		testWorkspace1 := mongodoc.WorkspaceDocument{
			ID:       "workspace11",
			Name:     "admin@eukarya.io",
			Personal: true,
		}

		testUser2 := mongodoc.UserDocument{
			ID:        "user12",
			Name:      "Gmail Developer",
			Email:     "dev@gmail.com",
			Workspace: "workspace12",
		}

		testWorkspace2 := mongodoc.WorkspaceDocument{
			ID:       "workspace12",
			Name:     "dev@gmail.com",
			Personal: true,
		}

		testUser3 := mongodoc.UserDocument{
			ID:        "user13",
			Name:      "Example User",
			Email:     "test@example.com",
			Workspace: "workspace13",
		}

		testWorkspace3 := mongodoc.WorkspaceDocument{
			ID:       "workspace13",
			Name:     "test@example.com",
			Personal: true,
		}

		_, err := userCol.Client().InsertOne(ctx, testUser1)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace1)
		assert.NoError(t, err)
		_, err = userCol.Client().InsertOne(ctx, testUser2)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace2)
		assert.NoError(t, err)
		_, err = userCol.Client().InsertOne(ctx, testUser3)
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, testWorkspace3)
		assert.NoError(t, err)

		// Run migration
		err = SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)

		// Verify all workspaces were updated correctly
		var result1 mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace11"}).Decode(&result1)
		assert.NoError(t, err)
		assert.Equal(t, "Eukarya Admin", result1.Name)

		var result2 mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace12"}).Decode(&result2)
		assert.NoError(t, err)
		assert.Equal(t, "Gmail Developer", result2.Name)

		var result3 mongodoc.WorkspaceDocument
		err = workspaceCol.Client().FindOne(ctx, bson.M{"id": "workspace13"}).Decode(&result3)
		assert.NoError(t, err)
		assert.Equal(t, "Example User", result3.Name)
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)

		// Run migration on empty database - should not error
		err := SyncUserNameToWorkspace(ctx, client)
		assert.NoError(t, err)
	})
}
