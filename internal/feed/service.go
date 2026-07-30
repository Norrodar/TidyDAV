// Package feed renders an output feed: fetch its sources, merge them, run the
// rule pipeline and serialize the result to ICS.
package feed

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"log/slog"
	"sort"
	"strconv"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/Norrodar/TidyDAV/internal/pipeline"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
	"github.com/emersion/go-ical"
)

// Service renders feeds.
type Service struct {
	fetcher *proxy.Fetcher
	log     *slog.Logger
}

// NewService creates a feed render service.
func NewService(fetcher *proxy.Fetcher, log *slog.Logger) *Service {
	return &Service{fetcher: fetcher, log: log}
}

// EventSummary is a compact view of an event for previews/diffs.
type EventSummary struct {
	UID         string `json:"uid"`
	Summary     string `json:"summary"`
	Start       string `json:"start"`
	Location    string `json:"location"`
	Description string `json:"description"`
}

// Render fetches and merges the feed's sources, applies the rule pipeline and
// returns the serialized ICS.
func (s *Service) Render(ctx context.Context, f *store.Feed) ([]byte, error) {
	merged, tzdefs, err := s.merge(ctx, f)
	if err != nil {
		return nil, err
	}
	p, err := buildPipeline(f.Rules)
	if err != nil {
		return nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	if err := p.Apply(merged); err != nil {
		return nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	s.attachTimezones(merged, tzdefs)
	var buf bytes.Buffer
	if err := ics.Serialize(&buf, merged); err != nil {
		return nil, fmt.Errorf("feed %s: serialize: %w", f.ID, err)
	}
	return buf.Bytes(), nil
}

// Preview returns the merged events before and after the rule pipeline, for a
// diff view. It does not require the feed to be persisted.
func (s *Service) Preview(ctx context.Context, f *store.Feed) (original, transformed []EventSummary, err error) {
	merged, _, err := s.merge(ctx, f)
	if err != nil {
		return nil, nil, err
	}
	original = summarize(merged) // snapshot before mutation

	p, err := buildPipeline(f.Rules)
	if err != nil {
		return nil, nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	if err := p.Apply(merged); err != nil {
		return nil, nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	transformed = summarize(merged)
	return original, transformed, nil
}

// Matches renders the feed (merge + pipeline) and returns the rule matches that
// drive notifications. It does not serialize the result.
func (s *Service) Matches(ctx context.Context, f *store.Feed) ([]pipeline.RuleMatch, error) {
	merged, _, err := s.merge(ctx, f)
	if err != nil {
		return nil, err
	}
	p, err := buildPipeline(f.Rules)
	if err != nil {
		return nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	if err := p.Apply(merged); err != nil {
		return nil, fmt.Errorf("feed %s: %w", f.ID, err)
	}
	return p.Matches(), nil
}

// CheckSource fetches a single source (with optional credentials) and reports
// how many events it yields, or an error describing why the tool cannot process
// it. Used by the editor's per-source validation indicator.
func (s *Service) CheckSource(ctx context.Context, url, username, password string) (int, error) {
	body, _, err := s.fetcher.FetchAuth(ctx, url, 0, username, password)
	if err != nil {
		return 0, err
	}
	cal, err := ics.Parse(bytes.NewReader(body))
	if err != nil {
		return 0, fmt.Errorf("response is not a valid iCalendar document")
	}
	return len(cal.Events()), nil
}

// newCalendar builds the empty output calendar carrying the feed's identity.
//
// Besides the mandatory PRODID/VERSION it publishes the feed name (NAME per
// RFC 7986 plus the widely implemented X-WR-CALNAME) so subscribers see a
// named calendar instead of a URL, and — when the feed caches — the refresh
// interval, so clients poll at the rate the cache actually refreshes instead
// of guessing. A blank name sets neither property: no name beats an empty one,
// which some clients render as a nameless calendar.
func newCalendar(f *store.Feed) *ical.Calendar {
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//TidyDAV//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")

	if name := strings.TrimSpace(f.Name); name != "" {
		cal.Props.Set(ics.TextProp(ical.PropName, name))
		cal.Props.Set(ics.TextProp("X-WR-CALNAME", name))
	}
	if f.TTLSeconds > 0 {
		d := time.Duration(f.TTLSeconds) * time.Second
		cal.Props.Set(ics.DurationProp(ical.PropRefreshInterval, d, true))
		// X-PUBLISHED-TTL is the pre-RFC-7986 spelling Outlook and Google read;
		// they expect it without a VALUE parameter.
		cal.Props.Set(ics.DurationProp("X-PUBLISHED-TTL", d, false))
	}
	return cal
}

// merge fetches every source (tolerating individual failures via the proxy's
// stale-on-error cache) and returns one calendar with their events, de-duplicated
// by UID, plus the upstream VTIMEZONE definitions keyed by TZID (reattached at
// render time for the TZIDs the final events actually reference).
func (s *Service) merge(ctx context.Context, f *store.Feed) (*ical.Calendar, map[string]*ical.Component, error) {
	ttl := time.Duration(f.TTLSeconds) * time.Second
	merged := newCalendar(f)

	seenUID := make(map[string]struct{})
	uidSeq := make(map[string]int)
	tzdefs := make(map[string]*ical.Component)
	var fetched int
	for _, src := range f.Sources {
		body, _, err := s.fetcher.FetchAuth(ctx, src.URL, ttl, src.Username, src.Password)
		if err != nil {
			s.log.Warn("feed source unavailable", "feed", f.ID, "url", src.URL, "error", err)
			continue
		}
		cal, err := ics.Parse(bytes.NewReader(body))
		if err != nil {
			s.log.Warn("feed source parse failed", "feed", f.ID, "url", src.URL, "error", err)
			continue
		}
		fetched++
		for _, child := range cal.Children {
			if child.Name != ical.CompTimezone {
				continue
			}
			if tzid, err := child.Props.Text(ical.PropTimezoneID); err == nil && tzid != "" {
				if _, ok := tzdefs[tzid]; !ok {
					tzdefs[tzid] = child
				}
			}
		}
		for _, e := range cal.Events() {
			uid := ics.Text(e, "UID")
			// A recurrence override (a moved or cancelled instance) shares its
			// UID with the series master and is told apart by RECURRENCE-ID.
			// Both must survive the merge, so the dedup identity spans the two.
			dedupID := uid + "\x00" + ics.Raw(e, ics.FieldRecurrenceID)
			if uid == "" {
				// Some real-world feeds omit UID, which go-ical requires on
				// encode. Synthesize a deterministic one so serialization works
				// and clients see stable identities across fetches. Exact
				// duplicates get a sequence suffix and are kept (the dedup rule
				// decides whether to drop them).
				uid = syntheticUID(src.URL, e, uidSeq)
				ics.SetText(e, "UID", uid)
			} else {
				if _, dup := seenUID[dedupID]; dup {
					continue
				}
				seenUID[dedupID] = struct{}{}
			}
			ensureDTStamp(e)
			merged.Children = append(merged.Children, e.Component)
		}
	}
	if fetched == 0 && len(f.Sources) > 0 {
		return nil, nil, fmt.Errorf("feed %s: no source could be fetched", f.ID)
	}
	return merged, tzdefs, nil
}

// attachTimezones prepends a VTIMEZONE component for every TZID the calendar's
// events reference (RFC 5545 requires it; strict clients misread local times
// otherwise). Upstream definitions are reused; missing IANA zones — e.g. one
// introduced by the timezone rule — are generated from the Go tzdata.
func (s *Service) attachTimezones(cal *ical.Calendar, upstream map[string]*ical.Component) {
	tzids := referencedTZIDs(cal)
	if len(tzids) == 0 {
		return
	}
	from, to := eventWindow(cal)
	defs := make([]*ical.Component, 0, len(tzids))
	for _, tzid := range tzids {
		if def, ok := upstream[tzid]; ok {
			defs = append(defs, def)
			continue
		}
		def, err := ics.VTimezone(tzid, from, to)
		if err != nil {
			s.log.Warn("cannot build VTIMEZONE for referenced TZID", "tzid", tzid, "error", err)
			continue
		}
		defs = append(defs, def)
	}
	cal.Children = append(defs, cal.Children...)
}

// referencedTZIDs returns the sorted set of TZID parameters used by the
// calendar's events.
func referencedTZIDs(cal *ical.Calendar) []string {
	set := make(map[string]struct{})
	for _, e := range cal.Events() {
		for _, props := range e.Props {
			for _, p := range props {
				if tzid := p.Params.Get(ical.PropTimezoneID); tzid != "" {
					set[tzid] = struct{}{}
				}
			}
		}
	}
	out := make([]string, 0, len(set))
	for tzid := range set {
		out = append(out, tzid)
	}
	sort.Strings(out)
	return out
}

// eventWindow returns the [min, max] DTSTART across events padded by a year on
// each side, so generated VTIMEZONEs cover every referenced occurrence.
//
// The span is clamped: VTIMEZONE generation scans it hour by hour, and a single
// event dated far in the future (year 2099 shows up in birthday feeds and in
// broken exports) would otherwise cost hundreds of thousands of iterations on
// every request. Observances outside the clamp are of no practical use anyway.
func eventWindow(cal *ical.Calendar) (time.Time, time.Time) {
	now := time.Now().UTC()
	minLo, maxHi := now.AddDate(-tzWindowPastYears, 0, 0), now.AddDate(tzWindowFutureYears, 0, 0)

	var lo, hi time.Time
	for _, e := range cal.Events() {
		t, err := e.DateTimeStart(time.UTC)
		if err != nil || t.IsZero() {
			continue
		}
		if lo.IsZero() || t.Before(lo) {
			lo = t
		}
		if hi.IsZero() || t.After(hi) {
			hi = t
		}
	}
	if lo.IsZero() {
		return now.AddDate(-1, 0, 0), now.AddDate(1, 0, 0)
	}

	lo, hi = lo.AddDate(-1, 0, 0), hi.AddDate(1, 0, 0)
	if lo.Before(minLo) {
		lo = minLo
	}
	if hi.After(maxHi) {
		hi = maxHi
	}
	if hi.Before(lo) {
		hi = lo.AddDate(1, 0, 0)
	}
	return lo, hi
}

// Bounds for generated VTIMEZONE observances, relative to now.
const (
	tzWindowPastYears   = 3
	tzWindowFutureYears = 6
)

// syntheticUID derives a stable UID for an event that has none: a hash of the
// source URL and the event's identifying fields, with a sequence suffix so
// exact duplicates stay distinct. Deterministic across fetches.
func syntheticUID(sourceURL string, e ical.Event, seq map[string]int) string {
	h := sha256.New()
	for _, part := range []string{
		sourceURL,
		ics.Text(e, ics.FieldSummary),
		ics.Raw(e, ics.FieldDTStart),
		ics.Text(e, ics.FieldLocation),
	} {
		h.Write([]byte(part))
		h.Write([]byte{0x1f})
	}
	key := hex.EncodeToString(h.Sum(nil))[:32]
	n := seq[key]
	seq[key]++
	uid := key + "@tidydav"
	if n > 0 {
		uid = key + "-" + strconv.Itoa(n) + "@tidydav"
	}
	return uid
}

// ensureDTStamp adds a DTSTAMP when missing (go-ical requires it on encode).
// The value is derived from DTSTART so it is deterministic across fetches.
func ensureDTStamp(e ical.Event) {
	if e.Props.Get("DTSTAMP") != nil {
		return
	}
	t, err := e.DateTimeStart(time.UTC)
	if err != nil || t.IsZero() {
		t = time.Unix(0, 0)
	}
	prop := ical.NewProp("DTSTAMP")
	prop.SetDateTime(t.UTC())
	e.Props.Set(prop)
}

func summarize(cal *ical.Calendar) []EventSummary {
	events := cal.Events()
	out := make([]EventSummary, 0, len(events))
	for _, e := range events {
		var start string
		if t, err := e.DateTimeStart(time.UTC); err == nil && !t.IsZero() {
			start = t.Format(time.RFC3339)
		}
		out = append(out, EventSummary{
			UID:         ics.Text(e, "UID"),
			Summary:     ics.Text(e, ics.FieldSummary),
			Start:       start,
			Location:    ics.Text(e, ics.FieldLocation),
			Description: ics.Text(e, ics.FieldDescription),
		})
	}
	return out
}

func buildPipeline(rules json.RawMessage) (*pipeline.Pipeline, error) {
	var configs []pipeline.RuleConfig
	if len(rules) > 0 {
		if err := json.Unmarshal(rules, &configs); err != nil {
			return nil, fmt.Errorf("decode rules: %w", err)
		}
	}
	return pipeline.BuildPipeline(configs)
}
