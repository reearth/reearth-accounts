//go:build integration

package postgres

import (
	"context"
	"testing"

	"github.com/reearth/reearth-accounts/server/pkg/workspace"
	"github.com/reearth/reearthx/usecasex"
	"github.com/stretchr/testify/assert"
)

func TestWorkspace_FindAll_NotImplemented(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()
	ctx := context.Background()
	r := NewWorkspace(NewClient(pool))

	// FindAll (admin cross-tenant listing) is intentionally not implemented on
	// the Postgres backend; the admin app runs on MongoDB.
	_, _, err := r.FindAll(ctx, nil, usecasex.OffsetPagination{Offset: 0, Limit: 10}.Wrap())
	assert.ErrorIs(t, err, workspace.ErrNotImplemented)
}
