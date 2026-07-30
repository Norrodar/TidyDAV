// Package ics provides small read/transform helpers over github.com/emersion/go-ical
// so the rest of the app can work with iCalendar events without depending on
// go-ical internals.
package ics

import (
	"bytes"
	"fmt"
	"io"
	"sort"
	"strings"
	"time"
	"unicode/utf8"

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
//
// A calendar without components is written by hand: go-ical refuses to encode
// it ("calendar is empty"), but an event-less feed still has to carry its
// identity — name and refresh interval — or subscribers see a nameless
// calendar the moment every event is filtered away.
func Serialize(w io.Writer, cal *ical.Calendar) error {
	if len(cal.Children) == 0 {
		return serializeEmpty(w, cal)
	}
	if err := ical.NewEncoder(w).Encode(cal); err != nil {
		return fmt.Errorf("ics: serialize: %w", err)
	}
	return nil
}

// serializeEmpty writes BEGIN/END:VCALENDAR around the calendar's properties,
// mirroring go-ical's encoder (sorted property names, sorted parameters) and
// folding per RFC 5545. Property values are passed through verbatim: they are
// already escaped by whoever set them.
func serializeEmpty(w io.Writer, cal *ical.Calendar) error {
	var buf bytes.Buffer
	foldLine(&buf, "BEGIN:"+ical.CompCalendar)

	names := make([]string, 0, len(cal.Props))
	for name := range cal.Props {
		names = append(names, name)
	}
	sort.Strings(names)
	for _, name := range names {
		for _, prop := range cal.Props[name] {
			line, err := contentLine(&prop)
			if err != nil {
				return fmt.Errorf("ics: serialize: %w", err)
			}
			foldLine(&buf, line)
		}
	}

	foldLine(&buf, "END:"+ical.CompCalendar)
	if _, err := w.Write(buf.Bytes()); err != nil {
		return fmt.Errorf("ics: serialize: %w", err)
	}
	return nil
}

// contentLine renders one unfolded content line: NAME;PARAM=value:value.
func contentLine(prop *ical.Prop) (string, error) {
	var sb strings.Builder
	sb.WriteString(prop.Name)

	paramNames := make([]string, 0, len(prop.Params))
	for name := range prop.Params {
		paramNames = append(paramNames, name)
	}
	sort.Strings(paramNames)
	for _, name := range paramNames {
		sb.WriteString(";")
		sb.WriteString(name)
		sb.WriteString("=")
		for i, v := range prop.Params[name] {
			if i > 0 {
				sb.WriteString(",")
			}
			if strings.ContainsRune(v, '"') {
				return "", fmt.Errorf("param %q contains a double-quote", name)
			}
			if strings.ContainsAny(v, ";:,") {
				sb.WriteString(`"` + v + `"`)
			} else {
				sb.WriteString(v)
			}
		}
	}

	if strings.ContainsAny(prop.Value, "\r\n") {
		return "", fmt.Errorf("property %q value contains a CR or LF", prop.Name)
	}
	sb.WriteString(":")
	sb.WriteString(prop.Value)
	return sb.String(), nil
}

// foldLine writes a CRLF-terminated content line, folded at 75 octets as
// RFC 5545 §3.1 requires. Continuation lines start with a single space (which
// counts towards the limit) and never split a multi-octet UTF-8 sequence.
func foldLine(buf *bytes.Buffer, line string) {
	const limit = 75
	max := limit
	for len(line) > max {
		cut := max
		for cut > 1 && !utf8.RuneStart(line[cut]) {
			cut--
		}
		buf.WriteString(line[:cut])
		buf.WriteString("\r\n ")
		line = line[cut:]
		max = limit - 1 // the leading space of the continuation line
	}
	buf.WriteString(line)
	buf.WriteString("\r\n")
}

// TextProp builds a property whose value is RFC 5545 escaped TEXT (commas,
// semicolons, backslashes and newlines). go-ical adds a redundant VALUE=TEXT
// parameter for property names it does not know — every X- name — which is
// dropped here: TEXT is the default type and clients read the bare form.
func TextProp(name, value string) *ical.Prop {
	prop := ical.NewProp(name)
	prop.SetText(value)
	prop.Params.Del(ical.ParamValue)
	return prop
}

// DurationProp builds a property holding an RFC 5545 DURATION value. The
// VALUE=DURATION parameter is only written when withValueParam is set: some
// properties (REFRESH-INTERVAL) are specified with it, while the de-facto
// X-PUBLISHED-TTL is expected bare by Outlook and Google.
func DurationProp(name string, d time.Duration, withValueParam bool) *ical.Prop {
	prop := ical.NewProp(name)
	prop.Value = Duration(d)
	if withValueParam {
		prop.Params.Set(ical.ParamValue, string(ical.ValueDuration))
	}
	return prop
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
