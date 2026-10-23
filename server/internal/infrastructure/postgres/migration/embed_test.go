//go:build integration

package migration_test

import (
	"context"
	"os"
	"testing"

	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/pkg/id"
	"github.com/reearth/reearth-accounts/server/pkg/role"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

func startPG(t *testing.T) (*pgxpool.Pool, func()) {
	t.Helper()
	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	return pool, func() { pool.Close(); _ = c.Terminate(ctx) }
}

func TestMigrate(t *testing.T) {
	pool, done := startPG(t)
	defer done()
	ctx := context.Background()

	require.NoError(t, migration.Migrate(ctx, pool))

	var n int
	require.NoError(t, pool.QueryRow(ctx,
		`SELECT count(*) FROM information_schema.tables WHERE table_name = 'users'`).Scan(&n))
	assert.Equal(t, 1, n)

	// idempotent: second Migrate is a no-op (ErrNoChange swallowed)
	require.NoError(t, migration.Migrate(ctx, pool))
}

func roleIDsByName(t *testing.T, pool *pgxpool.Pool) map[string]string {
	t.Helper()
	rows, err := pool.Query(context.Background(), `SELECT id, name FROM roles`)
	require.NoError(t, err)
	defer rows.Close()
	got := map[string]string{}
	for rows.Next() {
		var rid, name string
		require.NoError(t, rows.Scan(&rid, &name))
		got[name] = rid
	}
	require.NoError(t, rows.Err())
	return got
}

func TestMigrate_SeedRoles(t *testing.T) {
	pool, done := startPG(t)
	defer done()
	ctx := context.Background()

	require.NoError(t, migration.Migrate(ctx, pool))

	got := roleIDsByName(t, pool)
	assert.Len(t, got, 5)
	for _, rt := range []role.RoleType{role.RoleReader, role.RoleWriter, role.RoleMaintainer, role.RoleOwner, role.RoleSelf} {
		rid, ok := got[rt.String()]
		require.True(t, ok, "role %s missing", rt)
		r, err := pgdoc.RoleRow{ID: rid, Name: rt.String()}.Model()
		require.NoError(t, err, "role %s id %q must parse as a RoleID", rt, rid)
		assert.Equal(t, rt.String(), r.Name())
	}

	// A row that already exists under another id must be kept as is, and
	// re-running the migration must not add rows.
	existingOwner := id.NewRoleID().String()
	_, err := pool.Exec(ctx, `DELETE FROM roles WHERE name = 'owner'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO roles (id, name) VALUES ($1, 'owner')`, existingOwner)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `UPDATE schema_migrations SET version = 5, dirty = false`)
	require.NoError(t, err)

	require.NoError(t, migration.Migrate(ctx, pool))

	again := roleIDsByName(t, pool)
	assert.Len(t, again, 5)
	assert.Equal(t, existingOwner, again["owner"])
	for name, rid := range got {
		if name != "owner" {
			assert.Equal(t, rid, again[name])
		}
	}
}

func TestMigrate_SeedRolesDown(t *testing.T) {
	pool, done := startPG(t)
	defer done()
	ctx := context.Background()

	require.NoError(t, migration.Migrate(ctx, pool))
	seeded := roleIDsByName(t, pool)
	foreign := id.NewRoleID().String()
	_, err := pool.Exec(ctx, `DELETE FROM roles WHERE name = 'self'`)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO roles (id, name) VALUES ($1, 'self')`, foreign)
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO permittables (id, user_id, role_ids) VALUES ('p1', 'u1', ARRAY[$1])`, seeded["reader"])
	require.NoError(t, err)
	_, err = pool.Exec(ctx, `INSERT INTO permittable_workspace_roles (permittable_id, workspace_id, role_id) VALUES ('p1', 'w1', $1)`, seeded["owner"])
	require.NoError(t, err)

	down, err := os.ReadFile("0006_seed_roles.down.sql")
	require.NoError(t, err)
	_, err = pool.Exec(ctx, string(down))
	require.NoError(t, err)

	left := roleIDsByName(t, pool)
	assert.Equal(t, map[string]string{
		"reader": seeded["reader"],
		"owner":  seeded["owner"],
		"self":   foreign,
	}, left)
}
