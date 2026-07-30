package audit

import (
	"context"
	"database/sql"
	"io"
	"log/slog"
	"path/filepath"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/store"
)

func newTestLogger(t *testing.T) (*Logger, *store.Store) {
	t.Helper()
	log := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "audit.db"), log)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return New(st, log), st
}

func TestRecordWritesEntry(t *testing.T) {
	l, st := newTestLogger(t)
	ctx := context.Background()
	user := &store.User{ID: "u1", Kind: "password", Email: sql.NullString{String: "a@example.com", Valid: true}}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	l.Record(ctx, user, "feed.create", "feed-1", "Waste calendar")

	entries, err := st.ListAuditEntries(ctx, 10)
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 1 {
		t.Fatalf("entry count = %d, want 1", len(entries))
	}
	e := entries[0]
	if e.Action != "feed.create" || e.Target != "feed-1" || e.Detail != "Waste calendar" {
		t.Errorf("unexpected entry: %+v", e)
	}
	if e.UserEmail != "a@example.com" {
		t.Errorf("userEmail = %q, want the user's address", e.UserEmail)
	}
	if e.CreatedAt.IsZero() {
		t.Error("createdAt was not set")
	}
}

// Secret-id users have no email; auditing must still work.
func TestRecordWithoutEmail(t *testing.T) {
	l, st := newTestLogger(t)
	ctx := context.Background()
	user := &store.User{ID: "u2", Kind: "secret"}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	l.Record(ctx, user, "feed.delete", "feed-9", "")

	entries, _ := st.ListAuditEntries(ctx, 10)
	if len(entries) != 1 || entries[0].UserEmail != "" {
		t.Errorf("entries = %+v, want one entry without an email", entries)
	}
}

// Auditing is best-effort: an anonymous action records nothing and never panics.
func TestRecordIgnoresNilUser(t *testing.T) {
	l, st := newTestLogger(t)
	ctx := context.Background()

	l.Record(ctx, nil, "feed.create", "feed-1", "")

	entries, _ := st.ListAuditEntries(ctx, 10)
	if len(entries) != 0 {
		t.Errorf("entry count = %d, want 0 for a nil user", len(entries))
	}
}

// A failing write must not propagate — the audited action already happened.
func TestRecordSurvivesStoreFailure(t *testing.T) {
	l, st := newTestLogger(t)
	ctx := context.Background()
	user := &store.User{ID: "u3", Kind: "password"}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// Closing the store makes every later write fail.
	if err := st.Close(); err != nil {
		t.Fatalf("Close: %v", err)
	}

	l.Record(ctx, user, "feed.update", "feed-1", "after close") // must not panic
}

func TestListAuditEntriesIsNewestFirst(t *testing.T) {
	l, st := newTestLogger(t)
	ctx := context.Background()
	user := &store.User{ID: "u4", Kind: "password"}
	if err := st.CreateUser(ctx, user); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}

	for _, target := range []string{"first", "second", "third"} {
		l.Record(ctx, user, "feed.create", target, "")
	}

	entries, err := st.ListAuditEntries(ctx, 10)
	if err != nil {
		t.Fatalf("ListAuditEntries: %v", err)
	}
	if len(entries) != 3 {
		t.Fatalf("entry count = %d, want 3", len(entries))
	}
	if entries[0].Target != "third" {
		t.Errorf("newest entry = %q, want third", entries[0].Target)
	}

	// The limit is honoured.
	limited, err := st.ListAuditEntries(ctx, 2)
	if err != nil {
		t.Fatalf("ListAuditEntries(2): %v", err)
	}
	if len(limited) != 2 {
		t.Errorf("limited count = %d, want 2", len(limited))
	}
}
