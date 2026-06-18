package store

import (
	"context"
	"encoding/json"
	"errors"
	"testing"
)

func makeUser(t *testing.T, st *Store, id string) {
	t.Helper()
	if err := st.CreateUser(context.Background(), &User{ID: id, Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
}

func TestFeedCRUD(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	makeUser(t, st, "owner")

	f := &Feed{
		ID:         "feed-1",
		UserID:     "owner",
		Name:       "Müll",
		Secret:     "secret-abc",
		Sources:    []FeedSource{{URL: "https://up/feed.ics"}},
		Rules:      json.RawMessage(`[{"type":"dedup"}]`),
		TTLSeconds: 900,
	}
	if err := st.CreateFeed(ctx, f); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	got, err := st.FeedBySecret(ctx, "secret-abc")
	if err != nil {
		t.Fatalf("FeedBySecret: %v", err)
	}
	if got.Name != "Müll" || len(got.Sources) != 1 || got.Sources[0].URL != "https://up/feed.ics" {
		t.Errorf("unexpected feed: %+v", got)
	}
	if string(got.Rules) != `[{"type":"dedup"}]` {
		t.Errorf("rules = %s", got.Rules)
	}

	list, err := st.FeedsByUser(ctx, "owner")
	if err != nil || len(list) != 1 {
		t.Fatalf("FeedsByUser = %v, %v", list, err)
	}

	// Update.
	f.Name = "Renamed"
	if err := st.UpdateFeed(ctx, f); err != nil {
		t.Fatalf("UpdateFeed: %v", err)
	}
	got, _ = st.FeedByID(ctx, "feed-1")
	if got.Name != "Renamed" {
		t.Errorf("name after update = %q", got.Name)
	}

	// Update by a different owner must not match.
	other := *f
	other.UserID = "intruder"
	other.Name = "Hijacked"
	if err := st.UpdateFeed(ctx, &other); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner update err = %v, want ErrNotFound", err)
	}

	// Delete scoped to owner.
	if err := st.DeleteFeed(ctx, "feed-1", "intruder"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner delete err = %v, want ErrNotFound", err)
	}
	if err := st.DeleteFeed(ctx, "feed-1", "owner"); err != nil {
		t.Fatalf("DeleteFeed: %v", err)
	}
	if _, err := st.FeedByID(ctx, "feed-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("feed still present after delete: %v", err)
	}
}
