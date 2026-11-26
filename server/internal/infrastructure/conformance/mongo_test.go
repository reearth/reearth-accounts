//go:build integration

package conformance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo/mongodoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/interactor"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoConformance(t *testing.T) {
	ctx := context.Background()

	c, err := tcmongo.Run(ctx, "mongo:6")
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })
	dsn, err := c.ConnectionString(ctx)
	require.NoError(t, err)
	cli, err := mongo.Connect(ctx, options.Client().ApplyURI(dsn))
	require.NoError(t, err)
	t.Cleanup(func() { _ = cli.Disconnect(ctx) })

	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		dbName := "conf_" + uuid.NewString()
		db := cli.Database(dbName)

		// case-insensitive unique email index mirrors the production mongo migration
		_, err := db.Collection("user").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		})
		require.NoError(t, err)

		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)
		return repos, Caps{
			EnforcesFilter:  true,
			EnforcesLimit:   true,
			RealPagination:  true,
			SubstringSearch: true,
			UniqueEmail:     true,
		}, func() { _ = db.Drop(ctx) }
	})

	t.Run("Workspace_LeaveAllAsIntegration_MembersHash", func(t *testing.T) {
		db := cli.Database("conf_" + uuid.NewString())
		t.Cleanup(func() { _ = db.Drop(ctx) })
		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)
		iid, other := id.NewIntegrationID(), id.NewIntegrationID()
		member, _ := seedLeaveAllWorkspaces(t, repos, iid, other)

		_, err = interactor.NewWorkspace(repos, nil, nil).
			LeaveAllAsIntegration(ctx, &workspace.Operator{Integration: &iid, Client: &workspace.Client{ID: "c"}})
		require.NoError(t, err)

		for _, wid := range member {
			var doc mongodoc.WorkspaceDocument
			require.NoError(t, db.Collection("workspace").FindOne(ctx, bson.M{"id": wid.String()}).Decode(&doc))
			assert.NotContains(t, doc.Integrations, iid.String())
			want, err := mongodoc.ComputeWorkspaceMembersHash(doc.Members, doc.Integrations)
			require.NoError(t, err)
			assert.Equal(t, want, doc.MembersHash)
		}
	})
}
