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
