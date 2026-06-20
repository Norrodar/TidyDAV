package davsync

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/store"
)

func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "s.db"), logger())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return st
}

func TestRunnerRecordsErrorOnUnreachable(t *testing.T) {
	// A server that is immediately closed -> connections are refused.
	srv := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {}))
	url := srv.URL
	srv.Close()

	st := newStore(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := st.CreateSyncJob(ctx, &store.SyncJob{
		ID: "j", UserID: "u", Name: "J", Kind: "caldav",
		Direction: "a-to-b", Conflict: "newest-wins",
		AURL: url + "/cal/", BURL: url + "/cal2/",
		IntervalSeconds: 1, Enabled: true,
	}); err != nil {
		t.Fatalf("CreateSyncJob: %v", err)
	}

	if err := New(st, logger()).Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	job, err := st.SyncJobByID(ctx, "j")
	if err != nil {
		t.Fatalf("SyncJobByID: %v", err)
	}
	if !strings.HasPrefix(job.LastStatus, "error") && !strings.HasPrefix(job.LastStatus, "config") {
		t.Errorf("status = %q, want an error status", job.LastStatus)
	}
	if job.LastRunAt.IsZero() {
		t.Error("last run time not recorded")
	}
}

func TestRunnerInvalidKind(t *testing.T) {
	st := newStore(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := st.CreateSyncJob(ctx, &store.SyncJob{
		ID: "j", UserID: "u", Name: "J", Kind: "bogus",
		Direction: "a-to-b", AURL: "https://a", BURL: "https://b", Enabled: true,
	}); err != nil {
		t.Fatalf("CreateSyncJob: %v", err)
	}

	if err := New(st, logger()).Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	job, _ := st.SyncJobByID(ctx, "j")
	if !strings.HasPrefix(job.LastStatus, "config error") {
		t.Errorf("status = %q, want config error", job.LastStatus)
	}
}
