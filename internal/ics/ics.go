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

// Text returns the unescaped first text value of a field, or "" when absent.
func Text(e ical.Event, field string) string {
	prop := e.Props.Get(field)
	if prop == nil {
		return ""
	}
	if v, err := prop.Text(); err == nil {
		return v
	}
	return prop.Value
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
