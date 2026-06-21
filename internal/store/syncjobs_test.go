package store

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestSyncJobCRUD(t *testing.T) {
	st := newTestStore(t)
	ctx := context.Background()
	makeUser(t, st, "owner")

	j := &SyncJob{
		ID: "job-1", UserID: "owner", Name: "Cal", Kind: "caldav",
		Direction: "a-to-b", Conflict: "newest-wins",
		AURL: "https://a/cal", BURL: "https://b/cal",
		IntervalSeconds: 900, Enabled: true,
		WindowStart: "2026-01-01", WindowEnd: "2026-12-31",
	}
	if err := st.CreateSyncJob(ctx, j); err != nil {
		t.Fatalf("CreateSyncJob: %v", err)
	}

	got, err := st.SyncJobByID(ctx, "job-1")
	if err != nil {
		t.Fatalf("SyncJobByID: %v", err)
	}
	if got.Name != "Cal" || got.Kind != "caldav" || !got.Enabled || got.AURL != "https://a/cal" {
		t.Errorf("unexpected job: %+v", got)
	}
	if got.WindowStart != "2026-01-01" || got.WindowEnd != "2026-12-31" {
		t.Errorf("window not persisted: start=%q end=%q", got.WindowStart, got.WindowEnd)
	}

	// Window survives a config update.
	j.WindowEnd = "2027-06-30"
	if err := st.UpdateSyncJob(ctx, j); err != nil {
		t.Fatalf("UpdateSyncJob: %v", err)
	}
	if got, _ = st.SyncJobByID(ctx, "job-1"); got.WindowEnd != "2027-06-30" {
		t.Errorf("window end after update = %q, want 2027-06-30", got.WindowEnd)
	}

	if list, _ := st.SyncJobsByUser(ctx, "owner"); len(list) != 1 {
		t.Fatalf("SyncJobsByUser len = %d, want 1", len(list))
	}
	if enabled, _ := st.AllEnabledSyncJobs(ctx); len(enabled) != 1 {
		t.Fatalf("AllEnabledSyncJobs len = %d, want 1", len(enabled))
	}

	// Persist run outcome.
	if err := st.UpdateSyncJobRun(ctx, "job-1", []byte(`{"items":{}}`), time.Now(), "ok: +1 ~0 -0"); err != nil {
		t.Fatalf("UpdateSyncJobRun: %v", err)
	}
	got, _ = st.SyncJobByID(ctx, "job-1")
	if got.LastStatus != "ok: +1 ~0 -0" || got.LastRunAt.IsZero() {
		t.Errorf("run not persisted: %+v", got)
	}

	// Disabling removes it from the enabled set.
	j.Enabled = false
	if err := st.UpdateSyncJob(ctx, j); err != nil {
		t.Fatalf("UpdateSyncJob: %v", err)
	}
	if enabled, _ := st.AllEnabledSyncJobs(ctx); len(enabled) != 0 {
		t.Errorf("disabled job still listed as enabled")
	}

	// Owner-scoped delete.
	if err := st.DeleteSyncJob(ctx, "job-1", "intruder"); !errors.Is(err, ErrNotFound) {
		t.Errorf("cross-owner delete = %v, want ErrNotFound", err)
	}
	if err := st.DeleteSyncJob(ctx, "job-1", "owner"); err != nil {
		t.Fatalf("DeleteSyncJob: %v", err)
	}
	if _, err := st.SyncJobByID(ctx, "job-1"); !errors.Is(err, ErrNotFound) {
		t.Errorf("job still present after delete")
	}
}
