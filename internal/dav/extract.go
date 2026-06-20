package dav

import (
	"bytes"
	"time"

	"github.com/Norrodar/TidyDAV/internal/ics"
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
