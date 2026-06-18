package pipeline

import (
	"strings"
	"testing"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-ical"
)

// mustCal builds a calendar from VEVENT blocks (see event()).
func mustCal(t *testing.T, events ...string) *ical.Calendar {
	t.Helper()
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//TidyDAV//test//EN\r\n")
	for _, ev := range events {
		b.WriteString(ev)
	}
	b.WriteString("END:VCALENDAR\r\n")
	cal, err := ics.Parse(strings.NewReader(b.String()))
	if err != nil {
		t.Fatalf("parse fixture: %v", err)
	}
	return cal
}

// event builds a VEVENT block (CRLF) with UID and DTSTAMP added automatically.
func event(uid string, lines ...string) string {
	out := []string{"BEGIN:VEVENT", "UID:" + uid, "DTSTAMP:20260101T000000Z"}
	out = append(out, lines...)
	out = append(out, "END:VEVENT")
	return strings.Join(out, "\r\n") + "\r\n"
}

func summaries(cal *ical.Calendar) []string {
	s := make([]string, 0)
	for _, e := range cal.Events() {
		s = append(s, ics.Text(e, ics.FieldSummary))
	}
	return s
}

func assertSummaries(t *testing.T, cal *ical.Calendar, want ...string) {
	t.Helper()
	got := summaries(cal)
	if strings.Join(got, "|") != strings.Join(want, "|") {
		t.Fatalf("summaries = %v, want %v", got, want)
	}
}

func TestFilterBlacklist(t *testing.T) {
	cal := mustCal(t,
		event("1", "SUMMARY:Team Meeting"),
		event("2", "SUMMARY:Spam Offer"),
	)
	r, err := NewFilterRule(FilterBlacklist, MatchSubstring, "spam", nil)
	if err != nil {
		t.Fatalf("NewFilterRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertSummaries(t, cal, "Team Meeting")
}

func TestFilterWhitelist(t *testing.T) {
	cal := mustCal(t,
		event("1", "SUMMARY:Team Meeting"),
		event("2", "SUMMARY:Lunch"),
	)
	r, _ := NewFilterRule(FilterWhitelist, MatchSubstring, "meeting", []string{ics.FieldSummary})
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertSummaries(t, cal, "Team Meeting")
}

func TestDedupDefault(t *testing.T) {
	cal := mustCal(t,
		event("1", "SUMMARY:Schwarze Tonne", "DTSTART:20260115T060000Z"),
		event("2", "SUMMARY:Schwarze Tonne", "DTSTART:20260115T070000Z"), // same summary+date
		event("3", "SUMMARY:Schwarze Tonne", "DTSTART:20260116T060000Z"), // different date
	)
	r := NewDedupRule(nil)
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if n := len(cal.Events()); n != 2 {
		t.Fatalf("after dedup got %d events, want 2", n)
	}
}

func TestRenameRegexGroups(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:Bin 42 pickup"))
	r, err := NewRenameRule(ics.FieldSummary, MatchRegex, `Bin (\d+)`, "Trash $1")
	if err != nil {
		t.Fatalf("NewRenameRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertSummaries(t, cal, "Trash 42 pickup")
}

func TestRenameSubstringCaseInsensitive(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:ABK: Mathe"))
	r, err := NewRenameRule(ics.FieldSummary, MatchSubstring, "abk: ", "")
	if err != nil {
		t.Fatalf("NewRenameRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	assertSummaries(t, cal, "Mathe")
}

func TestRenameInvalidTarget(t *testing.T) {
	if _, err := NewRenameRule("DTSTART", MatchSubstring, "x", "y"); err == nil {
		t.Fatal("expected error for non-editable rename target")
	}
}

func TestStrip(t *testing.T) {
	cal := mustCal(t, event("1",
		"SUMMARY:Private",
		"DESCRIPTION:secret notes",
		"LOCATION:Home",
	))
	r, err := NewStripRule([]string{ics.FieldDescription})
	if err != nil {
		t.Fatalf("NewStripRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	e := cal.Events()[0]
	if got := ics.Text(e, ics.FieldDescription); got != "" {
		t.Errorf("DESCRIPTION = %q, want stripped", got)
	}
	if got := ics.Text(e, ics.FieldLocation); got != "Home" {
		t.Errorf("LOCATION = %q, want Home (untouched)", got)
	}
}

func TestTimezoneConvertUTCToBerlin(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:M", "DTSTART:20260115T090000Z"))
	r, err := NewTimezoneRule("Europe/Berlin", "")
	if err != nil {
		t.Fatalf("NewTimezoneRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	e := cal.Events()[0]
	// The instant must be preserved (09:00Z) ...
	start, err := e.DateTimeStart(time.UTC)
	if err != nil {
		t.Fatalf("DateTimeStart: %v", err)
	}
	if !start.Equal(time.Date(2026, 1, 15, 9, 0, 0, 0, time.UTC)) {
		t.Errorf("start instant = %v, want 2026-01-15T09:00:00Z", start)
	}
	// ... and the stored value should carry the target zone.
	if tzid := e.Props.Get(ics.FieldDTStart).Params.Get("TZID"); tzid != "Europe/Berlin" {
		t.Errorf("TZID = %q, want Europe/Berlin", tzid)
	}
}

func TestTimezoneFloatingUsesDefault(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:M", "DTSTART:20260115T090000")) // floating
	r, err := NewTimezoneRule("UTC", "Europe/Berlin")
	if err != nil {
		t.Fatalf("NewTimezoneRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// 09:00 Berlin (winter, UTC+1) == 08:00 UTC.
	start, _ := cal.Events()[0].DateTimeStart(time.UTC)
	if start.Hour() != 8 {
		t.Errorf("converted hour = %d, want 8 (08:00Z)", start.Hour())
	}
}

func TestTimezoneAllDayUnchanged(t *testing.T) {
	cal := mustCal(t, event("1", "SUMMARY:M", "DTSTART;VALUE=DATE:20260115"))
	r, _ := NewTimezoneRule("Europe/Berlin", "")
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	if got := cal.Events()[0].Props.Get(ics.FieldDTStart).Value; got != "20260115" {
		t.Errorf("all-day DTSTART = %q, want 20260115 (unchanged)", got)
	}
}

func TestExpire(t *testing.T) {
	cal := mustCal(t,
		event("old", "SUMMARY:Old", "DTSTART:20260101T090000Z", "DTEND:20260101T100000Z"),
		event("new", "SUMMARY:New", "DTSTART:20260601T090000Z", "DTEND:20260601T100000Z"),
		event("undated", "SUMMARY:Undated"),
	)
	r, err := NewExpireRule(30)
	if err != nil {
		t.Fatalf("NewExpireRule: %v", err)
	}
	r.now = func() time.Time { return time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC) }
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// "Old" ended > 30 days ago and is dropped; "New" and the undatable one stay.
	assertSummaries(t, cal, "New", "Undated")
}

func TestExpireInvalidDays(t *testing.T) {
	if _, err := NewExpireRule(0); err == nil {
		t.Fatal("expected error for non-positive days")
	}
}
