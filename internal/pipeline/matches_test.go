package pipeline

import (
	"testing"

	"github.com/Norrodar/TidyDAV/internal/ics"
)

func TestPipelineMatches(t *testing.T) {
	cal := mustCal(t,
		event("1", "SUMMARY:Spam offer"),
		event("2", "SUMMARY:ABK: Mathe"),
		event("3", "SUMMARY:Normal"),
	)
	filter, _ := NewFilterRule(FilterBlacklist, MatchSubstring, "spam", []string{ics.FieldSummary})
	rename, _ := NewRenameRule(ics.FieldSummary, MatchSubstring, "ABK: ", "")
	dedup := NewDedupRule(nil)

	p := New(filter, rename, dedup)
	if err := p.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	got := map[string][]MatchedEvent{}
	for _, m := range p.Matches() {
		got[m.Rule] = m.Events
	}
	if len(got["filter"]) != 1 || got["filter"][0].Summary != "Spam offer" {
		t.Errorf("filter matches = %v, want [Spam offer]", got["filter"])
	}
	if got["filter"][0].UID != "1" {
		t.Errorf("filter match UID = %q, want 1 (stable identity for dedup)", got["filter"][0].UID)
	}
	if len(got["rename"]) != 1 || got["rename"][0].Summary != "Mathe" {
		t.Errorf("rename matches = %v, want [Mathe]", got["rename"])
	}
	if _, ok := got["dedup"]; ok {
		t.Errorf("dedup should not report matches: %v", got["dedup"])
	}
}

func TestMatchesResetBetweenRuns(t *testing.T) {
	filter, _ := NewFilterRule(FilterBlacklist, MatchSubstring, "spam", []string{ics.FieldSummary})
	p := New(filter)

	_ = p.Apply(mustCal(t, event("1", "SUMMARY:Spam")))
	if len(p.Matches()) != 1 {
		t.Fatalf("first run matches = %d, want 1", len(p.Matches()))
	}
	_ = p.Apply(mustCal(t, event("2", "SUMMARY:Clean")))
	if len(p.Matches()) != 0 {
		t.Errorf("second run matches = %d, want 0 (reset)", len(p.Matches()))
	}
}
