package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// Session is a server-side login session referenced by an opaque cookie token.
type Session struct {
	ID        string
	UserID    string
	CreatedAt time.Time
	ExpiresAt time.Time
}

// CreateSession inserts a new session row.
func (s *Store) CreateSession(ctx context.Context, sess *Session) error {
	if sess.CreatedAt.IsZero() {
		sess.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sessions (id, user_id, created_at, expires_at) VALUES (?, ?, ?, ?)`,
		sess.ID, sess.UserID,
		sess.CreatedAt.UTC().Format(time.RFC3339),
		sess.ExpiresAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create session: %w", err)
	}
	return nil
}

// SessionByID returns the session with the given id, or ErrNotFound.
func (s *Store) SessionByID(ctx context.Context, id string) (*Session, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT id, user_id, created_at, expires_at FROM sessions WHERE id = ?", id)

	var (
		sess               Session
		createdAt, expires string
	)
	err := row.Scan(&sess.ID, &sess.UserID, &createdAt, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query session: %w", err)
	}
	sess.CreatedAt = parseTime(createdAt)
	sess.ExpiresAt = parseTime(expires)
	return &sess, nil
}

// DeleteSession removes a session by id. Removing a missing session is not an error.
func (s *Store) DeleteSession(ctx context.Context, id string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM sessions WHERE id = ?", id); err != nil {
		return fmt.Errorf("delete session: %w", err)
	}
	return nil
}

// DeleteExpiredSessions removes all sessions that expired at or before now and
// returns the number deleted.
func (s *Store) DeleteExpiredSessions(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM sessions WHERE expires_at <= ?", now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("delete expired sessions: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
