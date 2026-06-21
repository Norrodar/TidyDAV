package dav

import (
	"bytes"
	"fmt"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/emersion/go-vcard"
)

// CalendarUID returns the UID of the first component that has one (VEVENT/VTODO)
// in an iCalendar body, or "" if it cannot be parsed.
func CalendarUID(data []byte) string {
	cal, err := ics.Parse(bytes.NewReader(data))
	if err != nil {
		return ""
	}
	for _, child := range cal.Children {
		if uid, _ := child.Props.Text("UID"); uid != "" {
			return uid
		}
	}
	return ""
}

// CalendarModified returns the latest LAST-MODIFIED (falling back to DTSTAMP)
// across an iCalendar's components, used by the newest-wins conflict policy.
func CalendarModified(data []byte) time.Time {
	cal, err := ics.Parse(bytes.NewReader(data))
	if err != nil {
		return time.Time{}
	}
	var latest time.Time
	for _, child := range cal.Children {
		for _, field := range []string{"LAST-MODIFIED", "DTSTAMP"} {
			if t, err := child.Props.DateTime(field, time.UTC); err == nil && !t.IsZero() && t.After(latest) {
				latest = t
			}
		}
	}
	return latest
}

// CalendarStart returns the earliest DTSTART across an iCalendar's components,
// or the zero time if none can be parsed.
func CalendarStart(data []byte) time.Time {
	cal, err := ics.Parse(bytes.NewReader(data))
	if err != nil {
		return time.Time{}
	}
	var earliest time.Time
	for _, child := range cal.Children {
		if t, err := child.Props.DateTime("DTSTART", time.UTC); err == nil && !t.IsZero() {
			if earliest.IsZero() || t.Before(earliest) {
				earliest = t
			}
		}
	}
	return earliest
}

// ParseWindow parses optional window bounds. Each bound may be empty (unbounded),
// a date ("2006-01-02") or an RFC3339 timestamp. A date-only end bound is made
// inclusive (extended to the end of that day).
func ParseWindow(start, end string) (time.Time, time.Time, error) {
	s, err := parseBound(start, false)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("window start: %w", err)
	}
	e, err := parseBound(end, true)
	if err != nil {
		return time.Time{}, time.Time{}, fmt.Errorf("window end: %w", err)
	}
	return s, e, nil
}

func parseBound(s string, endInclusive bool) (time.Time, error) {
	if s == "" {
		return time.Time{}, nil
	}
	if t, err := time.Parse("2006-01-02", s); err == nil {
		if endInclusive {
			return t.Add(24*time.Hour - time.Second), nil
		}
		return t, nil
	}
	return time.Parse(time.RFC3339, s)
}

// EventInWindow reports whether an iCalendar body's start lies within
// [start, end]. A zero start/end bound is treated as unbounded. An event whose
// start cannot be parsed is considered in-window (never filtered out by a parse
// failure).
func EventInWindow(data []byte, start, end time.Time) bool {
	if start.IsZero() && end.IsZero() {
		return true
	}
	t := CalendarStart(data)
	if t.IsZero() {
		return true
	}
	if !start.IsZero() && t.Before(start) {
		return false
	}
	if !end.IsZero() && t.After(end) {
		return false
	}
	return true
}

// ContactUID returns the UID of a vCard body, or "" if it cannot be parsed.
func ContactUID(data []byte) string {
	card, err := vcard.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return ""
	}
	return card.Value(vcard.FieldUID)
}

// ContactModified returns the REV (revision) timestamp of a vCard, used by the
// newest-wins conflict policy.
func ContactModified(data []byte) time.Time {
	card, err := vcard.NewDecoder(bytes.NewReader(data)).Decode()
	if err != nil {
		return time.Time{}
	}
	rev := card.Value(vcard.FieldRevision)
	for _, layout := range []string{"20060102T150405Z", time.RFC3339, "2006-01-02T15:04:05Z07:00"} {
		if t, err := time.Parse(layout, rev); err == nil {
			return t.UTC()
		}
	}
	return time.Time{}
}
