package migration

import (
	"context"
	"database/sql"
	"embed"
	"errors"
	"fmt"

	"github.com/golang-migrate/migrate/v4"
	migratepgx "github.com/golang-migrate/migrate/v4/database/pgx/v5"
	"github.com/golang-migrate/migrate/v4/source/iofs"
	"github.com/jackc/pgx/v5/pgxpool"
	_ "github.com/jackc/pgx/v5/stdlib" // registers the "pgx" database/sql driver
)

//go:embed *.sql
var fs embed.FS

// Migrate applies all pending schema migrations using golang-migrate.
// golang-migrate manages its own advisory lock and schema_migrations table.
//
// It deliberately opens an independent database/sql connection (via the "pgx"
// driver) rather than borrowing from the pgxpool: golang-migrate owns its
// connection lifecycle, and binding that to the pool can leave a connection
// checked out, deadlocking a later pool.Close().
func Migrate(ctx context.Context, pool *pgxpool.Pool) error {
	src, err := iofs.New(fs, ".")
	if err != nil {
		return fmt.Errorf("postgres migration source: %w", err)
	}

	db, err := sql.Open("pgx", pool.Config().ConnConfig.ConnString())
	if err != nil {
		return fmt.Errorf("postgres migration open: %w", err)
	}
	defer func() { _ = db.Close() }()

	driver, err := migratepgx.WithInstance(db, &migratepgx.Config{})
	if err != nil {
		return fmt.Errorf("postgres migration driver: %w", err)
	}

	m, err := migrate.NewWithInstance("iofs", src, "pgx5", driver)
	if err != nil {
		return fmt.Errorf("postgres migrate init: %w", err)
	}

	if err := m.Up(); err != nil && !errors.Is(err, migrate.ErrNoChange) {
		return fmt.Errorf("postgres migrate up: %w", err)
	}
	return nil
}
