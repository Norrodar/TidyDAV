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

func (c *fakeCache) GetCachedFeed(_ context.Context, key string) (*store.CachedFeed, error) {
	if v, ok := c.m[key]; ok {
		cp := *v
		return &cp, nil
	}
	return nil, store.ErrNotFound
}

func (c *fakeCache) PutCachedFeed(_ context.Context, cf *store.CachedFeed) error {
	cp := *cf
	key := cf.Key
	if key == "" {
		key = cf.URL
	}
	c.m[key] = &cp
	return nil
}

// A copy fetched with credentials must never be handed to a request that
// supplies none — neither as a fresh hit nor as stale-on-error — otherwise one
// user could read another's private calendar just by knowing the URL.
func TestCacheIsScopedToCredentials(t *testing.T) {
	var unauthorized int32
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		user, pass, _ := r.BasicAuth()
		if user != "alice" || pass != "s3cret" {
			atomic.AddInt32(&unauthorized, 1)
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = w.Write([]byte("PRIVATE"))
	}))
	defer srv.Close()

	cache := newFakeCache()
	f := NewFetcher(cache, testLogger(), true)
	ctx := context.Background()

	// The owner populates the cache.
	body, _, err := f.FetchAuth(ctx, srv.URL, time.Hour, "alice", "s3cret")
	if err != nil || string(body) != "PRIVATE" {
		t.Fatalf("owner fetch = %q, %v", body, err)
	}

	// Another user asks for the same URL without credentials: a fresh-cache hit
	// must not happen, and the 401 must not fall back to the cached body.
	if _, _, err := f.FetchAuth(ctx, srv.URL, time.Hour, "", ""); err == nil {
		t.Fatal("anonymous fetch succeeded; the cached private body leaked")
	}
	if atomic.LoadInt32(&unauthorized) == 0 {
		t.Error("anonymous request never reached upstream — it was served from cache")
	}

	// Wrong credentials must not reach the cached copy either.
	if _, _, err := f.FetchAuth(ctx, srv.URL, time.Hour, "mallory", "guess"); err == nil {
		t.Error("fetch with wrong credentials succeeded; cached body leaked")
	}

	// The owner still gets a cached hit.
	if _, src, err := f.FetchAuth(ctx, srv.URL, time.Hour, "alice", "s3cret"); err != nil || src != SourceCacheFresh {
		t.Errorf("owner refetch = source %v, %v; want fresh cache hit", src, err)
	}
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

	body, src, err := NewFetcher(cache, testLogger(), true).Fetch(context.Background(), srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "CACHED" || src != SourceCacheFresh {
		t.Errorf("got (%q, %v), want (CACHED, SourceCacheFresh)", body, src)
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
	body, src, err := NewFetcher(cache, testLogger(), true).Fetch(context.Background(), srv.URL, time.Hour)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "UPSTREAM" || src != SourceUpstream {
		t.Errorf("got (%q, %v), want (UPSTREAM, SourceUpstream)", body, src)
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

	body, src, err := NewFetcher(cache, testLogger(), true).Fetch(context.Background(), srv.URL, time.Minute)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "STALE" || src != SourceCacheStale {
		t.Errorf("got (%q, %v), want (STALE, SourceCacheStale)", body, src)
	}
}

func TestFetchNoCacheError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	if _, _, err := NewFetcher(newFakeCache(), testLogger(), true).Fetch(context.Background(), srv.URL, time.Minute); err == nil {
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

	body, src, err := NewFetcher(cache, testLogger(), true).Fetch(context.Background(), srv.URL, time.Minute)
	if err != nil {
		t.Fatalf("Fetch: %v", err)
	}
	if string(body) != "OLD" || src != SourceUpstream {
		t.Errorf("got (%q, %v), want (OLD, SourceUpstream via 304)", body, src)
	}
}

func TestFetchBlocksPrivateTargets(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("SECRET"))
	}))
	defer srv.Close()

	// With the guard on, the loopback httptest address must be refused.
	f := NewFetcher(newFakeCache(), testLogger(), false)
	if _, _, err := f.Fetch(context.Background(), srv.URL, 0); err == nil {
		t.Fatal("expected refusal to connect to a loopback address")
	}
}
