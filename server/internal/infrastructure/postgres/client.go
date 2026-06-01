package postgres

import (
	"context"
	"strconv"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/sqlc/gen"
	"github.com/reearth/reearthx/rerror"
)

type DBTX = gen.DBTX

type txCtxKey struct{}

func txToContext(ctx context.Context, tx pgx.Tx) context.Context {
	return context.WithValue(ctx, txCtxKey{}, tx)
}

func txFromContext(ctx context.Context) (pgx.Tx, bool) {
	tx, ok := ctx.Value(txCtxKey{}).(pgx.Tx)
	return tx, ok
}

type Client struct {
	pool *pgxpool.Pool
}

func NewClient(pool *pgxpool.Pool) *Client {
	return &Client{pool: pool}
}

func (c *Client) db(ctx context.Context) DBTX {
	if tx, ok := txFromContext(ctx); ok {
		return tx
	}
	return c.pool
}

func (c *Client) queries(ctx context.Context) *gen.Queries {
	return gen.New(c.db(ctx))
}

// WithinTransaction reuses the ambient tx if one is in ctx (so nested calls compose).
func (c *Client) WithinTransaction(ctx context.Context, fn func(ctx context.Context) error) error {
	if _, ok := txFromContext(ctx); ok {
		return fn(ctx)
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

func itoa(n int) string { return strconv.Itoa(n) }
