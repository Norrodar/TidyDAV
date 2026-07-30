package store

import (
	"context"
	"fmt"
	"time"
)

// MarkNotified records that (feedID, key) has been notified. It returns true the
// first time a key is seen — the caller should then send the notification — and
// false if it was already recorded.
//
// A repeat sighting refreshes the timestamp so that an event which stays in the
// feed (a recurring series, a date far ahead) is not pruned by retention and
// then announced all over again.
func (s *Store) MarkNotified(ctx context.Context, feedID, key string) (bool, error) {
	now := time.Now().UTC().Format(time.RFC3339)
	res, err := s.db.ExecContext(ctx,
		`UPDATE notified SET notified_at = ? WHERE feed_id = ? AND event_key = ?`,
		now, feedID, key)
	if err != nil {
		return false, fmt.Errorf("refresh notified: %w", err)
	}
	if n, err := res.RowsAffected(); err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	} else if n > 0 {
		return false, nil // already known
	}

	res, err = s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO notified (feed_id, event_key, notified_at) VALUES (?, ?, ?)`,
		feedID, key, now)
	if err != nil {
		return false, fmt.Errorf("mark notified: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
}

// IsNotified reports whether (feedID, key) is currently recorded in the ledger,
// without creating it. Callers that only want to know the previous state — e.g.
// "was an outage announced for this source?" — must use this instead of
// MarkNotified, which would insert the very entry it is asked about.
func (s *Store) IsNotified(ctx context.Context, feedID, key string) (bool, error) {
	row := s.db.QueryRowContext(ctx,
		`SELECT EXISTS(SELECT 1 FROM notified WHERE feed_id = ? AND event_key = ?)`, feedID, key)
	var exists bool
	if err := row.Scan(&exists); err != nil {
		return false, fmt.Errorf("query notified: %w", err)
	}
	return exists, nil
}

// UnmarkNotified removes a ledger entry again, so a notification that could not
// be delivered to any target is retried on the next run instead of being lost.
func (s *Store) UnmarkNotified(ctx context.Context, feedID, key string) error {
	_, err := s.db.ExecContext(ctx,
		`DELETE FROM notified WHERE feed_id = ? AND event_key = ?`, feedID, key)
	if err != nil {
		return fmt.Errorf("unmark notified: %w", err)
	}
	return nil
}

// DeleteNotifiedBefore prunes notified rows older than cutoff and returns how
// many were removed.
func (s *Store) DeleteNotifiedBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		`DELETE FROM notified WHERE notified_at < ?`, cutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("prune notified: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
