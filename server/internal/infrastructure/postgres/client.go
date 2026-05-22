package postgres

import (
	"context"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
)

// DBTX is the interface satisfied by both *pgxpool.Pool and pgx.Tx, matching
// the sqlc-generated DBTX. We alias the generated one so repos and sqlc agree.
type DBTX = gen.DBTX

type txCtxKey struct{}

// withTx returns a child context carrying the active transaction.
func withTx(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

// txFromContext returns the active tx if present.
func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx)
	return tx, ok
}

// Client owns the pool and resolves the active DBTX (tx if present, else pool).
type Client struct {
	pool *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool) *Client {
	return &Client{pool: pool}
}

// resolve returns the ambient tx if one is in context, otherwise the pool.
func (c *Client) resolve(ctx context.Context) DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return c.pool
}

// queries returns sqlc Queries bound to the resolved DBTX so every call
// participates in the ambient transaction automatically.
func (c *Client) queries(ctx context.Context) *gen.Queries {
	return gen.New(c.resolve(ctx))
}

// lower lowercases a string (used for case-insensitive alias matching).
func lower(s string) string { return strings.ToLower(s) }

// itoa wraps strconv.Itoa for terse placeholder construction.
func itoa(n int) string { return strconv.Itoa(n) }
