package ics

import (
	"fmt"
	"time"

	"github.com/emersion/go-ical"
)

// VTimezone builds a VTIMEZONE component for an IANA tzid whose observances
// cover [from, to]. RFC 5545 requires every TZID referenced by an event to have
// a matching VTIMEZONE in the calendar; strict clients (e.g. Outlook) otherwise
// misread local times. The observances are emitted as explicit transitions
// (no RRULE), which is valid and simple.
func VTimezone(tzid string, from, to time.Time) (*ical.Component, error) {
	loc, err := time.LoadLocation(tzid)
	if err != nil {
		return nil, fmt.Errorf("ics: load timezone %q: %w", tzid, err)
	}
	if to.Before(from) {
		from, to = to, from
	}

	comp := ical.NewComponent(ical.CompTimezone)
	comp.Props.SetText(ical.PropTimezoneID, tzid)

	// Initial observance anchored at the window start, so zones without any
	// transition in the window (UTC, Asia/Tokyo, …) still get one child.
	comp.Children = append(comp.Children, observance(from, from.In(loc), from.In(loc)))

	// Scan for offset transitions: coarse hourly steps, then bisect to the
	// exact second. A three-year window costs ~26k cheap comparisons.
	prev := from
	_, prevOff := prev.In(loc).Zone()
	for t := from.Add(time.Hour); !t.After(to); t = t.Add(time.Hour) {
		_, off := t.In(loc).Zone()
		if off != prevOff {
			tr := bisectTransition(prev, t, prevOff, loc)
			before := tr.Add(-time.Second).In(loc)
			comp.Children = append(comp.Children, observance(tr, before, tr.In(loc)))
			prevOff = off
		}
		prev = t
	}
	return comp, nil
}

// observance builds a STANDARD/DAYLIGHT child for the transition at utc, where
// before/after are local times just before and at the transition.
func observance(utc time.Time, before, after time.Time) *ical.Component {
	name := ical.CompTimezoneStandard
	if after.IsDST() {
		name = ical.CompTimezoneDaylight
	}
	abbr, offTo := after.Zone()
	_, offFrom := before.Zone()

	child := ical.NewComponent(name)
	// DTSTART is the local wall-clock time at which the observance begins,
	// expressed in the previous (TZOFFSETFROM) offset — a floating value. The
	// offsets keep their default UTC-OFFSET value type, so set raw values
	// instead of SetText (which would tag them VALUE=TEXT).
	setRaw(child, ical.PropDateTimeStart, utc.Add(time.Duration(offFrom)*time.Second).UTC().Format("20060102T150405"))
	setRaw(child, ical.PropTimezoneOffsetFrom, fmtUTCOffset(offFrom))
	setRaw(child, ical.PropTimezoneOffsetTo, fmtUTCOffset(offTo))
	if abbr != "" {
		child.Props.SetText(ical.PropTimezoneName, abbr)
	}
	return child
}

// setRaw sets a property to a raw value without attaching a VALUE parameter.
func setRaw(c *ical.Component, name, value string) {
	prop := ical.NewProp(name)
	prop.Value = value
	c.Props.Set(prop)
}

// bisectTransition narrows an offset change known to happen in (lo, hi] down to
// the exact second.
func bisectTransition(lo, hi time.Time, loOff int, loc *time.Location) time.Time {
	for hi.Sub(lo) > time.Second {
		mid := lo.Add(hi.Sub(lo) / 2)
		if _, off := mid.In(loc).Zone(); off == loOff {
			lo = mid
		} else {
			hi = mid
		}
	}
	return hi
}

// fmtUTCOffset renders seconds east of UTC as ±hhmm (e.g. +0200).
func fmtUTCOffset(seconds int) string {
	sign := "+"
	if seconds < 0 {
		sign = "-"
		seconds = -seconds
	}
	return fmt.Sprintf("%s%02d%02d", sign, seconds/3600, (seconds%3600)/60)
}
