package store

import (
	"context"
	"fmt"
	"time"
)

// AuditEntry records a configuration change made by a user.
type AuditEntry struct {
	ID        int64
	UserID    string
	UserEmail string
	Action    string
	Target    string
	Detail    string
	CreatedAt time.Time
}

// AddAuditEntry appends an audit entry.
func (s *Store) AddAuditEntry(ctx context.Context, e *AuditEntry) error {
	if e.CreatedAt.IsZero() {
		e.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO audit_log (user_id, user_email, action, target, detail, created_at)
		 VALUES (?, ?, ?, ?, ?, ?)`,
		e.UserID, e.UserEmail, e.Action, e.Target, e.Detail, e.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("add audit entry: %w", err)
	}
	return nil
}

// ListAuditEntries returns the most recent entries, newest first.
func (s *Store) ListAuditEntries(ctx context.Context, limit int) ([]*AuditEntry, error) {
	if limit <= 0 || limit > 1000 {
		limit = 200
	}
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, user_id, user_email, action, target, detail, created_at
		 FROM audit_log ORDER BY id DESC LIMIT ?`, limit)
	if err != nil {
		return nil, fmt.Errorf("list audit entries: %w", err)
	}
	defer func() { _ = rows.Close() }()

	var out []*AuditEntry
	for rows.Next() {
		var (
			e       AuditEntry
			created string
		)
		if err := rows.Scan(&e.ID, &e.UserID, &e.UserEmail, &e.Action, &e.Target, &e.Detail, &created); err != nil {
			return nil, err
		}
		e.CreatedAt = parseTime(created)
		out = append(out, &e)
	}
	return out, rows.Err()
}
