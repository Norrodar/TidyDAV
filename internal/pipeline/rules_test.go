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

func TestRenameEmptyPattern(t *testing.T) {
	if _, err := NewRenameRule(ics.FieldSummary, MatchSubstring, "  ", "x"); err == nil {
		t.Fatal("expected error for empty rename pattern")
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

// Recurring events must survive expiry when they still have occurrences ahead:
// a weekly meeting or yearly birthday started years ago, but recurs forever.
func TestExpireKeepsRecurringEvents(t *testing.T) {
	cal := mustCal(t,
		event("weekly", "SUMMARY:Weekly", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z", "RRULE:FREQ=WEEKLY"),
		event("birthday", "SUMMARY:Birthday", "DTSTART;VALUE=DATE:20100315", "RRULE:FREQ=YEARLY"),
		event("until-future", "SUMMARY:UntilFuture", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z",
			"RRULE:FREQ=WEEKLY;UNTIL=20301231T000000Z"),
		event("counted", "SUMMARY:Counted", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z",
			"RRULE:FREQ=WEEKLY;COUNT=10"),
		event("rdate", "SUMMARY:RDate", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z",
			"RDATE:20301106T090000Z"),
		event("ended", "SUMMARY:Ended", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z",
			"RRULE:FREQ=WEEKLY;UNTIL=20200331T000000Z"),
		event("plain-old", "SUMMARY:PlainOld", "DTSTART:20200106T090000Z", "DTEND:20200106T100000Z"),
	)
	r, err := NewExpireRule(30)
	if err != nil {
		t.Fatalf("NewExpireRule: %v", err)
	}
	r.now = func() time.Time { return time.Date(2026, 6, 18, 0, 0, 0, 0, time.UTC) }
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	// Series that ended in 2020 and the plain old event go; everything with
	// occurrences still ahead (or an unknowable end) stays.
	assertSummaries(t, cal, "Weekly", "Birthday", "UntilFuture", "Counted", "RDate")
}

// An empty pattern matches everything: as a blacklist it would silently blank
// the whole calendar, so building the rule must fail instead.
func TestFilterRejectsEmptyPattern(t *testing.T) {
	for _, mode := range []FilterMode{FilterBlacklist, FilterWhitelist} {
		if _, err := NewFilterRule(mode, MatchSubstring, "", nil); err == nil {
			t.Errorf("%s filter with empty pattern was accepted", mode)
		}
		if _, err := NewFilterRule(mode, MatchSubstring, "   ", nil); err == nil {
			t.Errorf("%s filter with blank pattern was accepted", mode)
		}
	}
}

// A single event in a timezone Go does not know (e.g. Outlook's Windows zone
// names) must not take the whole feed down.
func TestTimezoneSkipsUnknownZoneInsteadOfFailing(t *testing.T) {
	cal := mustCal(t,
		event("win", "SUMMARY:Outlook", "DTSTART;TZID=W. Europe Standard Time:20260702T090000"),
		event("ok", "SUMMARY:Convertible", "DTSTART:20260702T090000Z"),
	)
	r, err := NewTimezoneRule("Europe/Berlin", "")
	if err != nil {
		t.Fatalf("NewTimezoneRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply must tolerate unknown zones, got: %v", err)
	}
	if got := len(cal.Events()); got != 2 {
		t.Fatalf("event count = %d, want 2 (nothing dropped)", got)
	}
	// The convertible event was moved, the unknown one kept verbatim.
	if got := ics.Raw(cal.Events()[1], ics.FieldDTStart); !strings.Contains(got, "20260702T110000") {
		t.Errorf("convertible DTSTART = %q, want 11:00 Berlin", got)
	}
	if got := ics.Raw(cal.Events()[0], ics.FieldDTStart); got != "20260702T090000" {
		t.Errorf("unknown-zone DTSTART = %q, want the original value", got)
	}
}

func TestAlarmAddsReminder(t *testing.T) {
	cal := mustCal(t,
		event("a", "SUMMARY:Braune Tonne", "DTSTART;VALUE=DATE:20260702"),
		// An upstream alarm must be replaced, not doubled.
		event("b", "SUMMARY:Meeting", "DTSTART:20260702T090000Z",
			"BEGIN:VALARM", "ACTION:DISPLAY", "DESCRIPTION:old", "TRIGGER:-PT5M", "END:VALARM"),
	)
	r, err := NewAlarmRule(360, "") // 6h before midnight = 18:00 the evening before
	if err != nil {
		t.Fatalf("NewAlarmRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}

	var buf strings.Builder
	if err := ics.Serialize(&buf, cal); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	out := buf.String()
	if n := strings.Count(out, "BEGIN:VALARM"); n != 2 {
		t.Fatalf("VALARM count = %d, want 2 (one per event)", n)
	}
	if n := strings.Count(out, "TRIGGER:-PT6H"); n != 2 {
		t.Errorf("expected both alarms at -PT6H:\n%s", out)
	}
	if strings.Contains(out, "TRIGGER:-PT5M") {
		t.Errorf("upstream alarm was not replaced:\n%s", out)
	}
	// The description defaults to the event's own summary.
	if !strings.Contains(out, "DESCRIPTION:Braune Tonne") {
		t.Errorf("alarm description should default to the summary:\n%s", out)
	}
}

func TestAlarmCustomTextAndDurations(t *testing.T) {
	cal := mustCal(t, event("a", "SUMMARY:X", "DTSTART:20260702T090000Z"))
	r, err := NewAlarmRule(1560, "Tonne rausstellen") // 26h -> -P1DT2H
	if err != nil {
		t.Fatalf("NewAlarmRule: %v", err)
	}
	if err := r.Apply(cal); err != nil {
		t.Fatalf("Apply: %v", err)
	}
	var buf strings.Builder
	if err := ics.Serialize(&buf, cal); err != nil {
		t.Fatalf("serialize: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, "TRIGGER:-P1DT2H") {
		t.Errorf("26h lead should render as -P1DT2H:\n%s", out)
	}
	if !strings.Contains(out, "DESCRIPTION:Tonne rausstellen") {
		t.Errorf("custom alarm text missing:\n%s", out)
	}
}

func TestAlarmDurationFormats(t *testing.T) {
	tests := []struct {
		minutes int
		want    string
	}{
		{0, "PT0S"},
		{15, "-PT15M"},
		{60, "-PT1H"},
		{90, "-PT1H30M"},
		{360, "-PT6H"},
		{1440, "-P1D"},
		{1560, "-P1DT2H"},
	}
	for _, tt := range tests {
		if got := negativeDuration(time.Duration(tt.minutes) * time.Minute); got != tt.want {
			t.Errorf("negativeDuration(%dm) = %q, want %q", tt.minutes, got, tt.want)
		}
	}
}

func TestAlarmRejectsNegativeLead(t *testing.T) {
	if _, err := NewAlarmRule(-5, ""); err == nil {
		t.Error("expected error for negative lead time")
	}
}

func TestExpireInvalidDays(t *testing.T) {
	if _, err := NewExpireRule(0); err == nil {
		t.Fatal("expected error for non-positive days")
	}
}
