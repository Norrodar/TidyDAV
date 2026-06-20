package store

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"
)

// SyncJob defines a DAV synchronisation between two collections.
type SyncJob struct {
	ID        string
	UserID    string
	Name      string
	Kind      string // caldav | carddav
	Direction string // a-to-b | b-to-a | bidirectional
	Conflict  string // newest-wins | source-wins

	AURL      string
	AUsername string
	APassword string
	BURL      string
	BUsername string
	BPassword string

	IntervalSeconds int
	Enabled         bool
	State           json.RawMessage
	LastRunAt       time.Time
	LastStatus      string

	CreatedAt time.Time
	UpdatedAt time.Time
}

const syncJobColumns = "id, user_id, name, kind, direction, conflict, " +
	"a_url, a_username, a_password, b_url, b_username, b_password, " +
	"interval_seconds, enabled, state, last_run_at, last_status, created_at, updated_at"

// CreateSyncJob inserts a new sync job.
func (s *Store) CreateSyncJob(ctx context.Context, j *SyncJob) error {
	now := time.Now().UTC()
	if j.CreatedAt.IsZero() {
		j.CreatedAt = now
	}
	j.UpdatedAt = now
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO sync_jobs (`+syncJobColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		j.ID, j.UserID, j.Name, j.Kind, j.Direction, j.Conflict,
		j.AURL, j.AUsername, j.APassword, j.BURL, j.BUsername, j.BPassword,
		j.IntervalSeconds, boolToInt(j.Enabled), notifOrEmpty(j.State),
		formatTime(j.LastRunAt), j.LastStatus,
		j.CreatedAt.Format(time.RFC3339), j.UpdatedAt.Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create sync job: %w", err)
	}
	return nil
}

// UpdateSyncJob updates a job's configuration (owner-scoped). State and run
// metadata are updated separately by UpdateSyncJobRun.
func (s *Store) UpdateSyncJob(ctx context.Context, j *SyncJob) error {
	j.UpdatedAt = time.Now().UTC()
	res, err := s.db.ExecContext(ctx,
		`UPDATE sync_jobs SET name = ?, kind = ?, direction = ?, conflict = ?,
		     a_url = ?, a_username = ?, a_password = ?, b_url = ?, b_username = ?, b_password = ?,
		     interval_seconds = ?, enabled = ?, updated_at = ?
		 WHERE id = ? AND user_id = ?`,
		j.Name, j.Kind, j.Direction, j.Conflict,
		j.AURL, j.AUsername, j.APassword, j.BURL, j.BUsername, j.BPassword,
		j.IntervalSeconds, boolToInt(j.Enabled), j.UpdatedAt.Format(time.RFC3339),
		j.ID, j.UserID,
	)
	if err != nil {
		return fmt.Errorf("update sync job: %w", err)
	}
	return checkAffected(res)
}

// UpdateSyncJobRun persists the sync state and last-run outcome of a job.
func (s *Store) UpdateSyncJobRun(ctx context.Context, id string, state json.RawMessage, lastRun time.Time, status string) error {
	_, err := s.db.ExecContext(ctx,
		`UPDATE sync_jobs SET state = ?, last_run_at = ?, last_status = ?, updated_at = ? WHERE id = ?`,
		notifOrEmpty(state), formatTime(lastRun), status, time.Now().UTC().Format(time.RFC3339), id,
	)
	if err != nil {
		return fmt.Errorf("update sync job run: %w", err)
	}
	return nil
}

// DeleteSyncJob removes a job owned by userID.
func (s *Store) DeleteSyncJob(ctx context.Context, id, userID string) error {
	res, err := s.db.ExecContext(ctx, "DELETE FROM sync_jobs WHERE id = ? AND user_id = ?", id, userID)
	if err != nil {
		return fmt.Errorf("delete sync job: %w", err)
	}
	return checkAffected(res)
}

// SyncJobByID returns a job by id, or ErrNotFound.
func (s *Store) SyncJobByID(ctx context.Context, id string) (*SyncJob, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+syncJobColumns+" FROM sync_jobs WHERE id = ?", id)
	j, err := scanSyncJob(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query sync job: %w", err)
	}
	return j, nil
}

// SyncJobsByUser lists a user's jobs, oldest first.
func (s *Store) SyncJobsByUser(ctx context.Context, userID string) ([]*SyncJob, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+syncJobColumns+" FROM sync_jobs WHERE user_id = ? ORDER BY created_at", userID)
	if err != nil {
		return nil, fmt.Errorf("query sync jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanSyncJobs(rows)
}

// AllEnabledSyncJobs returns every enabled job across users.
func (s *Store) AllEnabledSyncJobs(ctx context.Context) ([]*SyncJob, error) {
	rows, err := s.db.QueryContext(ctx,
		"SELECT "+syncJobColumns+" FROM sync_jobs WHERE enabled = 1 ORDER BY created_at")
	if err != nil {
		return nil, fmt.Errorf("query enabled sync jobs: %w", err)
	}
	defer func() { _ = rows.Close() }()
	return scanSyncJobs(rows)
}

func scanSyncJobs(rows *sql.Rows) ([]*SyncJob, error) {
	var jobs []*SyncJob
	for rows.Next() {
		j, err := scanSyncJob(rows)
		if err != nil {
			return nil, err
		}
		jobs = append(jobs, j)
	}
	return jobs, rows.Err()
}

func scanSyncJob(sc rowScanner) (*SyncJob, error) {
	var (
		j                         SyncJob
		enabled                   int64
		state                     string
		lastRun, created, updated string
	)
	if err := sc.Scan(&j.ID, &j.UserID, &j.Name, &j.Kind, &j.Direction, &j.Conflict,
		&j.AURL, &j.AUsername, &j.APassword, &j.BURL, &j.BUsername, &j.BPassword,
		&j.IntervalSeconds, &enabled, &state, &lastRun, &j.LastStatus, &created, &updated); err != nil {
		return nil, err
	}
	j.Enabled = enabled != 0
	j.State = json.RawMessage(state)
	j.LastRunAt = parseTime(lastRun)
	j.CreatedAt = parseTime(created)
	j.UpdatedAt = parseTime(updated)
	return &j, nil
}

func formatTime(t time.Time) string {
	if t.IsZero() {
		return ""
	}
	return t.UTC().Format(time.RFC3339)
}
