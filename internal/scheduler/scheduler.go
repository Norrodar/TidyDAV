// Package scheduler runs registered jobs at fixed intervals inside the process.
package scheduler

import (
	"context"
	"log/slog"
	"sync"
	"time"
)

// Job is a unit of recurring work.
type Job struct {
	Name     string
	Interval time.Duration
	Run      func(ctx context.Context) error
}

// Scheduler runs jobs on their intervals until stopped.
type Scheduler struct {
	log    *slog.Logger
	jobs   []Job
	cancel context.CancelFunc
	wg     sync.WaitGroup
}

// New creates an empty scheduler.
func New(log *slog.Logger) *Scheduler {
	return &Scheduler{log: log}
}

// Add registers a job. Jobs with a non-positive interval are ignored.
func (s *Scheduler) Add(job Job) {
	if job.Interval <= 0 || job.Run == nil {
		return
	}
	s.jobs = append(s.jobs, job)
}

// Len returns the number of registered jobs.
func (s *Scheduler) Len() int { return len(s.jobs) }

// Start launches each job in its own goroutine. Start returns immediately.
func (s *Scheduler) Start(parent context.Context) {
	ctx, cancel := context.WithCancel(parent)
	s.cancel = cancel
	for _, job := range s.jobs {
		s.wg.Add(1)
		go s.loop(ctx, job)
	}
}

// Stop cancels all jobs and waits for them to return.
func (s *Scheduler) Stop() {
	if s.cancel != nil {
		s.cancel()
	}
	s.wg.Wait()
}

func (s *Scheduler) loop(ctx context.Context, job Job) {
	defer s.wg.Done()
	ticker := time.NewTicker(job.Interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			// A failing job is logged but never stops the scheduler.
			if err := job.Run(ctx); err != nil {
				s.log.Warn("scheduled job failed", "job", job.Name, "error", err)
			}
		}
	}
}
