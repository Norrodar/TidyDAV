package ics

import (
	"strings"
	"testing"
	"time"
)

func firstEvent(t *testing.T, vevent string) (cal string) {
	t.Helper()
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n" + vevent + "END:VCALENDAR\r\n"
}

func TestLastOccurrence(t *testing.T) {
	tests := []struct {
		name    string
		vevent  string
		bounded bool
		want    time.Time
	}{
		{
			name:    "plain event ends at DTEND",
			vevent:  "BEGIN:VEVENT\r\nUID:a\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nDTEND:20260106T100000Z\r\nEND:VEVENT\r\n",
			bounded: true,
			want:    time.Date(2026, 1, 6, 10, 0, 0, 0, time.UTC),
		},
		{
			name:    "open-ended RRULE is unbounded",
			vevent:  "BEGIN:VEVENT\r\nUID:b\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nDTEND:20260106T100000Z\r\nRRULE:FREQ=WEEKLY\r\nEND:VEVENT\r\n",
			bounded: false,
		},
		{
			name:    "COUNT-limited RRULE is treated as unbounded",
			vevent:  "BEGIN:VEVENT\r\nUID:c\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nDTEND:20260106T100000Z\r\nRRULE:FREQ=WEEKLY;COUNT=5\r\nEND:VEVENT\r\n",
			bounded: false,
		},
		{
			name:   "UNTIL bounds the series, extended by the duration",
			vevent: "BEGIN:VEVENT\r\nUID:d\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nDTEND:20260106T100000Z\r\nRRULE:FREQ=WEEKLY;UNTIL=20260331T090000Z\r\nEND:VEVENT\r\n",
			// UNTIL is the last start; the occurrence still runs its one hour.
			bounded: true,
			want:    time.Date(2026, 3, 31, 10, 0, 0, 0, time.UTC),
		},
		{
			name:   "date-only UNTIL",
			vevent: "BEGIN:VEVENT\r\nUID:e\r\nDTSTAMP:20260101T000000Z\r\nDTSTART;VALUE=DATE:20260106\r\nRRULE:FREQ=YEARLY;UNTIL=20300106\r\nEND:VEVENT\r\n",
			// An all-day occurrence runs until the next midnight (DTEND is
			// exclusive), so the final one ends on the 7th.
			bounded: true,
			want:    time.Date(2030, 1, 7, 0, 0, 0, 0, time.UTC),
		},
		{
			name:    "RDATE extends beyond DTEND",
			vevent:  "BEGIN:VEVENT\r\nUID:f\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nDTEND:20260106T100000Z\r\nRDATE:20300106T090000Z\r\nEND:VEVENT\r\n",
			bounded: true,
			want:    time.Date(2030, 1, 6, 9, 0, 0, 0, time.UTC),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			cal, err := Parse(strings.NewReader(firstEvent(t, tt.vevent)))
			if err != nil {
				t.Fatalf("parse: %v", err)
			}
			got, bounded := LastOccurrence(cal.Events()[0])
			if bounded != tt.bounded {
				t.Fatalf("bounded = %v, want %v (got %v)", bounded, tt.bounded, got)
			}
			if tt.bounded && !got.Equal(tt.want) {
				t.Errorf("last occurrence = %v, want %v", got, tt.want)
			}
		})
	}
}

func TestIsRecurring(t *testing.T) {
	rec := "BEGIN:VEVENT\r\nUID:a\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nRRULE:FREQ=WEEKLY\r\nEND:VEVENT\r\n"
	plain := "BEGIN:VEVENT\r\nUID:b\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260106T090000Z\r\nEND:VEVENT\r\n"

	cal, err := Parse(strings.NewReader(firstEvent(t, rec+plain)))
	if err != nil {
		t.Fatalf("parse: %v", err)
	}
	if !IsRecurring(cal.Events()[0]) {
		t.Error("event with RRULE should be recurring")
	}
	if IsRecurring(cal.Events()[1]) {
		t.Error("event without RRULE/RDATE should not be recurring")
	}
}
