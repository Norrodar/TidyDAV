// Package proxy fetches upstream ICS feeds and caches the last good copy in the
// store so a dead upstream can still be served.
package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"time"

	"github.com/Norrodar/TidyDAV/internal/store"
)

// maxFeedSize caps how much we read from an upstream feed (25 MiB).
const maxFeedSize = 25 << 20

// Cache is the subset of the store the fetcher needs.
type Cache interface {
	GetCachedFeed(ctx context.Context, url string) (*store.CachedFeed, error)
	PutCachedFeed(ctx context.Context, cf *store.CachedFeed) error
}

// Source records where a fetched body came from.
type Source int

const (
	SourceNone Source = iota
	SourceUpstream
	SourceCacheFresh
	SourceCacheStale
)

func (s Source) String() string {
	switch s {
	case SourceUpstream:
		return "upstream"
	case SourceCacheFresh:
		return "cache-fresh"
	case SourceCacheStale:
		return "cache-stale"
	default:
		return "none"
	}
}

// Fetcher retrieves upstream feeds with caching.
type Fetcher struct {
	client *http.Client
	cache  Cache
	log    *slog.Logger
	now    func() time.Time
}

// NewFetcher creates a Fetcher backed by cache.
func NewFetcher(cache Cache, log *slog.Logger) *Fetcher {
	return &Fetcher{
		client: &http.Client{Timeout: 30 * time.Second},
		cache:  cache,
		log:    log,
		now:    time.Now,
	}
}

// Fetch is FetchAuth without upstream credentials.
func (f *Fetcher) Fetch(ctx context.Context, url string, ttl time.Duration) ([]byte, Source, error) {
	return f.FetchAuth(ctx, url, ttl, "", "")
}

// FetchAuth returns the ICS body for url, sending HTTP Basic Auth when username
// is non-empty. If the cached copy is younger than ttl it is returned without a
// network call. Otherwise the upstream is fetched (using ETag revalidation); on
// success the cache is refreshed, and on failure the last good cached copy is
// served (stale-on-error).
func (f *Fetcher) FetchAuth(ctx context.Context, url string, ttl time.Duration, username, password string) ([]byte, Source, error) {
	cached, err := f.cache.GetCachedFeed(ctx, url)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		return nil, SourceNone, fmt.Errorf("proxy: read cache: %w", err)
	}
	hasCache := err == nil

	if hasCache && ttl > 0 && f.now().Sub(cached.FetchedAt) < ttl {
		return cached.Body, SourceCacheFresh, nil
	}

	body, etag, fetchErr := f.fetchUpstream(ctx, url, cached, username, password)
	if fetchErr != nil {
		if hasCache {
			f.log.Warn("upstream fetch failed; serving stale cache", "url", url, "error", fetchErr)
			return cached.Body, SourceCacheStale, nil
		}
		return nil, SourceNone, fmt.Errorf("proxy: fetch %s: %w", url, fetchErr)
	}

	// A 304 yields a nil body: reuse the cached copy.
	if body == nil {
		if !hasCache {
			return nil, SourceNone, fmt.Errorf("proxy: empty response for %s", url)
		}
		body = cached.Body
	}

	put := &store.CachedFeed{URL: url, Body: body, ETag: etag, FetchedAt: f.now().UTC()}
	if err := f.cache.PutCachedFeed(ctx, put); err != nil {
		f.log.Warn("failed to update feed cache", "url", url, "error", err)
	}
	return body, SourceUpstream, nil
}

// fetchUpstream performs the HTTP GET. A 304 response returns a nil body and the
// existing ETag, signalling the caller to reuse the cached body.
func (f *Fetcher) fetchUpstream(ctx context.Context, url string, cached *store.CachedFeed, username, password string) ([]byte, string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, "", err
	}
	if username != "" {
		req.SetBasicAuth(username, password)
	}
	if cached != nil && cached.ETag != "" {
		req.Header.Set("If-None-Match", cached.ETag)
	}

	resp, err := f.client.Do(req)
	if err != nil {
		return nil, "", err
	}
	defer func() { _ = resp.Body.Close() }()

	switch resp.StatusCode {
	case http.StatusNotModified:
		if cached != nil {
			return nil, cached.ETag, nil
		}
		return nil, "", fmt.Errorf("unexpected 304 without cache")
	case http.StatusOK:
		body, err := io.ReadAll(io.LimitReader(resp.Body, maxFeedSize))
		if err != nil {
			return nil, "", err
		}
		return body, resp.Header.Get("ETag"), nil
	default:
		return nil, "", fmt.Errorf("unexpected status %d", resp.StatusCode)
	}
}
