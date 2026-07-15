// Package ics provides small read/transform helpers over github.com/emersion/go-ical
// so the rest of the app can work with iCalendar events without depending on
// go-ical internals.
package ics

import (
	"fmt"
	"io"
	"strings"

	"github.com/emersion/go-ical"
)

// Common iCalendar field names, re-exported so callers need not import go-ical.
const (
	FieldSummary     = "SUMMARY"
	FieldDescription = "DESCRIPTION"
	FieldLocation    = "LOCATION"
	FieldCategories  = "CATEGORIES"
	FieldDTStart     = "DTSTART"
	FieldDTEnd       = "DTEND"
)

// Parse decodes an iCalendar document.
func Parse(r io.Reader) (*ical.Calendar, error) {
	cal, err := ical.NewDecoder(r).Decode()
	if err != nil {
		return nil, fmt.Errorf("ics: parse: %w", err)
	}
	return cal, nil
}

// Serialize encodes a calendar. Note go-ical validates required properties
// (VCALENDAR needs PRODID+VERSION, VEVENT needs UID+DTSTAMP).
func Serialize(w io.Writer, cal *ical.Calendar) error {
	if err := ical.NewEncoder(w).Encode(cal); err != nil {
		return fmt.Errorf("ics: serialize: %w", err)
	}
	return nil
}

// Text returns the unescaped text value of a field, or "" when absent. Unlike
// go-ical's Prop.Text it treats the value as a single text (not a
// comma-separated list): real-world feeds often leave commas unescaped in
// single-value fields like SUMMARY ("Braune Tonne, Bioabfall"), which
// Prop.Text would silently truncate at the comma.
func Text(e ical.Event, field string) string {
	prop := e.Props.Get(field)
	if prop == nil {
		return ""
	}
	return unescapeText(prop.Value)
}

// unescapeText resolves RFC 5545 TEXT escapes (\\ \, \; \n) and keeps any
// unescaped separator characters verbatim.
func unescapeText(v string) string {
	if !strings.ContainsRune(v, '\\') {
		return v
	}
	var sb strings.Builder
	sb.Grow(len(v))
	for i := 0; i < len(v); i++ {
		c := v[i]
		if c != '\\' || i+1 >= len(v) {
			sb.WriteByte(c)
			continue
		}
		i++
		switch n := v[i]; n {
		case 'n', 'N':
			sb.WriteByte('\n')
		case '\\', ',', ';':
			sb.WriteByte(n)
		default:
			// Unknown escape: keep it untouched rather than failing.
			sb.WriteByte('\\')
			sb.WriteByte(n)
		}
	}
	return sb.String()
}

// Raw returns the raw (still-escaped) value of a field, or "" when absent.
// Useful for multi-value fields like CATEGORIES where Text returns only the first.
func Raw(e ical.Event, field string) string {
	if prop := e.Props.Get(field); prop != nil {
		return prop.Value
	}
	return ""
}

// SetText sets a field's text value.
func SetText(e ical.Event, field, value string) {
	e.Props.SetText(field, value)
}

// Remove deletes a field (all its values) from the event.
func Remove(e ical.Event, field string) {
	e.Props.Del(strings.ToUpper(field))
}

// FilterEvents rebuilds the calendar in place, keeping every non-event child and
// only the events for which keep returns true. Order is preserved.
func FilterEvents(cal *ical.Calendar, keep func(ical.Event) bool) {
	kept := make([]*ical.Component, 0, len(cal.Children))
	for _, child := range cal.Children {
		if child.Name != ical.CompEvent {
			kept = append(kept, child)
			continue
		}
		if keep(ical.Event{Component: child}) {
			kept = append(kept, child)
		}
	}
	cal.Children = kept
}

// IsDateOnly reports whether a property holds a date (no time component),
// i.e. an all-day value such as 20260131.
func IsDateOnly(prop *ical.Prop) bool {
	return len(prop.Value) == len("20060102") && !strings.Contains(prop.Value, "T")
}
