package proxy

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Norrodar/TidyDAV/internal/store"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

type fakeCache struct {
	m map[string]*store.CachedFeed
}

func newFakeCache() *fakeCache { return &fakeCache{m: map[string]*store.CachedFeed{}} }

func (c *fakeCache) GetCachedFeed(_ context.Context, url string) (*store.CachedFeed, error) {
	if v, ok := c.m[url]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, store.ErrNotFound
}

func (c *fakeCache) PutCachedFeed(_ context.Context, cf *store.CachedFeed) error {
	cp := *cf
	c.m[cf.URL] = &cp
	return nil
}

func TestFetchFreshCacheSkipsUpstream(t *testing.T) {
	var hits int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
		_, _ = w.Write([]byte("UPSTREAM"))
	}))
	defer srv.Close()

	cache := newFakeCache()
	cache.m[srv.URL] = &store.CachedFeed{URL: srv.URL, Body: []byte("CACHED"), FetchedAt: time.Now()}

	body, src, err := NewFetcher(cache, testLogger()).Fetch(context.Background(), srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "CACHED" || src != SourceCacheFresh {
		t.Errorf("got (%q, %s), want (CACHED, cache-fresh)", body, src)
	}
	if atomic.LoadInt32(&hits) != 0 {
		t.Errorf("upstream hit %d times, want 0", hits)
	}
}

func TestFetchUpstreamPopulatesCache(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.Header().Set("ETag", "v1")
		_, _ = w.Write([]byte("UPSTREAM"))
	}))
	defer srv.Close()

	cache := newFakeCache()
	body, src, err := NewFetcher(cache, testLogger()).Fetch(context.Background(), srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "UPSTREAM" || src != SourceUpstream {
		t.Errorf("got (%q, %s), want (UPSTREAM, upstream)", body, src)
	}
	if cached := cache.m[srv.URL]; cached == nil || string(cached.Body) != "UPSTREAM" || cached.ETag != "v1" {
		t.Errorf("cache not populated correctly: %+v", cached)
	}
}

func TestFetchStaleOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	cache := newFakeCache()
	cache.m[srv.URL] = &store.CachedFeed{
		URL: srv.URL, Body: []byte("STALE"), FetchedAt: time.Now().Add(-time.Hour),
	}

	body, src, err := NewFetcher(cache, testLogger()).Fetch(context.Background(), srv.URL, time.Minute)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "STALE" || src != SourceCacheStale {
		t.Errorf("got (%q, %s), want (STALE, cache-stale)", body, src)
	}
}

func TestFetchNoCacheError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, _, err := NewFetcher(newFakeCache(), testLogger()).Fetch(context.Background(), srv.URL, time.Minute); err == nil {
		t.Fatal("expected error when upstream fails and cache is empty")
	}
}

func TestFetch304ReusesBody(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Header.Get("If-None-Match") == "v1" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Set("ETag", "v1")
		_, _ = w.Write([]byte("FRESH"))
	}))
	defer srv.Close()

	cache := newFakeCache()
	cache.m[srv.URL] = &store.CachedFeed{
		URL: srv.URL, Body: []byte("OLD"), ETag: "v1", FetchedAt: time.Now().Add(-time.Hour),
	}

	body, src, err := NewFetcher(cache, testLogger()).Fetch(context.Background(), srv.URL, time.Minute)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "OLD" || src != SourceUpstream {
		t.Errorf("got (%q, %s), want (OLD, upstream via 304)", body, src)
	}
}
