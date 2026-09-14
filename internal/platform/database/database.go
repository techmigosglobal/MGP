package database

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"
	"path/filepath"
	"sort"

	_ "github.com/jackc/pgx/v5/stdlib"
	"github.com/techmigos/mgp/db"
)

func Open(ctx context.Context, databaseURL string) (*sql.DB, error) {
	connection, err := sql.Open("pgx", databaseURL)
	if err != nil {
		return nil, fmt.Errorf("open postgres: %w", err)
	}
	if err := connection.PingContext(ctx); err != nil {
		_ = connection.Close()
		return nil, fmt.Errorf("ping postgres: %w", err)
	}
	return connection, nil
}

func Migrate(ctx context.Context, connection *sql.DB) error {
	if _, err := connection.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (version text PRIMARY KEY, applied_at timestamptz NOT NULL DEFAULT now())`); err != nil {
		return err
	}
	entries, err := fs.Glob(db.Migrations, "migrations/*.sql")
	if err != nil {
		return err
	}
	sort.Strings(entries)
	for _, entry := range entries {
		version := filepath.Base(entry)
		var exists bool
		if err := connection.QueryRowContext(ctx, `SELECT EXISTS(SELECT 1 FROM schema_migrations WHERE version=$1)`, version).Scan(&exists); err != nil {
			return err
		}
		if exists {
			continue
		}
		contents, err := fs.ReadFile(db.Migrations, entry)
		if err != nil {
			return err
		}
		transaction, err := connection.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		if _, err = transaction.ExecContext(ctx, string(contents)); err == nil {
			_, err = transaction.ExecContext(ctx, `INSERT INTO schema_migrations(version) VALUES($1)`, version)
		}
		if err != nil {
			_ = transaction.Rollback()
			return fmt.Errorf("migration %s: %w", version, err)
		}
		if err = transaction.Commit(); err != nil {
			return err
		}
	}
	return nil
}
