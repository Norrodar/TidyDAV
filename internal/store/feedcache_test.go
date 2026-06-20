package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestFeedCacheRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if _, err := st.GetCachedFeed(ctx, "https://x/feed.ics"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("empty cache error = %v, want ErrNotFound", err)
	}

	cf := &CachedFeed{
		URL:       "https://x/feed.ics",
		Body:      []byte("BEGIN:VCALENDAR"),
		ETag:      "v1",
		FetchedAt: time.Now().UTC().Truncate(time.Second),
	}
	if err := st.PutCachedFeed(ctx, cf); err != nil {
		t.Fatalf("PutCachedFeed: %v", err)
	}

	got, err := st.GetCachedFeed(ctx, cf.URL)
	if err != nil {
		t.Fatalf("GetCachedFeed: %v", err)
	}
	if string(got.Body) != "BEGIN:VCALENDAR" || got.ETag != "v1" {
		t.Errorf("unexpected cached feed: %+v", got)
	}
	if !got.FetchedAt.Equal(cf.FetchedAt) {
		t.Errorf("FetchedAt = %v, want %v", got.FetchedAt, cf.FetchedAt)
	}

	// Upsert overwrites.
	cf.Body = []byte("UPDATED")
	cf.ETag = "v2"
	if err := st.PutCachedFeed(ctx, cf); err != nil {
		t.Fatalf("PutCachedFeed (update): %v", err)
	}
	got, _ = st.GetCachedFeed(ctx, cf.URL)
	if string(got.Body) != "UPDATED" || got.ETag != "v2" {
		t.Errorf("after upsert: %+v", got)
	}
}
