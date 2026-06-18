package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// CachedFeed is the last successfully fetched copy of an upstream ICS feed.
type CachedFeed struct {
	URL       string
	Body      []byte
	ETag      string
	FetchedAt time.Time
}

// GetCachedFeed returns the cached feed for url, or ErrNotFound.
func (s *Store) GetCachedFeed(ctx context.Context, url string) (*CachedFeed, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT url, body, etag, fetched_at FROM feed_cache WHERE url = ?", url)

	var (
		cf      CachedFeed
		fetched string
	)
	err := row.Scan(&cf.URL, &cf.Body, &cf.ETag, &fetched)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query feed cache: %w", err)
	}
	cf.FetchedAt = parseTime(fetched)
	return &cf, nil
}

// PutCachedFeed inserts or updates the cached copy for a URL.
func (s *Store) PutCachedFeed(ctx context.Context, cf *CachedFeed) error {
	if cf.FetchedAt.IsZero() {
		cf.FetchedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO feed_cache (url, body, etag, fetched_at) VALUES (?, ?, ?, ?)
		 ON CONFLICT(url) DO UPDATE SET
		     body = excluded.body, etag = excluded.etag, fetched_at = excluded.fetched_at`,
		cf.URL, cf.Body, cf.ETag, cf.FetchedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("put feed cache: %w", err)
	}
	return nil
}
