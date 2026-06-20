package store

import (
	"context"
	"fmt"
	"time"
)

// MarkNotified records that (feedID, key) has been notified. It returns true the
// first time a key is seen — the caller should then send the notification — and
// false if it was already recorded.
func (s *Store) MarkNotified(ctx context.Context, feedID, key string) (bool, error) {
	res, err := s.db.ExecContext(ctx,
		`INSERT OR IGNORE INTO notified (feed_id, event_key, notified_at) VALUES (?, ?, ?)`,
		feedID, key, time.Now().UTC().Format(time.RFC3339))
	if err != nil {
		return false, fmt.Errorf("mark notified: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return false, fmt.Errorf("rows affected: %w", err)
	}
	return n > 0, nil
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
