package pipeline

import (
	"errors"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-ical"
)

func TestPipelineSequential(t *testing.T) {
	cal := mustCal(t,
		event("1", "SUMMARY:ABK: Mathe", "DESCRIPTION:secret"),
		event("2", "SUMMARY:Cancelled trip"),
		event("3", "SUMMARY:ABK: Mathe", "DTSTART:20260115T060000Z"),
		event("4", "SUMMARY:ABK: Mathe", "DTSTART:20260115T070000Z"),
	)

	filter, _ := NewFilterRule(FilterBlacklist, MatchSubstring, "cancelled", nil)
	rename, _ := NewRenameRule(ics.FieldSummary, MatchSubstring, "ABK: ", "")
	strip, _ := NewStripRule([]string{ics.FieldDescription})
	dedup := NewDedupRule(nil)

	p := New(filter, rename, strip, dedup)
	if p.Len() != 4 {
		t.Fatalf("Len = %d, want 4", p.Len())
	}
	if err := p.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	// "Cancelled trip" filtered; "ABK: " removed; events 3 & 4 ("Mathe", same
	// date) deduped to one; event 1 ("Mathe", undated) kept → 2 events remain.
	got := summaries(cal)
	if len(got) != 2 {
		t.Fatalf("got %d events, want 2 (%v)", len(got), got)
	}
	for _, s := range got {
		if strings.Contains(s, "ABK") {
			t.Errorf("summary still contains ABK: %q", s)
		}
	}
	if d := ics.Text(cal.Events()[0], ics.FieldDescription); d != "" {
		t.Errorf("DESCRIPTION = %q, want stripped", d)
	}

	// Transformed feed must still serialize (required properties intact).
	var sb strings.Builder
	if err := ics.Serialize(&sb, cal); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
}

type failingRule struct{}

func (failingRule) Name() string               { return "boom" }
func (failingRule) Apply(*ical.Calendar) error { return errors.New("kaboom") }

func TestPipelinePropagatesError(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:x"))
	err := New(failingRule{}).Apply(cal)
	if err == nil || !strings.Contains(err.Error(), "boom") {
		t.Fatalf("expected wrapped rule error, got %v", err)
	}
}
