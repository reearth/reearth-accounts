package migration

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearthx/mongox"
	"github.com/reearth/reearthx/mongox/mongotest"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"go.mongodb.org/mongo-driver/bson"
	mongodriver "go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func init() {
	mongotest.Env = "REEARTH_DB"
}

// createWorkspaceAliasUniqueIndex mirrors the index that
// AddCaseInsensitiveWorkspaceAliasIndex puts on the workspace collection in
// production. Without it a regression in the conflict handling would write the
// duplicate alias and the test would still pass.
func createWorkspaceAliasUniqueIndex(t *testing.T, ctx context.Context, db *mongodriver.Database) {
	t.Helper()

	_, err := db.Collection("workspace").Indexes().CreateOne(ctx, mongodriver.IndexModel{
		Keys: bson.D{{Key: "alias", Value: 1}},
		Options: options.Index().SetCollation(&options.Collation{
			Locale:   "en",
			Strength: 2,
		}).SetUnique(true).SetName("alias_case_insensitive_unique"),
	})
	require.NoError(t, err)
}

func fetchAliasSyncSkips(t *testing.T, ctx context.Context, db *mongodriver.Database) []AliasSyncSkipRecord {
	t.Helper()

	cursor, err := db.Collection(aliasSyncSkipCollection).Find(ctx, bson.M{})
	require.NoError(t, err)

	var records []AliasSyncSkipRecord
	require.NoError(t, cursor.All(ctx, &records))
	return records
}

