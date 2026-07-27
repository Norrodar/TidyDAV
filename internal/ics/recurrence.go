package ics

import (
	"strings"
	"time"

	"github.com/emersion/go-ical"
)

// Recurrence-related property names.
const (
	FieldRRule        = "RRULE"
	FieldRDate        = "RDATE"
	FieldRecurrenceID = "RECURRENCE-ID"
)

// IsRecurring reports whether an event repeats (has an RRULE or extra RDATEs).
func IsRecurring(e ical.Event) bool {
	return Raw(e, FieldRRule) != "" || Raw(e, FieldRDate) != ""
}

// LastOccurrence returns the end of an event's final occurrence, and whether
// that end is bounded at all.
//
// A non-recurring event ends at DTEND (falling back to DTSTART). A recurring
// one ends at its RRULE's UNTIL, extended by the event's duration, or at its
// latest RDATE — whichever is later. Rules without UNTIL (including COUNT-
// limited ones, whose end would require expanding the series) are reported as
// unbounded: callers must then keep the event, since dropping it would delete
// occurrences that are still to come.
func LastOccurrence(e ical.Event) (time.Time, bool) {
	start, errStart := e.DateTimeStart(time.UTC)
	end, err := e.DateTimeEnd(time.UTC)
	if err != nil || end.IsZero() {
		end = start
	}

	rrule := Raw(e, FieldRRule)
	rdate := Raw(e, FieldRDate)
	if rrule == "" && rdate == "" {
		if end.IsZero() {
			return time.Time{}, false
		}
		return end, true
	}

	var last time.Time
	if rrule != "" {
		until, ok := ruleUntil(rrule)
		if !ok {
			return time.Time{}, false // open-ended (or COUNT-limited): never expires
		}
		// UNTIL bounds the last *start*; the occurrence still runs for the
		// event's duration.
		if errStart == nil && !start.IsZero() && end.After(start) {
			until = until.Add(end.Sub(start))
		}
		last = until
	}
	if latest, ok := latestRDate(rdate); ok && latest.After(last) {
		last = latest
	}
	if last.IsZero() {
		return time.Time{}, false
	}
	return last, true
}

// ruleUntil extracts the UNTIL bound from an RRULE value.
func ruleUntil(rrule string) (time.Time, bool) {
	for _, part := range strings.Split(rrule, ";") {
		name, value, found := strings.Cut(part, "=")
		if !found || !strings.EqualFold(strings.TrimSpace(name), "UNTIL") {
			continue
		}
		if t, ok := parseICalTime(strings.TrimSpace(value)); ok {
			return t, true
		}
		return time.Time{}, false // malformed UNTIL: treat as unbounded
	}
	return time.Time{}, false
}

// latestRDate returns the latest date in an RDATE value (a comma-separated
// list, whose entries may be periods of the form start/end).
func latestRDate(rdate string) (time.Time, bool) {
	var latest time.Time
	for _, entry := range strings.Split(rdate, ",") {
		entry = strings.TrimSpace(entry)
		if entry == "" {
			continue
		}
		// A period "start/end" (or "start/duration") is bounded by its start at
		// minimum; use the part before the slash.
		if start, _, isPeriod := strings.Cut(entry, "/"); isPeriod {
			entry = start
		}
		if t, ok := parseICalTime(entry); ok && t.After(latest) {
			latest = t
		}
	}
	return latest, !latest.IsZero()
}

// parseICalTime parses the DATE and DATE-TIME forms used by UNTIL and RDATE.
// Floating (zoneless) values are read as UTC, which is precise enough for the
// day-granularity expiry check.
func parseICalTime(v string) (time.Time, bool) {
	for _, layout := range []string{"20060102T150405Z", "20060102T150405", "20060102"} {
		if t, err := time.Parse(layout, v); err == nil {
			return t.UTC(), true
		}
	}
	return time.Time{}, false
}
