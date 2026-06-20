package ics

import (
	"strings"
	"testing"

	"github.com/emersion/go-ical"
)

// sample returns a small two-event calendar with CRLF line endings (RFC 5545).
func sample() string {
	lines := []string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"PRODID:-//TidyDAV//test//EN",
		"BEGIN:VEVENT",
		"UID:1@test",
		"DTSTAMP:20260101T000000Z",
		"DTSTART:20260115T090000Z",
		"DTEND:20260115T100000Z",
		"SUMMARY:Team Meeting",
		"DESCRIPTION:Weekly sync",
		"LOCATION:Room A",
		"CATEGORIES:work,sync",
		"END:VEVENT",
		"BEGIN:VEVENT",
		"UID:2@test",
		"DTSTAMP:20260101T000000Z",
		"DTSTART:20260116T090000Z",
		"SUMMARY:Schwarze Tonne",
		"CATEGORIES:waste",
		"END:VEVENT",
		"END:VCALENDAR",
		"",
	}
	return strings.Join(lines, "\r\n")
}

func TestParseAndFields(t *testing.T) {
	cal, err := Parse(strings.NewReader(sample()))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	events := cal.Events()
	if len(events) != 2 {
		t.Fatalf("got %d events, want 2", len(events))
	}

	if got := Text(events[0], FieldSummary); got != "Team Meeting" {
		t.Errorf("SUMMARY = %q, want Team Meeting", got)
	}
	if got := Text(events[0], FieldLocation); got != "Room A" {
		t.Errorf("LOCATION = %q, want Room A", got)
	}
	if got := Raw(events[0], FieldCategories); got != "work,sync" {
		t.Errorf("CATEGORIES raw = %q, want work,sync", got)
	}
	if got := Text(events[1], FieldDescription); got != "" {
		t.Errorf("missing DESCRIPTION = %q, want empty", got)
	}
}

func TestSetAndRemove(t *testing.T) {
	cal, err := Parse(strings.NewReader(sample()))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	e := cal.Events()[0]

	SetText(e, FieldSummary, "Renamed")
	if got := Text(e, FieldSummary); got != "Renamed" {
		t.Errorf("after SetText SUMMARY = %q, want Renamed", got)
	}

	Remove(e, FieldLocation)
	if got := Text(e, FieldLocation); got != "" {
		t.Errorf("after Remove LOCATION = %q, want empty", got)
	}
}

func TestFilterEvents(t *testing.T) {
	cal, err := Parse(strings.NewReader(sample()))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	FilterEvents(cal, func(e ical.Event) bool {
		return Text(e, FieldSummary) == "Schwarze Tonne"
	})
	events := cal.Events()
	if len(events) != 1 {
		t.Fatalf("got %d events after filter, want 1", len(events))
	}
	if got := Text(events[0], FieldSummary); got != "Schwarze Tonne" {
		t.Errorf("kept SUMMARY = %q, want Schwarze Tonne", got)
	}
}

func TestSerializeRoundTrip(t *testing.T) {
	cal, err := Parse(strings.NewReader(sample()))
	if err != nil {
		t.Fatalf("Parse() error: %v", err)
	}
	var sb strings.Builder
	if err := Serialize(&sb, cal); err != nil {
		t.Fatalf("Serialize() error: %v", err)
	}
	out := sb.String()
	if !strings.Contains(out, "SUMMARY:Team Meeting") {
		t.Errorf("serialized output missing summary:\n%s", out)
	}

	again, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("re-parse error: %v", err)
	}
	if len(again.Events()) != 2 {
		t.Errorf("round-trip event count = %d, want 2", len(again.Events()))
	}
}
