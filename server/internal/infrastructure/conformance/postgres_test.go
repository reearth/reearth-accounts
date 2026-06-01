//go:build integration

package conformance

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

const pgTruncate = `TRUNCATE users, workspaces, workspace_members, workspace_integrations,
	roles, permittables, permittable_workspace_roles, config RESTART IDENTITY CASCADE`

func TestPostgresConformance(t *testing.T) {
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, migration.Migrate(ctx, pool))

	Run(t, func(t *testing.T) (*repo.Container, Caps, func()) {
		_, err := pool.Exec(ctx, pgTruncate)
		require.NoError(t, err)
		repos, err := postgres.New(ctx, pool, nil)
		require.NoError(t, err)
		return repos, Caps{
			RealTransactions:     true,
			EnforcesFilter:       true,
			OrderedFindByIDs:     true,
			RealPagination:       true,
			UniqueEmail:          true,
			SubstringSearch:      true,
			CaseInsensitiveEmail: true,
		}, func() {}
	})
}
