package postgres

import (
	"context"
	"strings"

	"github.com/exaring/otelpgx"
	"github.com/jackc/pgx/v5/pgxpool"
)

// NewPool opens a pool whose queries produce OpenTelemetry spans named after
// their sqlc query, such as "query UserFindByEmail" (ADR-0003 decision 10). Spans carry the parameterized SQL,
// never the query arguments.
func NewPool(ctx context.Context, dsn string) (*pgxpool.Pool, error) {
	cfg, err := pgxpool.ParseConfig(dsn)
	if err != nil {
		return nil, err
	}
	cfg.ConnConfig.Tracer = otelpgx.NewTracer(otelpgx.WithTrimSQLInSpanName(), otelpgx.WithSpanNameFunc(spanName))
	return pgxpool.NewWithConfig(ctx, cfg)
}

// spanName returns the sqlc query name for statements that start with
// "-- name: <QueryName> :<kind>", and the SQL verb otherwise.
func spanName(stmt string) string {
	s := strings.TrimSpace(stmt)
	if rest, ok := strings.CutPrefix(s, "-- name: "); ok {
		if name, _, ok := strings.Cut(rest, " "); ok {
			return name
		}
	}
	if verb, _, _ := strings.Cut(s, " "); verb != "" {
		return strings.ToUpper(verb)
	}
	return "SQL"
}
