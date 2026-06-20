package dav

import "testing"

func TestCalendarUIDAndModified(t *testing.T) {
	data := []byte("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:abc@example.com\r\nDTSTAMP:20260101T000000Z\r\n" +
		"LAST-MODIFIED:20260102T100000Z\r\nDTSTART:20260115T090000Z\r\nSUMMARY:X\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n")

	if uid := CalendarUID(data); uid != "abc@example.com" {
		t.Errorf("CalendarUID = %q, want abc@example.com", uid)
	}

	m := CalendarModified(data)
	if m.IsZero() || m.UTC().Format("2006-01-02") != "2026-01-02" {
		t.Errorf("CalendarModified = %v, want 2026-01-02", m)
	}

	if CalendarUID([]byte("not a calendar")) != "" {
		t.Error("unparseable body should yield empty UID")
	}
	if !CalendarModified([]byte("not a calendar")).IsZero() {
		t.Error("unparseable body should yield zero time")
	}
}

func TestContactUIDAndModified(t *testing.T) {
	data := []byte("BEGIN:VCARD\r\nVERSION:4.0\r\nUID:urn:uuid:abc\r\nFN:Jane\r\nREV:20260102T100000Z\r\nEND:VCARD\r\n")

	if uid := ContactUID(data); uid != "urn:uuid:abc" {
		t.Errorf("ContactUID = %q, want urn:uuid:abc", uid)
	}
	m := ContactModified(data)
	if m.IsZero() || m.UTC().Format("2006-01-02") != "2026-01-02" {
		t.Errorf("ContactModified = %v, want 2026-01-02", m)
	}
	if ContactUID([]byte("not a vcard")) != "" {
		t.Error("unparseable body should yield empty UID")
	}
}
