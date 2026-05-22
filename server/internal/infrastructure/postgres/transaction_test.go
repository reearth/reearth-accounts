//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestTransaction_CommitAndRollback(t *testing.T) {
	pool, done := pgPool(t)
	defer done()
	ctx := context.Background()
	trx := NewTransaction(pool)

	// commit persists
	tx, err := trx.Begin(ctx)
	require.NoError(t, err)
	txh, ok := txFromContext(tx.Context())
	require.True(t, ok)
	_, err = txh.Exec(tx.Context(), `INSERT INTO roles (id, name) VALUES ($1,$2)`, "r1", "n1")
	require.NoError(t, err)
	tx.Commit()
	require.NoError(t, tx.End(ctx))

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE id='r1'`).Scan(&n))
	assert.Equal(t, 1, n)

	// rollback discards (no Commit())
	tx2, err := trx.Begin(ctx)
	require.NoError(t, err)
	txh2, _ := txFromContext(tx2.Context())
	_, err = txh2.Exec(tx2.Context(), `INSERT INTO roles (id, name) VALUES ($1,$2)`, "r2", "n2")
	require.NoError(t, err)
	require.NoError(t, tx2.End(ctx)) // no Commit() -> rollback

	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE id='r2'`).Scan(&n))
	assert.Equal(t, 0, n)
}

func TestTransaction_NestedReusesAmbient(t *testing.T) {
	pool, done := pgPool(t)
	defer done()
	ctx := context.Background()
	trx := NewTransaction(pool)

	outer, err := trx.Begin(ctx)
	require.NoError(t, err)
	octx := outer.Context()

	inner, err := trx.Begin(octx) // ambient tx in ctx -> nested, shares the tx
	require.NoError(t, err)
	assert.True(t, inner.(*Tx).nested)
	require.NoError(t, inner.End(octx)) // nested End is a no-op (does not commit/rollback)

	oh, _ := txFromContext(octx)
	_, err = oh.Exec(octx, `INSERT INTO roles (id, name) VALUES ($1,$2)`, "r3", "n3")
	require.NoError(t, err)

	outer.Commit()
	require.NoError(t, outer.End(ctx))

	var n int
	require.NoError(t, pool.QueryRow(ctx, `SELECT count(*) FROM roles WHERE id='r3'`).Scan(&n))
	assert.Equal(t, 1, n)
}
