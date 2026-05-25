//go:build integration

package e2e

import (
	"context"
	"encoding/json"
	"net/http"
	"testing"

	httpexpect "github.com/gavv/httpexpect/v2"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/app"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
)

// TestPostgresE2E_GetAndUpdateMe drives the real server (GraphQL over HTTP) with
// the PostgreSQL backend: read -> mutate -> read, all the way down to SQL.
func TestPostgresE2E_GetAndUpdateMe(t *testing.T) {
	ctx := context.Background()

	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"), tcpostgres.WithUsername("test"), tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForListeningPort("5432/tcp")))
	require.NoError(t, err)
	t.Cleanup(func() { _ = c.Terminate(ctx) })

	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)
	pool, err := pgxpool.New(ctx, dsn)
	require.NoError(t, err)
	t.Cleanup(pool.Close)
	require.NoError(t, migration.Migrate(ctx, pool))

	repos, err := postgres.New(ctx, pool, nil)
	require.NoError(t, err)

	// Seed users + workspaces through the postgres repos (real SQL writes).
	require.NoError(t, baseSeederGetMe(ctx, repos))

	// Boot the actual server wired to the postgres repo container.
	e := StartServerWithRepos(t, &app.Config{}, repos)

	post := func(query string) *httpexpect.Object {
		jsonData, mErr := json.Marshal(GraphQLRequest{Query: query})
		require.NoError(t, mErr)
		return e.POST("/api/graphql").
			WithHeader("authorization", "Bearer test").
			WithHeader("Content-Type", "application/json").
			WithHeader("X-Reearth-Debug-User", uId.String()).
			WithBytes(jsonData).
			Expect().Status(http.StatusOK).
			JSON().Object()
	}

	// 1. Read `me` -> GraphQL -> resolver -> service -> postgres read.
	me := post(`{ me { id name alias email myWorkspaceId } }`).
		Value("data").Object().Value("me").Object()
	me.Value("id").String().IsEqual(uId.String())
	me.Value("name").String().IsEqual("Test User")
	me.Value("alias").String().IsEqual("testuser")
	me.Value("myWorkspaceId").String().IsEqual(wId.String())

	// 2. Mutate alias -> GraphQL mutation -> service -> postgres write.
	updated := post(`mutation { updateMe(input: {alias: "pgaliasupdated"}){ me{ id alias } }}`).
		Value("data").Object().Value("updateMe").Object().Value("me").Object()
	updated.Value("alias").String().IsEqual("pgaliasupdated")

	// 3. Re-read -> the write actually persisted to postgres.
	post(`{ me { id alias } }`).
		Value("data").Object().Value("me").Object().
		Value("alias").String().IsEqual("pgaliasupdated")

	// 4. Confirm at the SQL layer, independent of the API.
	var alias string
	require.NoError(t, pool.QueryRow(ctx, `SELECT alias FROM users WHERE id = $1`, uId.String()).Scan(&alias))
	require.Equal(t, "pgaliasupdated", alias)
}
