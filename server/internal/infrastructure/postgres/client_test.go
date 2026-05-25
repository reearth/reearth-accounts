package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

// fakeTx satisfies pgx.Tx (and therefore DBTX) via the embedded nil interface;
// its methods are never called in these resolution-only tests.
type fakeTx struct{ pgx.Tx }

func TestResolveDBTX(t *testing.T) {
	c := &Client{} // nil pool is fine for resolution checks
	ctx := context.Background()

	// no tx in ctx -> falls back to the pool (here a typed-nil *pgxpool.Pool).
	assert.Equal(t, DBTX(c.pool), c.db(ctx))

	// tx in ctx -> returns that tx.
	tx := fakeTx{}
	ctx2 := txToContext(ctx, tx)
	assert.Equal(t, DBTX(tx), c.db(ctx2))

	// txFromContext round-trips.
	got, ok := txFromContext(ctx2)
	assert.True(t, ok)
	assert.Equal(t, pgx.Tx(tx), got)

	_, ok = txFromContext(ctx)
	assert.False(t, ok)
}
