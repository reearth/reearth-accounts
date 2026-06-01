//go:build integration

package postgres

import (
	"context"
	"testing"
	"time"

	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestConfigLockAndLoadDoesNotDeadlock ensures a second LockAndLoad on the same
// *Config instance returns repo.ErrAlreadyLocked rather than blocking on
// pg_advisory_lock while still holding r.mu (which would prevent Unlock from
// ever running).
func TestConfigLockAndLoadDoesNotDeadlock(t *testing.T) {
	pool, cleanup := pgPool(t)
	defer cleanup()

	cfg := NewConfig(pool)
	ctx := context.Background()

	_, err := cfg.LockAndLoad(ctx)
	require.NoError(t, err)
	t.Cleanup(func() { _ = cfg.Unlock(context.Background()) })

	// Second call must return promptly with ErrAlreadyLocked, never block.
	done := make(chan error, 1)
	go func() {
		callCtx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
		defer cancel()
		_, err := cfg.LockAndLoad(callCtx)
		done <- err
	}()

	select {
	case err := <-done:
		assert.ErrorIs(t, err, repo.ErrAlreadyLocked)
	case <-time.After(5 * time.Second):
		t.Fatal("second LockAndLoad deadlocked (did not return within 5s)")
	}

	// After Unlock the same instance must be lockable again.
	require.NoError(t, cfg.Unlock(ctx))
	_, err = cfg.LockAndLoad(ctx)
	require.NoError(t, err)
	require.NoError(t, cfg.Unlock(ctx))
}
