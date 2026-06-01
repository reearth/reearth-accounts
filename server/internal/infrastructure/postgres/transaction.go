package postgres

import (
	"context"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearthx/usecasex"
)

type Transaction struct {
	pool *pgxpool.Pool
}

func NewTransaction(pool *pgxpool.Pool) *Transaction {
	return &Transaction{pool: pool}
}

func (t *Transaction) Begin(ctx context.Context) (usecasex.Tx, error) {
	if existing, ok := txFromContext(ctx); ok {
		return &Tx{ctx: ctx, tx: existing, nested: true}, nil
	}
	pgtx, err := t.pool.Begin(ctx)
	if err != nil {
		return nil, err
	}
	return &Tx{ctx: txToContext(ctx, pgtx), tx: pgtx}, nil
}

type Tx struct {
	ctx       context.Context
	tx        pgx.Tx
	committed bool
	nested    bool
}

func (t *Tx) Context() context.Context { return t.ctx }
func (t *Tx) Commit()                  { t.committed = true }
func (t *Tx) IsCommitted() bool        { return t.committed }

func (t *Tx) End(ctx context.Context) error {
	if t.nested {
		return nil
	}
	if t.committed {
		return t.tx.Commit(ctx)
	}
	return t.tx.Rollback(ctx)
}
