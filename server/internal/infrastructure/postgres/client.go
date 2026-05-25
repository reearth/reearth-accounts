package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearthx/rerror"
)

// Transactions follow the "transactor" pattern: the active transaction is
// carried on the context (txToContext/txFromContext), db(ctx) is the DBGetter
// that hands back the ambient tx or the pool, and WithinTransaction wraps a
// closure with begin/commit/rollback. Repositories only ever go through
// queries(ctx)/db(ctx), so they participate in an ambient transaction without
// knowing whether one exists.

// DBTX is the interface satisfied by both *pgxpool.Pool and pgx.Tx, matching
// the sqlc-generated DBTX. We alias the generated one so repos and sqlc agree.
type DBTX = gen.DBTX

type txCtxKey struct{}

// txToContext returns a child context carrying the active transaction.
func txToContext(ctx context.Context, tx pgx.Tx) context.Context {
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

// db is the DBGetter: it returns the ambient tx if one is in context, otherwise
// the pool. Always safe to call (falls back to the pool for an empty context).
func (c *Client) db(ctx context.Context) DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return c.pool
}

// queries returns sqlc Queries bound to the resolved DBTX so every call
// participates in the ambient transaction automatically.
func (c *Client) queries(ctx context.Context) *gen.Queries {
	return gen.New(c.db(ctx))
}

// WithinTransaction runs fn inside a transaction, passing a tx-enriched context
// to the closure. On success it commits; if fn returns an error it rolls back
// and returns that error. If a transaction is already active in ctx it reuses it
// (nested calls share the ambient tx), so transactional methods compose freely.
// This keeps multi-statement writes (parent row + child rows) atomic.
func (c *Client) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx) // already inside a transaction: reuse it
	}
	tx, err := c.pool.Begin(ctx)
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	if err := fn(txToContext(ctx, tx)); err != nil {
		_ = tx.Rollback(ctx)
		return err
	}
	if err := tx.Commit(ctx); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

// itoa wraps strconv.Itoa for terse placeholder construction.
func itoa(n int) string { return strconv.Itoa(n) }
