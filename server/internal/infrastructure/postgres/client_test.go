package postgres

import (
	"context"
	"testing"

	"github.com/jackc/pgx/v5"
	"github.com/stretchr/testify/assert"
)

type fakeTx struct{ pgx.Tx }

func TestResolveDBTX(t *testing.T) {
	c := &Client{}
	ctx := context.Background()

	assert.Equal(t, DBTX(c.pool), c.db(ctx))

	tx := fakeTx{}
	ctx2 := txToContext(ctx, tx)
	assert.Equal(t, DBTX(tx), c.db(ctx2))

	got, ok := txFromContext(ctx2)
	assert.True(t, ok)
	assert.Equal(t, pgx.Tx(tx), got)

	_, ok = txFromContext(ctx)
	assert.False(t, ok)
}
