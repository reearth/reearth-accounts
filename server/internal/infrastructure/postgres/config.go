package postgres

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/internal/usecase/repo"
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearthx/rerror"
)

const configAdvisoryLockKey int64 = 0x52454143 // "REAC"

type Config struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	conn *pgxpool.Conn // session holding the advisory lock; nil when unlocked
}

func NewConfig(pool *pgxpool.Pool) config.Repo { return &Config{pool: pool} }

func (r *Config) LockAndLoad(ctx context.Context) (*config.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

	// Fast-path: if this instance already holds the advisory lock, refuse to
	// re-acquire. Without this guard, a second LockAndLoad on the same *Config
	// would block on pg_advisory_lock while still holding r.mu, deadlocking
	// any concurrent Unlock that needs r.mu to release the existing connection.
	// We return repo.ErrAlreadyLocked for parity with the Mongo implementation.
	if r.conn != nil {
		return nil, repo.ErrAlreadyLocked
	}

	conn, err := r.pool.Acquire(ctx)
	if err != nil {
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	if _, err := conn.Exec(ctx, `SELECT pg_advisory_lock($1)`, configAdvisoryLockKey); err != nil {
		conn.Release()
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	r.conn = conn

	row := pgdoc.ConfigRow{}
	err = conn.QueryRow(ctx, `SELECT migration, auth_cert, auth_key, default_policy FROM config WHERE id = 1`).
		Scan(&row.Migration, &row.AuthCert, &row.AuthKey, &row.DefaultPolicy)
	if errors.Is(err, pgx.ErrNoRows) {
		return &config.Config{}, nil
	}
	if err != nil {
		// release lock+conn on load failure to avoid leaking either
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, configAdvisoryLockKey)
		conn.Release()
		r.conn = nil
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return row.Model(), nil
}

// exec holds r.mu across the locked-conn Exec because pgxpool.Conn isn't safe
// for concurrent use; releasing mid-Exec would race Unlock returning the conn to the pool.
func (r *Config) exec(ctx context.Context, sql string, args ...any) error {
	r.mu.Lock()
	if r.conn != nil {
		_, err := r.conn.Exec(ctx, sql, args...)
		r.mu.Unlock()
		return err
	}
	r.mu.Unlock()
	_, err := r.pool.Exec(ctx, sql, args...)
	return err
}

func (r *Config) Save(ctx context.Context, cfg *config.Config) error {
	if cfg == nil {
		return nil
	}
	row := pgdoc.NewConfigRow(*cfg)
	if err := r.exec(ctx,
		`INSERT INTO config (id, migration, auth_cert, auth_key, default_policy) VALUES (1,$1,$2,$3,$4)
		 ON CONFLICT (id) DO UPDATE SET migration=EXCLUDED.migration, auth_cert=EXCLUDED.auth_cert, auth_key=EXCLUDED.auth_key, default_policy=EXCLUDED.default_policy`,
		row.Migration, row.AuthCert, row.AuthKey, row.DefaultPolicy); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Config) SaveAuth(ctx context.Context, a *config.Auth) error {
	if a == nil {
		return nil
	}
	if err := r.exec(ctx,
		`INSERT INTO config (id, auth_cert, auth_key) VALUES (1,$1,$2)
		 ON CONFLICT (id) DO UPDATE SET auth_cert=EXCLUDED.auth_cert, auth_key=EXCLUDED.auth_key`,
		a.Cert, a.Key); err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}

func (r *Config) SaveAndUnlock(ctx context.Context, cfg *config.Config) error {
	if err := r.Save(ctx, cfg); err != nil {
		return err
	}
	return r.Unlock(ctx)
}

func (r *Config) Unlock(ctx context.Context) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if r.conn == nil {
		return nil
	}
	_, err := r.conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, configAdvisoryLockKey)
	r.conn.Release()
	r.conn = nil
	if err != nil {
		return rerror.ErrInternalByWithContext(ctx, err)
	}
	return nil
}
