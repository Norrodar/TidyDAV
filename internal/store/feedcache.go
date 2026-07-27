package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CachedFeed is the last successfully fetched copy of an upstream ICS feed.
// Key identifies the (URL, credentials) pair it was fetched with; URL is kept
// separately so health lookups can find a source regardless of credentials.
type CachedFeed struct {
	Key       string
	URL       string
	Body      []byte
	ETag      string
	FetchedAt time.Time
}

// GetCachedFeed returns the cached feed for key, or ErrNotFound.
func (s *Store) GetCachedFeed(ctx context.Context, key string) (*CachedFeed, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT cache_key, url, body, etag, fetched_at FROM feed_cache WHERE cache_key = ?", key)

	var (
		cf      CachedFeed
		fetched string
	)
	err := row.Scan(&cf.Key, &cf.URL, &cf.Body, &cf.ETag, &fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query feed cache: %w", err)
	}
	cf.FetchedAt = parseTime(fetched)
	return &cf, nil
}

// CachedFeedFetchedAt returns when a copy of url was last fetched successfully
// (whatever credentials were used), or the zero time when it never was.
func (s *Store) CachedFeedFetchedAt(ctx context.Context, url string) (time.Time, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT fetched_at FROM feed_cache WHERE url = ? ORDER BY fetched_at DESC LIMIT 1", url)
	var fetched string
	err := row.Scan(&fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return time.Time{}, nil
	}
	if err != nil {
		return time.Time{}, fmt.Errorf("query feed cache fetched_at: %w", err)
	}
	return parseTime(fetched), nil
}

// PutCachedFeed inserts or updates the cached copy for a cache key.
func (s *Store) PutCachedFeed(ctx context.Context, cf *CachedFeed) error {
	if cf.FetchedAt.IsZero() {
		cf.FetchedAt = time.Now().UTC()
	}
	if cf.Key == "" {
		cf.Key = cf.URL
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feed_cache (cache_key, url, body, etag, fetched_at) VALUES (?, ?, ?, ?, ?)
		 ON CONFLICT(cache_key) DO UPDATE SET
		     url = excluded.url, body = excluded.body, etag = excluded.etag,
		     fetched_at = excluded.fetched_at`,
		cf.Key, cf.URL, cf.Body, cf.ETag, cf.FetchedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("put feed cache: %w", err)
	}
	return nil
}

// DeleteCachedFeedsBefore prunes cache entries not refreshed since cutoff and
// returns how many were removed, so bodies of removed feeds/sources (up to
// 25 MiB each) do not accumulate forever.
func (s *Store) DeleteCachedFeedsBefore(ctx context.Context, cutoff time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM feed_cache WHERE fetched_at < ?", cutoff.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("prune feed cache: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}
