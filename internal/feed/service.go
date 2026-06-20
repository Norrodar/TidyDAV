// Package feed renders an output feed: fetch its sources, merge them, run the
// rule pipeline and serialize the result to ICS.
package feed

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/Norrodar/TidyDAV/internal/pipeline"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
	"github.com/emersion/go-ical"
)

// emptyCalendar is a valid, event-less ICS document (go-ical refuses to encode
// a calendar with no components, so the empty case is built by hand).
const emptyCalendar = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//TidyDAV//EN\r\nEND:VCALENDAR\r\n"

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
	merged, err := s.merge(ctx, f)
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
	if len(merged.Children) == 0 {
		return []byte(emptyCalendar), nil
	}
	var buf bytes.Buffer
	if err := ics.Serialize(&buf, merged); err != nil {
		return nil, fmt.Errorf("feed %s: serialize: %w", f.ID, err)
	}
	return buf.Bytes(), nil
}

// Preview returns the merged events before and after the rule pipeline, for a
// diff view. It does not require the feed to be persisted.
func (s *Service) Preview(ctx context.Context, f *store.Feed) (original, transformed []EventSummary, err error) {
	merged, err := s.merge(ctx, f)
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
	merged, err := s.merge(ctx, f)
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

// merge fetches every source (tolerating individual failures via the proxy's
// stale-on-error cache) and returns one calendar with their events, de-duplicated
// by UID.
func (s *Service) merge(ctx context.Context, f *store.Feed) (*ical.Calendar, error) {
	ttl := time.Duration(f.TTLSeconds) * time.Second
	merged := ical.NewCalendar()
	merged.Props.SetText(ical.PropProductID, "-//TidyDAV//EN")
	merged.Props.SetText(ical.PropVersion, "2.0")

	seenUID := make(map[string]struct{})
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
		for _, e := range cal.Events() {
			if uid := ics.Text(e, "UID"); uid != "" {
				if _, dup := seenUID[uid]; dup {
					continue
				}
				seenUID[uid] = struct{}{}
			}
			merged.Children = append(merged.Children, e.Component)
		}
	}
	if fetched == 0 && len(f.Sources) > 0 {
		return nil, fmt.Errorf("feed %s: no source could be fetched", f.ID)
	}
	return merged, nil
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
