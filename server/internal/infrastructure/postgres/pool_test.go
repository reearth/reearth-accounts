//go:build integration

package postgres

import (
	"context"
	"strings"
	"testing"

	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/migration"
	"github.com/reearth/reearth-accounts/server/pkg/user"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
	"github.com/testcontainers/testcontainers-go"
	tcpostgres "github.com/testcontainers/testcontainers-go/modules/postgres"
	"github.com/testcontainers/testcontainers-go/wait"
	"go.opentelemetry.io/otel"
	sdktrace "go.opentelemetry.io/otel/sdk/trace"
	"go.opentelemetry.io/otel/sdk/trace/tracetest"
)

func TestNewPool_QueriesProduceSpansWithoutArguments(t *testing.T) {
	exp := tracetest.NewInMemoryExporter()
	tp := sdktrace.NewTracerProvider(sdktrace.WithSyncer(exp))
	otel.SetTracerProvider(tp)

	ctx := context.Background()
	c, err := tcpostgres.Run(ctx, "postgres:17-alpine",
		tcpostgres.WithDatabase("test"),
		tcpostgres.WithUsername("test"),
		tcpostgres.WithPassword("test"),
		testcontainers.WithWaitStrategy(wait.ForLog("database system is ready to accept connections").WithOccurrence(2)),
	)
	require.NoError(t, err)
	defer func() { _ = c.Terminate(ctx) }()
	dsn, err := c.ConnectionString(ctx, "sslmode=disable")
	require.NoError(t, err)

	pool, err := NewPool(ctx, dsn)
	require.NoError(t, err)
	defer pool.Close()
	require.NoError(t, migration.Migrate(ctx, pool))

	const email = "span-probe@example.com"
	r := NewUser(NewClient(pool))
	u := user.New().NewID().Name("span probe").Email(email).Workspace(user.NewWorkspaceID()).MustBuild()
	exp.Reset()
	// otelpgx only traces queries under a recording span, as in a request.
	ctx, root := tp.Tracer("test").Start(ctx, "request")
	defer root.End()
	require.NoError(t, r.Save(ctx, u))
	got, err := r.FindByEmail(ctx, email)
	require.NoError(t, err)
	require.Equal(t, u.ID(), got.ID())

	var names []string
	for _, s := range exp.GetSpans() {
		names = append(names, s.Name)
		for _, kv := range s.Attributes {
			assert.False(t, strings.Contains(kv.Value.Emit(), email), "%s carries the email", kv.Key)
		}
	}
	assert.Contains(t, names, "query UserUpsert")
	assert.Contains(t, names, "query UserFindByEmail")
}
