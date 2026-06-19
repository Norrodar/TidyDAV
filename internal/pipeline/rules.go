package pipeline

import (
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-ical"
)

// ── Filter ───────────────────────────────────────────────────────────────────

// FilterMode selects whether matches are removed or are the only ones kept.
type FilterMode string

const (
	// FilterBlacklist removes events that match.
	FilterBlacklist FilterMode = "blacklist"
	// FilterWhitelist keeps only events that match.
	FilterWhitelist FilterMode = "whitelist"
)

// FilterRule includes or excludes events based on a pattern match.
type FilterRule struct {
	matcher *Matcher
	fields  []string
	mode    FilterMode
	matched []string
}

// NewFilterRule builds a filter. Empty fields default to summary/description/
// location/categories.
func NewFilterRule(mode FilterMode, matchMode MatchMode, pattern string, fields []string) (*FilterRule, error) {
	switch mode {
	case FilterBlacklist, FilterWhitelist:
	default:
		return nil, fmt.Errorf("pipeline: unknown filter mode %q", mode)
	}
	m, err := NewMatcher(matchMode, pattern)
	if err != nil {
		return nil, err
	}
	if len(fields) == 0 {
		fields = defaultMatchFields()
	}
	return &FilterRule{matcher: m, fields: normalizeFields(fields), mode: mode}, nil
}

// Name implements Rule.
func (r *FilterRule) Name() string { return "filter" }

// Apply implements Rule.
func (r *FilterRule) Apply(cal *ical.Calendar) error {
	r.matched = nil
	ics.FilterEvents(cal, func(e ical.Event) bool {
		matched := r.matches(e)
		if matched {
			r.matched = append(r.matched, ics.Text(e, ics.FieldSummary))
		}
		if r.mode == FilterWhitelist {
			return matched
		}
		return !matched
	})
	return nil
}

// Matched implements Reporter.
func (r *FilterRule) Matched() []string { return r.matched }

func (r *FilterRule) matches(e ical.Event) bool {
	for _, f := range r.fields {
		if r.matcher.MatchString(fieldValue(e, f)) {
			return true
		}
	}
	return false
}

// ── Dedup ────────────────────────────────────────────────────────────────────

// PseudoFieldDate is a dedup key field referring to the start date (no time).
const PseudoFieldDate = "DATE"

// DedupRule removes duplicate events, keeping the first occurrence.
type DedupRule struct {
	keyFields []string
}

// NewDedupRule builds a dedup rule. Empty keyFields default to SUMMARY + DATE.
func NewDedupRule(keyFields []string) *DedupRule {
	if len(keyFields) == 0 {
		keyFields = []string{ics.FieldSummary, PseudoFieldDate}
	}
	return &DedupRule{keyFields: normalizeFields(keyFields)}
}

// Name implements Rule.
func (r *DedupRule) Name() string { return "dedup" }

// Apply implements Rule.
func (r *DedupRule) Apply(cal *ical.Calendar) error {
	seen := make(map[string]struct{})
	ics.FilterEvents(cal, func(e ical.Event) bool {
		key := r.key(e)
		if _, ok := seen[key]; ok {
			return false
		}
		seen[key] = struct{}{}
		return true
	})
	return nil
}

func (r *DedupRule) key(e ical.Event) string {
	parts := make([]string, 0, len(r.keyFields))
	for _, f := range r.keyFields {
		if f == PseudoFieldDate {
			parts = append(parts, startDate(e))
			continue
		}
		parts = append(parts, fieldValue(e, f))
	}
	return strings.Join(parts, "\x1f")
}

func startDate(e ical.Event) string {
	t, err := e.DateTimeStart(time.UTC)
	if err != nil {
		return ""
	}
	return t.Format("2006-01-02")
}

// ── Rename / field edit ──────────────────────────────────────────────────────

// RenameRule rewrites a text field via pattern replacement. In regex mode the
// replacement may reference capture groups ($1, $2). In substring mode matches
// are replaced literally, case-insensitively.
type RenameRule struct {
	field       string
	re          *regexp.Regexp
	replacement string
	literal     bool
	matched     []string
}

// NewRenameRule builds a rename rule. Target must be SUMMARY, DESCRIPTION or
// LOCATION.
func NewRenameRule(field string, matchMode MatchMode, pattern, replacement string) (*RenameRule, error) {
	field = strings.ToUpper(strings.TrimSpace(field))
	switch field {
	case ics.FieldSummary, ics.FieldDescription, ics.FieldLocation:
	default:
		return nil, fmt.Errorf("pipeline: rename target %q is not an editable text field", field)
	}

	var (
		re      *regexp.Regexp
		err     error
		literal bool
	)
	switch matchMode {
	case MatchRegex:
		re, err = regexp.Compile(pattern)
	case MatchSubstring:
		re, err = regexp.Compile("(?i)" + regexp.QuoteMeta(pattern))
		literal = true
	default:
		return nil, fmt.Errorf("pipeline: unknown match mode %q", matchMode)
	}
	if err != nil {
		return nil, fmt.Errorf("pipeline: rename pattern: %w", err)
	}
	return &RenameRule{field: field, re: re, replacement: replacement, literal: literal}, nil
}

