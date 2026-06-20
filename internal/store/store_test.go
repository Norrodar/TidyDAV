package store

import (
	"context"
	"database/sql"
	"errors"
	"io"
	"log/slog"
	"path/filepath"
	"testing"
	"time"
)

func newTestStore(t *testing.T) *Store {
	t.Helper()
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "test.db")
	st, err := Open(ctx, path, slog.New(slog.NewTextHandler(io.Discard, nil)))
	if err != nil {
		t.Fatalf("Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}
	return st
}

func TestMigrateIsIdempotent(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	// Second run must be a no-op without error.
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("second Migrate() error: %v", err)
	}

	var count int
	if err := st.db.QueryRowContext(ctx, "SELECT COUNT(1) FROM schema_migrations").Scan(&count); err != nil {
		t.Fatalf("count migrations: %v", err)
	}
	if count < 1 {
		t.Fatalf("schema_migrations has %d rows, want >= 1", count)
	}
}

func TestUserRoundTrip(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	u := &User{
		ID:           "user-1",
		Kind:         "password",
		Email:        sql.NullString{String: "a@example.com", Valid: true},
		PasswordHash: sql.NullString{String: "hash", Valid: true},
		IsAdmin:      true,
	}
	if err := st.CreateUser(ctx, u); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	got, err := st.UserByEmail(ctx, "a@example.com")
	if err != nil {
		t.Fatalf("UserByEmail() error: %v", err)
	}
	if got.ID != "user-1" || got.Kind != "password" || !got.IsAdmin {
		t.Errorf("unexpected user: %+v", got)
	}
	if got.CreatedAt.IsZero() {
		t.Error("CreatedAt was not set")
	}

	byID, err := st.UserByID(ctx, "user-1")
	if err != nil {
		t.Fatalf("UserByID() error: %v", err)
	}
	if byID.Email.String != "a@example.com" {
		t.Errorf("UserByID email = %q", byID.Email.String)
	}
}

func TestUserNotFound(t *testing.T) {
	st := newTestStore(t)
	if _, err := st.UserByEmail(context.Background(), "missing@example.com"); !errors.Is(err, ErrNotFound) {
		t.Fatalf("error = %v, want ErrNotFound", err)
	}
}

func TestSessionLifecycle(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()

	if err := st.CreateUser(ctx, &User{ID: "u", Kind: "secret", SecretHash: sql.NullString{String: "h", Valid: true}}); err != nil {
		t.Fatalf("CreateUser() error: %v", err)
	}

	live := &Session{ID: "sess-live", UserID: "u", ExpiresAt: time.Now().Add(time.Hour)}
	expired := &Session{ID: "sess-old", UserID: "u", ExpiresAt: time.Now().Add(-time.Hour)}
	for _, sess := range []*Session{live, expired} {
		if err := st.CreateSession(ctx, sess); err != nil {
			t.Fatalf("CreateSession(%s) error: %v", sess.ID, err)
		}
	}

	got, err := st.SessionByID(ctx, "sess-live")
	if err != nil {
		t.Fatalf("SessionByID() error: %v", err)
	}
	if got.UserID != "u" {
		t.Errorf("session UserID = %q, want u", got.UserID)
	}

	n, err := st.DeleteExpiredSessions(ctx, time.Now())
	if err != nil {
		t.Fatalf("DeleteExpiredSessions() error: %v", err)
	}
	if n != 1 {
		t.Errorf("deleted %d expired sessions, want 1", n)
	}
	if _, err := st.SessionByID(ctx, "sess-old"); !errors.Is(err, ErrNotFound) {
		t.Errorf("expired session still present: %v", err)
	}

	if err := st.DeleteSession(ctx, "sess-live"); err != nil {
		t.Fatalf("DeleteSession() error: %v", err)
	}
	if _, err := st.SessionByID(ctx, "sess-live"); !errors.Is(err, ErrNotFound) {
		t.Errorf("session not deleted: %v", err)
	}
}
