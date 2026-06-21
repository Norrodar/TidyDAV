// Package davsync runs DAV sync jobs on a schedule, building the right
// CalDAV/CardDAV collections and persisting sync state and outcomes.
package davsync

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Norrodar/TidyDAV/internal/dav"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// Runner executes due sync jobs.
type Runner struct {
	store *store.Store
	log   *slog.Logger
}

// New creates a Runner.
func New(st *store.Store, log *slog.Logger) *Runner {
	return &Runner{store: st, log: log}
}

// Run executes every enabled job whose interval has elapsed since its last run.
func (r *Runner) Run(ctx context.Context) error {
	jobs, err := r.store.AllEnabledSyncJobs(ctx)
	if err != nil {
		return err
	}
	now := time.Now()
	for _, job := range jobs {
		if due(job, now) {
			r.runJob(ctx, job)
		}
	}
	return nil
}

func due(job *store.SyncJob, now time.Time) bool {
	if job.LastRunAt.IsZero() {
		return true
	}
	interval := time.Duration(job.IntervalSeconds) * time.Second
	if interval <= 0 {
		interval = 15 * time.Minute
	}
	return now.Sub(job.LastRunAt) >= interval
}

// RunOne executes a single job immediately, ignoring its interval. Used by the
// manual-run API.
func (r *Runner) RunOne(ctx context.Context, job *store.SyncJob) {
	r.runJob(ctx, job)
}

func (r *Runner) runJob(ctx context.Context, job *store.SyncJob) {
	a, b, opts, err := buildSync(job)
	if err != nil {
		r.finish(ctx, job, nil, "config error: "+err.Error())
		return
	}

	state := &dav.State{Items: map[string]dav.ItemState{}}
	if len(job.State) > 0 {
		if err := json.Unmarshal(job.State, state); err != nil {
			r.log.Warn("decode sync state", "job", job.ID, "error", err)
			state = &dav.State{Items: map[string]dav.ItemState{}}
		}
	}

	res, err := dav.Sync(ctx, a, b, state, opts)
	if err != nil {
		r.finish(ctx, job, state, "error: "+err.Error())
		return
	}
	r.finish(ctx, job, state, fmt.Sprintf("ok: +%d ~%d -%d", res.Created, res.Updated, res.Deleted))
}

func (r *Runner) finish(ctx context.Context, job *store.SyncJob, state *dav.State, status string) {
	var raw json.RawMessage
	if state != nil {
		if b, err := json.Marshal(state); err == nil {
			raw = b
		}
	}
	if err := r.store.UpdateSyncJobRun(ctx, job.ID, raw, time.Now().UTC(), status); err != nil {
		r.log.Warn("persist sync job run", "job", job.ID, "error", err)
	}
	r.log.Info("sync job finished", "job", job.ID, "name", job.Name, "status", status)
}

// buildSync resolves a job into two collections and sync options.
func buildSync(job *store.SyncJob) (dav.Collection, dav.Collection, dav.Options, error) {
	var opts dav.Options
	switch dav.Direction(job.Direction) {
	case dav.AToB, dav.BToA, dav.Bidirectional:
		opts.Direction = dav.Direction(job.Direction)
	default:
		return nil, nil, opts, fmt.Errorf("invalid direction %q", job.Direction)
	}
	switch dav.Conflict(job.Conflict) {
	case dav.NewestWins, dav.SourceWins, "":
		opts.Conflict = dav.Conflict(job.Conflict)
	default:
		return nil, nil, opts, fmt.Errorf("invalid conflict %q", job.Conflict)
	}

	var a, b dav.Collection
	var err error
	switch job.Kind {
	case "caldav":
		opts.UID, opts.Modified, opts.HrefSuffix = dav.CalendarUID, dav.CalendarModified, ".ics"
		if opts.WindowStart, opts.WindowEnd, err = dav.ParseWindow(job.WindowStart, job.WindowEnd); err != nil {
			return nil, nil, opts, err
		}
		if a, err = dav.NewCalDAVCollection(job.AURL, job.AUsername, job.APassword); err != nil {
			return nil, nil, opts, err
		}
		if b, err = dav.NewCalDAVCollection(job.BURL, job.BUsername, job.BPassword); err != nil {
			return nil, nil, opts, err
		}
	case "carddav":
		opts.UID, opts.Modified, opts.HrefSuffix = dav.ContactUID, dav.ContactModified, ".vcf"
		if a, err = dav.NewCardDAVCollection(job.AURL, job.AUsername, job.APassword); err != nil {
			return nil, nil, opts, err
		}
		if b, err = dav.NewCardDAVCollection(job.BURL, job.BUsername, job.BPassword); err != nil {
			return nil, nil, opts, err
		}
	default:
		return nil, nil, opts, fmt.Errorf("invalid kind %q", job.Kind)
	}
	return a, b, opts, nil
}
