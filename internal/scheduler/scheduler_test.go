package scheduler

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

// waitFor polls until runs reaches want or the deadline elapses.
func waitFor(t *testing.T, runs *int32, want int32) {
	t.Helper()
	deadline := time.After(2 * time.Second)
	for atomic.LoadInt32(runs) < want {
		select {
		case <-deadline:
			t.Fatalf("job ran %d times, want >= %d", atomic.LoadInt32(runs), want)
		case <-time.After(time.Millisecond):
		}
	}
}

func TestRunsJobRepeatedlyDespiteErrors(t *testing.T) {
	var runs int32
	s := New(testLogger())
	s.Add(Job{Name: "tick", Interval: 2 * time.Millisecond, Run: func(_ context.Context) error {
		atomic.AddInt32(&runs, 1)
		return errors.New("boom") // must not stop the scheduler
	}})
	s.Start(context.Background())
	defer s.Stop()

	waitFor(t, &runs, 3)
}

func TestStopHaltsJobs(t *testing.T) {
	var runs int32
	s := New(testLogger())
	s.Add(Job{Name: "tick", Interval: 2 * time.Millisecond, Run: func(_ context.Context) error {
		atomic.AddInt32(&runs, 1)
		return nil
	}})
	s.Start(context.Background())
	waitFor(t, &runs, 1)

	s.Stop() // returns only after goroutines exit, so the count is final
	after := atomic.LoadInt32(&runs)
	time.Sleep(20 * time.Millisecond)
	if got := atomic.LoadInt32(&runs); got != after {
		t.Errorf("job ran after Stop: %d -> %d", after, got)
	}
}

func TestAddIgnoresInvalidJobs(t *testing.T) {
	s := New(testLogger())
	s.Add(Job{Name: "no-interval", Interval: 0, Run: func(_ context.Context) error { return nil }})
	s.Add(Job{Name: "no-run", Interval: time.Second})
	if s.Len() != 0 {
		t.Errorf("Len = %d, want 0 (invalid jobs ignored)", s.Len())
	}
}
