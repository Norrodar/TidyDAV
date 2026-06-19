package pipeline

import (
	"fmt"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-ical"
)

// Rule transforms a calendar in place.
type Rule interface {
	Name() string
	Apply(cal *ical.Calendar) error
}

// Pipeline applies a fixed list of rules sequentially.
type Pipeline struct {
	rules []Rule
}

// New builds a pipeline; rules are applied in the given order.
func New(rules ...Rule) *Pipeline {
	return &Pipeline{rules: rules}
}

// Apply runs each rule in order, stopping at the first error. A failing rule
// returns an error rather than panicking, so the caller can serve stale data.
func (p *Pipeline) Apply(cal *ical.Calendar) error {
	for _, r := range p.rules {
		if err := r.Apply(cal); err != nil {
			return fmt.Errorf("pipeline: rule %q: %w", r.Name(), err)
		}
	}
	return nil
}

// Len returns the number of rules in the pipeline.
func (p *Pipeline) Len() int { return len(p.rules) }

// Reporter is implemented by rules that record which events they matched during
// the most recent Apply (currently filter and rename).
type Reporter interface {
	Matched() []string
}

// RuleMatch lists the event summaries a rule matched.
type RuleMatch struct {
	Rule   string
	Events []string
}

// Matches returns, for each reporting rule that matched at least one event, the
// matched event summaries. Call after Apply. This is the signal used to trigger
// notifications.
func (p *Pipeline) Matches() []RuleMatch {
	var out []RuleMatch
	for _, r := range p.rules {
		reporter, ok := r.(Reporter)
		if !ok {
			continue
		}
		if events := reporter.Matched(); len(events) > 0 {
			out = append(out, RuleMatch{Rule: r.Name(), Events: events})
		}
	}
	return out
}

// defaultMatchFields are the event fields matched when a rule specifies none.
func defaultMatchFields() []string {
	return []string{
		ics.FieldSummary,
		ics.FieldDescription,
		ics.FieldLocation,
		ics.FieldCategories,
	}
}

// fieldValue returns the matchable string for a field (raw for multi-value
// CATEGORIES, unescaped text otherwise).
func fieldValue(e ical.Event, field string) string {
	if field == ics.FieldCategories {
		return ics.Raw(e, field)
	}
	return ics.Text(e, field)
}
