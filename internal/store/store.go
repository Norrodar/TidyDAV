// Package store is the SQLite access layer. All SQL lives here; the rest of the
// app talks to typed methods. Uses modernc.org/sqlite (pure Go, no CGO).
package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"log/slog"
	"time"

	_ "modernc.org/sqlite"
)

// ErrNotFound is returned when a lookup matches no row.
var ErrNotFound = errors.New("store: not found")

// Store wraps a SQLite database handle.
type Store struct {
	db  *sql.DB
	log *slog.Logger
}

// Open opens (and pings) the SQLite database at path. Use ":memory:" for an
// ephemeral in-memory database (handy for tests).
func Open(ctx context.Context, path string, log *slog.Logger) (*Store, error) {
	db, err := sql.Open("sqlite", buildDSN(path))
	if err != nil {
		return nil, fmt.Errorf("open sqlite: %w", err)
	}
	// SQLite handles one writer at a time; a single connection avoids
	// "database is locked" entirely and keeps an in-memory DB alive.
	db.SetMaxOpenConns(1)
	db.SetConnMaxLifetime(0)

	pingCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	if err := db.PingContext(pingCtx); err != nil {
		_ = db.Close()
		return nil, fmt.Errorf("ping sqlite: %w", err)
	}
	return &Store{db: db, log: log}, nil
}

// Close closes the database handle.
func (s *Store) Close() error {
	return s.db.Close()
}

func buildDSN(path string) string {
	const pragmas = "_pragma=busy_timeout(5000)&_pragma=foreign_keys(1)"
	if path == ":memory:" || path == "" {
		return "file::memory:?cache=shared&" + pragmas
	}
	return fmt.Sprintf("file:%s?_pragma=journal_mode(WAL)&%s", path, pragmas)
}

func nowUTC() string {
	return time.Now().UTC().Format(time.RFC3339)
}
