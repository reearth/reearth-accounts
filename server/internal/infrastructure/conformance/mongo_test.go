//go:build integration

package conformance

import (
	"context"
	"testing"

	mongorepo "github.com/reearth/reearth-accounts/server/internal/infrastructure/mongo"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/require"
	tcmongo "github.com/testcontainers/testcontainers-go/modules/mongodb"
	"go.mongodb.org/mongo-driver/mongo"
	"go.mongodb.org/mongo-driver/mongo/options"
)

func TestMongoConformance(t *testing.T) {
	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		ctx := context.Background()
		c, err := tcmongo.Run(ctx, "mongo:6")
		require.NoError(t, err)
		dsn, err := c.ConnectionString(ctx)
		require.NoError(t, err)
		cli, err := mongo.Connect(ctx, options.Client().ApplyURI(dsn))
		require.NoError(t, err)
		repos, err := mongorepo.New(ctx, cli.Database("test"), false, false, []user.Repo{})
		require.NoError(t, err)
		// No migrations run here, so unique indexes / case-insensitive email are
		// absent; those behaviors are covered by mongo's own migration tests.
		return repos, Caps{
			EnforcesFilter:   true,
			OrderedFindByIDs: true,
			RealPagination:   true,
		}, func() { _ = cli.Disconnect(ctx); _ = c.Terminate(ctx) }
	})
}
