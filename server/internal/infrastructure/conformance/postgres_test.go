//go:build integration

package conformance

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func TestPostgresConformance(t *testing.T) {
	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		ctx := context.Background()
		c, err := tcpostgres.Run(ctx, "postgres:16-alpine",
			tcpostgres.WithDatabase("test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
			testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")))
		require.NoError(t, err)
		dsn, err := c.ConnectionString(ctx, "sslmode=disable")
		require.NoError(t, err)
		pool, err := pgxpool.New(ctx, dsn)
		require.NoError(t, err)
		require.NoError(t, migration.Migrate(ctx, pool))
		repos, err := postgres.New(ctx, pool, []user.Repo{})
		require.NoError(t, err)
		return repos, Caps{
			RealTransactions:     true,
			EnforcesFilter:       true,
			OrderedFindByIDs:     true,
			RealPagination:       true,
			CaseInsensitiveEmail: true,
			UniqueEmail:          true,
		}, func() { pool.Close(); _ = c.Terminate(ctx) }
	})
}