// Name implements Rule.
func (r *RenameRule) Name() string { return "rename" }

// Apply implements Rule.
func (r *RenameRule) Apply(cal *ical.Calendar) error {
	r.matched = nil
	for _, e := range cal.Events() {
		val := ics.Text(e, r.field)
		if val == "" || !r.re.MatchString(val) {
			continue
		}
		var out string
		if r.literal {
			out = r.re.ReplaceAllLiteralString(val, r.replacement)
		} else {
			out = r.re.ReplaceAllString(val, r.replacement)
		}
		if out != val {
			ics.SetText(e, r.field, out)
			r.matched = append(r.matched, out)
		}
	}
	return nil
}

// Matched implements Reporter.
func (r *RenameRule) Matched() []string { return r.matched }

// ── Strip ────────────────────────────────────────────────────────────────────

// StripRule removes fields from every event (e.g. for privacy).
type StripRule struct {
	fields []string
}

// NewStripRule builds a strip rule; at least one field is required.
func NewStripRule(fields []string) (*StripRule, error) {
	if len(fields) == 0 {
		return nil, fmt.Errorf("pipeline: strip rule needs at least one field")
	}
	return &StripRule{fields: normalizeFields(fields)}, nil
}

// Name implements Rule.
func (r *StripRule) Name() string { return "strip" }

// Apply implements Rule.
func (r *StripRule) Apply(cal *ical.Calendar) error {
	for _, e := range cal.Events() {
		for _, f := range r.fields {
			ics.Remove(e, f)
		}
	}
	return nil
}

// ── Timezone ─────────────────────────────────────────────────────────────────

// TimezoneRule converts DTSTART/DTEND of timed events into a target timezone.
// Floating (zoneless) values are first interpreted in the configured default.
type TimezoneRule struct {
	target   *time.Location
	floating *time.Location
}

// NewTimezoneRule builds a timezone rule. target and floatingDefault are IANA
// names (e.g. "Europe/Berlin"); an empty floatingDefault reuses target.
func NewTimezoneRule(target, floatingDefault string) (*TimezoneRule, error) {
	loc, err := time.LoadLocation(target)
	if err != nil {
		return nil, fmt.Errorf("pipeline: timezone %q: %w", target, err)
	}
	floating := loc
	if floatingDefault != "" {
		floating, err = time.LoadLocation(floatingDefault)
		if err != nil {
			return nil, fmt.Errorf("pipeline: floating timezone %q: %w", floatingDefault, err)
		}
	}
	return &TimezoneRule{target: loc, floating: floating}, nil
}

// Name implements Rule.
func (r *TimezoneRule) Name() string { return "timezone" }

// Apply implements Rule.
func (r *TimezoneRule) Apply(cal *ical.Calendar) error {
	for _, e := range cal.Events() {
		for _, field := range []string{ics.FieldDTStart, ics.FieldDTEnd} {
			if err := r.convert(e, field); err != nil {
				return err
			}
		}
	}
	return nil
}

func (r *TimezoneRule) convert(e ical.Event, field string) error {
	prop := e.Props.Get(field)
	if prop == nil || ics.IsDateOnly(prop) {
		return nil // absent or all-day value: leave untouched
	}
	t, err := prop.DateTime(r.floating)
	if err != nil {
		return fmt.Errorf("parse %s: %w", field, err)
	}
	np := ical.NewProp(field)
	np.SetDateTime(t.In(r.target))
	e.Props.Set(np)
	return nil
}

// ── Expire ───────────────────────────────────────────────────────────────────

// ExpireRule drops events that ended more than maxAge ago.
type ExpireRule struct {
	maxAge time.Duration
	now    func() time.Time
}

// NewExpireRule builds an expire rule keeping events newer than `days` days.
func NewExpireRule(days int) (*ExpireRule, error) {
	if days <= 0 {
		return nil, fmt.Errorf("pipeline: expire days must be > 0, got %d", days)
	}
	return &ExpireRule{maxAge: time.Duration(days) * 24 * time.Hour, now: time.Now}, nil
}

// Name implements Rule.
func (r *ExpireRule) Name() string { return "expire" }

// Apply implements Rule.
func (r *ExpireRule) Apply(cal *ical.Calendar) error {
	cutoff := r.now().Add(-r.maxAge)
	ics.FilterEvents(cal, func(e ical.Event) bool {
		end, err := e.DateTimeEnd(time.UTC)
		if err != nil || end.IsZero() {
			return true // keep events we cannot date
		}
		return !end.Before(cutoff)
	})
	return nil
}

// normalizeFields upper-cases and trims field names.
func normalizeFields(fields []string) []string {
	out := make([]string, 0, len(fields))
	for _, f := range fields {
		out = append(out, strings.ToUpper(strings.TrimSpace(f)))
	}
	return out
}
