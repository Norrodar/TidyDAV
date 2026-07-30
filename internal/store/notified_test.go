package store

import (
	"context"
	"testing"
	"time"
)

func TestMarkNotified(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	first, err := st.MarkNotified(ctx, "feed1", "key1")
	if err != nil {
		t.Fatalf("MarkNotified: %v", err)
	}
	if !first {
		t.Error("first MarkNotified should report new")
	}

	if again, _ := st.MarkNotified(ctx, "feed1", "key1"); again {
		t.Error("repeat MarkNotified should report not-new")
	}
	if other, _ := st.MarkNotified(ctx, "feed1", "key2"); !other {
		t.Error("a different key should be new")
	}

	n, err := st.DeleteNotifiedBefore(ctx, time.Now().Add(time.Hour))
	if err != nil {
		t.Fatalf("DeleteNotifiedBefore: %v", err)
	}
	if n != 2 {
		t.Errorf("pruned %d rows, want 2", n)
	}
	if again, _ := st.MarkNotified(ctx, "feed1", "key1"); !again {
		t.Error("after pruning, the key should be new again")
	}
}

func TestAllFeeds(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	makeUser(t, st, "u1")
	makeUser(t, st, "u2")

	for _, f := range []*Feed{
		{ID: "a", UserID: "u1", Name: "A", Secret: "sa"},
		{ID: "b", UserID: "u2", Name: "B", Secret: "sb"},
	} {
		if err := st.CreateFeed(ctx, f); err != nil {
			t.Fatalf("CreateFeed: %v", err)
		}
	}

	all, err := st.AllFeeds(ctx)
	if err != nil {
		t.Fatalf("AllFeeds: %v", err)
	}
	if len(all) != 2 {
		t.Fatalf("AllFeeds len = %d, want 2", len(all))
	}
}

func TestIsNotified(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// A plain read must not create the entry it asks about.
	if ok, err := st.IsNotified(ctx, "feed1", "key1"); err != nil || ok {
		t.Fatalf("IsNotified on empty ledger = %v, %v; want false, nil", ok, err)
	}
	if first, _ := st.MarkNotified(ctx, "feed1", "key1"); !first {
		t.Error("IsNotified must not have inserted the key")
	}
	if ok, _ := st.IsNotified(ctx, "feed1", "key1"); !ok {
		t.Error("marked key should read back as notified")
	}
	if ok, _ := st.IsNotified(ctx, "feed2", "key1"); ok {
		t.Error("the ledger must be scoped per feed")
	}
	if err := st.UnmarkNotified(ctx, "feed1", "key1"); err != nil {
		t.Fatalf("UnmarkNotified: %v", err)
	}
	if ok, _ := st.IsNotified(ctx, "feed1", "key1"); ok {
		t.Error("unmarked key should read back as not notified")
	}
}
