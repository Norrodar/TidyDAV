package store

import (
	"context"
	"embed"
	"fmt"
	"io/fs"
	"sort"
	"strings"
)

//go:embed migrations/*.sql
var migrationsFS embed.FS

// Migrate applies all embedded migrations that have not yet run, in filename
// order. Each migration runs in its own transaction and is recorded in the
// schema_migrations table, so Migrate is safe to call on every boot.
func (s *Store) Migrate(ctx context.Context) error {
	if _, err := s.db.ExecContext(ctx, `CREATE TABLE IF NOT EXISTS schema_migrations (
		version    TEXT PRIMARY KEY,
		applied_at TEXT NOT NULL
	)`); err != nil {
		return fmt.Errorf("create schema_migrations: %w", err)
	}

	entries, err := fs.ReadDir(migrationsFS, "migrations")
	if err != nil {
		return fmt.Errorf("read migrations dir: %w", err)
	}
	names := make([]string, 0, len(entries))
	for _, e := range entries {
		if !e.IsDir() && strings.HasSuffix(e.Name(), ".sql") {
			names = append(names, e.Name())
		}
	}
	sort.Strings(names)

	for _, name := range names {
		applied, err := s.migrationApplied(ctx, name)
		if err != nil {
			return err
		}
		if applied {
			continue
		}
		script, err := migrationsFS.ReadFile("migrations/" + name)
		if err != nil {
			return fmt.Errorf("read migration %s: %w", name, err)
		}
		if err := s.applyMigration(ctx, name, string(script)); err != nil {
			return fmt.Errorf("apply migration %s: %w", name, err)
		}
		s.log.Info("applied migration", "version", name)
	}
	return nil
}

func (s *Store) migrationApplied(ctx context.Context, name string) (bool, error) {
	var n int
	if err := s.db.QueryRowContext(ctx,
		"SELECT COUNT(1) FROM schema_migrations WHERE version = ?", name,
	).Scan(&n); err != nil {
		return false, fmt.Errorf("check migration %s: %w", name, err)
	}
	return n > 0, nil
}

func (s *Store) applyMigration(ctx context.Context, name, script string) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, script); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx,
		"INSERT INTO schema_migrations (version, applied_at) VALUES (?, ?)", name, nowUTC(),
	); err != nil {
		return err
	}
	return tx.Commit()
}
