package ics

import (
	"strings"
	"testing"
	"time"

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

// Real-world feeds often leave commas unescaped in single-value text fields
// (RFC-invalid but common, e.g. "Braune Tonne, Bioabfall"). Text must keep the
// whole value instead of truncating at the comma, while still resolving proper
// escape sequences.
func TestTextKeepsUnescapedCommas(t *testing.T) {
	raw := strings.Join([]string{
		"BEGIN:VCALENDAR",
		"VERSION:2.0",
		"BEGIN:VEVENT",
		"UID:c@test",
		"DTSTAMP:20260101T000000Z",
		"SUMMARY:Braune Tonne, Bioabfall",
		"DESCRIPTION:escaped\\, comma\\nand newline",
		"END:VEVENT",
		"END:VCALENDAR",
		"",
	}, "\r\n")
	cal, err := Parse(strings.NewReader(raw))
	if err != nil {
		t.Fatalf("Parse: %v", err)
	}
	e := cal.Events()[0]
	if got := Text(e, FieldSummary); got != "Braune Tonne, Bioabfall" {
		t.Errorf("SUMMARY = %q, want full value with comma", got)
	}
	if got := Text(e, FieldDescription); got != "escaped, comma\nand newline" {
		t.Errorf("DESCRIPTION = %q, want unescaped comma and newline", got)
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

// go-ical refuses to encode a component-less VCALENDAR, so Serialize writes it
// by hand. That path must still produce a parseable document whose already
// escaped values are passed through untouched — and fold long lines.
func TestSerializeEventlessCalendar(t *testing.T) {
	const name = `a,b;c\d`
	longName := strings.Repeat("Very long calendar name ", 6)

	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//TidyDAV//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Props.Set(TextProp("X-WR-CALNAME", name))
	cal.Props.Set(TextProp("X-WR-CALDESC", longName))
	cal.Props.Set(DurationProp(ical.PropRefreshInterval, 90*time.Minute, true))
	cal.Props.Set(DurationProp("X-PUBLISHED-TTL", 90*time.Minute, false))

	var sb strings.Builder
	if err := Serialize(&sb, cal); err != nil {
		t.Fatalf("Serialize: %v", err)
	}
	out := sb.String()

	for _, want := range []string{
		"BEGIN:VCALENDAR\r\n", "END:VCALENDAR\r\n",
		"PRODID:-//TidyDAV//EN\r\n", "VERSION:2.0\r\n",
		`X-WR-CALNAME:a\,b\;c\\d` + "\r\n",
		"REFRESH-INTERVAL;VALUE=DURATION:PT1H30M\r\n",
		"X-PUBLISHED-TTL:PT1H30M\r\n",
	} {
		if !strings.Contains(out, want) {
			t.Errorf("output missing %q:\n%s", want, out)
		}
	}
	for _, line := range strings.Split(strings.TrimSuffix(out, "\r\n"), "\r\n") {
		if len(line) > 75 {
			t.Errorf("line not folded at 75 octets (%d): %q", len(line), line)
		}
	}

	again, err := Parse(strings.NewReader(out))
	if err != nil {
		t.Fatalf("re-parse: %v", err)
	}
	got, err := again.Props.Text("X-WR-CALNAME")
	if err != nil {
		t.Fatalf("read X-WR-CALNAME: %v", err)
	}
	if got != name {
		t.Errorf("X-WR-CALNAME round-trip = %q, want %q (double escaping?)", got, name)
	}
	if got, err := again.Props.Text("X-WR-CALDESC"); err != nil || got != longName {
		t.Errorf("folded value round-trip = %q (err %v), want %q", got, err, longName)
	}
}