func workspaceAlias(t *testing.T, ctx context.Context, db *mongodriver.Database, id string) string {
	t.Helper()

	var ws mongodoc.WorkspaceDocument
	require.NoError(t, db.Collection("workspace").FindOne(ctx, bson.M{"id": id}).Decode(&ws))
	return ws.Alias
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
		assert.Equal(t, "userone", workspaceAlias(t, ctx, db, "workspace1"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
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
		assert.Equal(t, "usertwo", workspaceAlias(t, ctx, db, "workspace2"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
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
		assert.Equal(t, "teamworkspace", workspaceAlias(t, ctx, db, "workspace3"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
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
		assert.Equal(t, "orphanworkspace", workspaceAlias(t, ctx, db, "workspace4"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
	})

	t.Run("EmptyDatabase", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)

		client := mongox.NewClientWithDatabase(db)

		// Run migration on empty database - should not error
		err := SyncPersonalWorkspaceAlias(ctx, client)
		assert.NoError(t, err)
	})

	t.Run("SkipsWhenDesiredAliasIsHeldByAnotherWorkspace", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// A team workspace already holds the alias the user wants, differing
		// only by case - which alias_case_insensitive_unique treats as equal.
		_, err := workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:       "teamws",
			Name:     "Team Workspace",
			Alias:    "SharedAlias",
			Personal: false,
		})
		assert.NoError(t, err)

		_, err = userCol.Client().InsertOne(ctx, mongodoc.UserDocument{
			ID:        "user5",
			Name:      "User Five",
			Email:     "user5@example.com",
			Alias:     "sharedalias",
			Workspace: "personalws",
		})
		assert.NoError(t, err)

		_, err = workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:       "personalws",
			Name:     "Personal Workspace",
			Alias:    "oldpersonalalias",
			Personal: true,
		})
		assert.NoError(t, err)

		// The migration must complete instead of aborting on E11000.
		err = SyncPersonalWorkspaceAlias(ctx, client)
		require.NoError(t, err)

		// Neither workspace is touched.
		assert.Equal(t, "oldpersonalalias", workspaceAlias(t, ctx, db, "personalws"))
		assert.Equal(t, "SharedAlias", workspaceAlias(t, ctx, db, "teamws"))

		// The skip is recorded so it can be followed up on later.
		records := fetchAliasSyncSkips(t, ctx, db)
		require.Len(t, records, 1)
		assert.Equal(t, "personalws", records[0].ID)
		assert.Equal(t, "personalws", records[0].WorkspaceID)
		assert.Equal(t, "oldpersonalalias", records[0].WorkspaceAlias)
		assert.Equal(t, "user5", records[0].UserID)
		assert.Equal(t, "sharedalias", records[0].UserAlias)
		assert.Equal(t, "teamws", records[0].ConflictingWorkspaceID)
		assert.Equal(t, aliasSyncSkipReason, records[0].Reason)
		assert.Equal(t, syncPersonalWorkspaceAliasKey, records[0].MigrationKey)
		assert.False(t, records[0].RecordedAt.IsZero())
	})

	t.Run("SyncsNonConflictingWorkspacesEvenWhenOthersAreSkipped", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		_, err := workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:    "teamws",
			Name:  "Team Workspace",
			Alias: "takenalias",
		})
		assert.NoError(t, err)

		users := []mongodoc.UserDocument{
			{ID: "blocked", Name: "Blocked", Email: "b@example.com", Alias: "takenalias", Workspace: "blockedws"},
			{ID: "ok", Name: "Ok", Email: "o@example.com", Alias: "freealias", Workspace: "okws"},
		}
		for _, u := range users {
			_, err = userCol.Client().InsertOne(ctx, u)
			assert.NoError(t, err)
		}

		workspaces := []mongodoc.WorkspaceDocument{
			{ID: "blockedws", Name: "Blocked WS", Alias: "oldblocked", Personal: true},
			{ID: "okws", Name: "Ok WS", Alias: "oldok", Personal: true},
		}
		for _, ws := range workspaces {
			_, err = workspaceCol.Client().InsertOne(ctx, ws)
			assert.NoError(t, err)
		}

		err = SyncPersonalWorkspaceAlias(ctx, client)
		require.NoError(t, err)

		assert.Equal(t, "oldblocked", workspaceAlias(t, ctx, db, "blockedws"))
		assert.Equal(t, "freealias", workspaceAlias(t, ctx, db, "okws"))

		records := fetchAliasSyncSkips(t, ctx, db)
		require.Len(t, records, 1)
		assert.Equal(t, "blockedws", records[0].WorkspaceID)
	})

	t.Run("NormalizesAliasDifferingOnlyByCase", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		_, err := userCol.Client().InsertOne(ctx, mongodoc.UserDocument{
			ID:        "user6",
			Name:      "User Six",
			Email:     "user6@example.com",
			Alias:     "myalias",
			Workspace: "casews",
		})
		assert.NoError(t, err)

		_, err = workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:       "casews",
			Name:     "Case Workspace",
			Alias:    "MyAlias",
			Personal: true,
		})
		assert.NoError(t, err)

		// The workspace holds the index entry itself, so this is not a conflict.
		err = SyncPersonalWorkspaceAlias(ctx, client)
		require.NoError(t, err)

		assert.Equal(t, "myalias", workspaceAlias(t, ctx, db, "casews"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
	})

	t.Run("ClaimsAnAliasReleasedByALowerNumberedWorkspace", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// ws-a gives up "aliasone", which is exactly what ws-b wants. ws-a
		// sorts first, so it is decided first and ws-b can take the released
		// alias. Both rows are inserted in the opposite order on purpose: if
		// the decision pass followed scan order instead of workspace ID, ws-b
		// would be decided while ws-a still held "aliasone" and get skipped.
		users := []mongodoc.UserDocument{
			{ID: "user-b", Name: "B", Email: "b@example.com", Alias: "aliasone", Workspace: "ws-b"},
			{ID: "user-a", Name: "A", Email: "a@example.com", Alias: "freshalias", Workspace: "ws-a"},
		}
		for _, u := range users {
			_, err := userCol.Client().InsertOne(ctx, u)
			assert.NoError(t, err)
		}

		workspaces := []mongodoc.WorkspaceDocument{
			{ID: "ws-b", Name: "B WS", Alias: "aliastwo", Personal: true},
			{ID: "ws-a", Name: "A WS", Alias: "aliasone", Personal: true},
		}
		for _, ws := range workspaces {
			_, err := workspaceCol.Client().InsertOne(ctx, ws)
			assert.NoError(t, err)
		}

		require.NoError(t, SyncPersonalWorkspaceAlias(ctx, client))

		assert.Equal(t, "freshalias", workspaceAlias(t, ctx, db, "ws-a"))
		assert.Equal(t, "aliasone", workspaceAlias(t, ctx, db, "ws-b"))
		assert.Empty(t, fetchAliasSyncSkips(t, ctx, db))
	})

	t.Run("PicksTheLowestUserIDWhenAWorkspaceHasSeveralOwners", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		// Two users claim the same personal workspace. The higher ID is
		// inserted first, so last-writer-wins would pick it.
		users := []mongodoc.UserDocument{
			{ID: "user-2", Name: "Two", Email: "two@example.com", Alias: "secondalias", Workspace: "sharedws"},
			{ID: "user-1", Name: "One", Email: "one@example.com", Alias: "firstalias", Workspace: "sharedws"},
		}
		for _, u := range users {
			_, err := userCol.Client().InsertOne(ctx, u)
			assert.NoError(t, err)
		}

		_, err := workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:       "sharedws",
			Name:     "Shared WS",
			Alias:    "oldshared",
			Personal: true,
		})
		assert.NoError(t, err)

		require.NoError(t, SyncPersonalWorkspaceAlias(ctx, client))

		assert.Equal(t, "firstalias", workspaceAlias(t, ctx, db, "sharedws"))
	})

	t.Run("SkipRecordsDoNotDuplicateOnRerun", func(t *testing.T) {
		ctx := context.Background()
		db := mongotest.Connect(t)(t)
		createWorkspaceAliasUniqueIndex(t, ctx, db)

		client := mongox.NewClientWithDatabase(db)
		userCol := client.WithCollection("user")
		workspaceCol := client.WithCollection("workspace")

		_, err := workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:    "teamws",
			Name:  "Team Workspace",
			Alias: "conflict",
		})
		assert.NoError(t, err)
		_, err = userCol.Client().InsertOne(ctx, mongodoc.UserDocument{
			ID:        "user7",
			Name:      "User Seven",
			Email:     "user7@example.com",
			Alias:     "conflict",
			Workspace: "personalws",
		})
		assert.NoError(t, err)
		_, err = workspaceCol.Client().InsertOne(ctx, mongodoc.WorkspaceDocument{
			ID:       "personalws",
			Name:     "Personal Workspace",
			Alias:    "oldalias",
			Personal: true,
		})
		assert.NoError(t, err)

		require.NoError(t, SyncPersonalWorkspaceAlias(ctx, client))
		require.NoError(t, SyncPersonalWorkspaceAlias(ctx, client))

		assert.Len(t, fetchAliasSyncSkips(t, ctx, db), 1)
	})
}
