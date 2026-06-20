package dav

import (
	"bytes"
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
