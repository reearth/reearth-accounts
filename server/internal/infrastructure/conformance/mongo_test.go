//go:build integration

package conformance

import (
	"context"
	"testing"

	"github.com/google/uuid"
	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/stretchr/testify/require"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/bson"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoConformance(t *testing.T) {
	ctx := context.Background()

	// One container for the whole suite; isolate subtests with a fresh database each.
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

		// Create the case-insensitive unique email index (mirrors the production
		// mongo migration) so duplicate-email parity is exercised here too.
		_, err := db.Collection("user").Indexes().CreateOne(ctx, mongo.IndexModel{
			Keys:    bson.D{{Key: "email", Value: 1}},
			Options: options.Index().SetUnique(true).SetCollation(&options.Collation{Locale: "en", Strength: 2}),
		})
		require.NoError(t, err)

		repos, err := mongorepo.New(ctx, db, false, false, nil)
		require.NoError(t, err)
		return repos, Caps{
			EnforcesFilter:   true,
			OrderedFindByIDs: true,
			RealPagination:   true,
			UniqueEmail:      true,
			SubstringSearch:  true,
		}, func() { _ = db.Drop(ctx) }
	})
}
