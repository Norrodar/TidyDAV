package ics

import (
	"bytes"
	"strings"
	"testing"
	"time"

	"github.com/emersion/go-ical"
)

func encodeComponent(t *testing.T, comp *ical.Component) string {
	t.Helper()
	cal := ical.NewCalendar()
	cal.Props.SetText(ical.PropProductID, "-//t//EN")
	cal.Props.SetText(ical.PropVersion, "2.0")
	cal.Children = append(cal.Children, comp)
	var buf bytes.Buffer
	if err := Serialize(&buf, cal); err != nil {
		t.Fatalf("serialize VTIMEZONE: %v", err)
	}
	return buf.String()
}

func TestVTimezoneBerlinHasDSTTransitions(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	to := time.Date(2026, 12, 31, 0, 0, 0, 0, time.UTC)
	comp, err := VTimezone("Europe/Berlin", from, to)
	if err != nil {
		t.Fatalf("VTimezone: %v", err)
	}
	s := encodeComponent(t, comp)

	for _, want := range []string{
		"TZID:Europe/Berlin",
		"BEGIN:DAYLIGHT",
		"BEGIN:STANDARD",
		"TZOFFSETTO:+0200", // CEST
		"TZOFFSETTO:+0100", // CET
	} {
		if !strings.Contains(s, want) {
			t.Errorf("VTIMEZONE missing %q:\n%s", want, s)
		}
	}
}

func TestVTimezoneFixedOffsetZone(t *testing.T) {
	from := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	comp, err := VTimezone("UTC", from, from.AddDate(1, 0, 0))
	if err != nil {
		t.Fatalf("VTimezone: %v", err)
	}
	s := encodeComponent(t, comp)
	if !strings.Contains(s, "TZOFFSETTO:+0000") {
		t.Errorf("UTC VTIMEZONE missing zero offset:\n%s", s)
	}
	if strings.Contains(s, "BEGIN:DAYLIGHT") {
		t.Errorf("UTC must not contain DAYLIGHT observances:\n%s", s)
	}
}

func TestVTimezoneUnknownZone(t *testing.T) {
	if _, err := VTimezone("Mars/Phobos", time.Now(), time.Now().AddDate(1, 0, 0)); err == nil {
		t.Error("expected error for unknown timezone")
	}
}
