package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearthx/usecasex"
)

// Transaction implements usecasex.Transaction over a pgxpool.Pool.
type Transaction struct {
	pool *pgxpool.Pool
}

func NewTransaction(pool *pgxpool.Pool) usecasex.Transaction {
	return &Transaction{pool: pool}
}

func (t *Transaction) Begin(ctx context.Context) (usecasex.Tx, error) {
	// If a tx is already active in ctx, reuse it (nested calls share the ambient tx).
	if existing, ok := txFromContext(ctx); ok {
		return &Tx{ctx: ctx, tx: existing, nested: true}, nil
	}
	pgtx, err := t.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Tx{ctx: txToContext(ctx, pgtx), tx: pgtx}, nil
}

// Tx implements usecasex.Tx; it commits on End only when Commit was called.
type Tx struct {
	ctx       context.Context
	tx        pgx.Tx
	committed bool
	nested    bool
}

func (t *Tx) Context() context.Context { return t.ctx }

func (t *Tx) Commit() { t.committed = true }

func (t *Tx) IsCommitted() bool { return t.committed }

func (t *Tx) End(ctx context.Context) error {
	// A nested Tx is owned by the outer Begin; do not commit/rollback here.
	if t.nested {
		return nil
	}
	if t.committed {
		return t.tx.Commit(ctx)
	}
	return t.tx.Rollback(ctx)
}
