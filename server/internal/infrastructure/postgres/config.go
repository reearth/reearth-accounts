package postgres

import (
	"context"
	"errors"
	"sync"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/reearth/reearth-accounts/server/internal/infrastructure/postgres/pgdoc"
	"github.com/reearth/reearth-accounts/server/pkg/config"
	"github.com/reearth/reearthx/rerror"
)

// configAdvisoryLockKey is an arbitrary, stable 64-bit key for the single config row.
const configAdvisoryLockKey int64 = 0x52454143 // "REAC"

type Config struct {
	pool *pgxpool.Pool
	mu   sync.Mutex
	conn *pgxpool.Conn // session holding the advisory lock; nil when unlocked
}

func NewConfig(pool *pgxpool.Pool) *Config { return &Config{pool: pool} }

var _ config.Repo = (*Config)(nil)

func (r *Config) LockAndLoad(ctx context.Context) (*config.Config, error) {
	r.mu.Lock()
	defer r.mu.Unlock()

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
		// Release the advisory lock and connection so a failed load doesn't leak
		// a pool connection or hold the lock forever. Safe to clear r.conn here:
		// we still hold r.mu (released by the deferred Unlock above).
		_, _ = conn.Exec(ctx, `SELECT pg_advisory_unlock($1)`, configAdvisoryLockKey)
		conn.Release()
		r.conn = nil
		return nil, rerror.ErrInternalByWithContext(ctx, err)
	}
	return row.Model(), nil
}

// exec runs on the locked connection if held, else on the pool. The held
// connection is read under r.mu to avoid racing with LockAndLoad/Unlock.
func (r *Config) exec(ctx context.Context, sql string, args ...any) error {
	r.mu.Lock()
	conn := r.conn
	r.mu.Unlock()
	if conn != nil {
		_, err := conn.Exec(ctx, sql, args...)
		return err
	}
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
