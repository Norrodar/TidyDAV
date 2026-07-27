package dav

import (
	"context"
	"errors"
	"fmt"
	"testing"
)

// vevent builds a minimal one-event iCalendar body.
func vevent(uid, summary, date string) string {
	return "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//t//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:" + uid + "\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART;VALUE=DATE:" + date + "\r\nSUMMARY:" + summary + "\r\nEND:VEVENT\r\nEND:VCALENDAR\r\n"
}

func uids(entries []PreviewEntry) map[string]bool {
	m := make(map[string]bool, len(entries))
	for _, e := range entries {
		m[e.UID] = true
	}
	return m
}

func TestPreviewMergeAToB(t *testing.T) {
	ctx := context.Background()
	a, b := newFake(), newFake()
	a.set("/a1", "1", vevent("u1", "A One", "20260115"))
	a.set("/a2", "2", vevent("u2", "A Two", "20260116"))
	b.set("/b2", "3", vevent("u2", "B Two", "20260116"))
	b.set("/b3", "4", vevent("u3", "B Three", "20260117"))

	aOut, bOut, merged, err := PreviewMerge(ctx, a, b, Options{Direction: AToB, UID: CalendarUID}, "caldav")
	if err != nil {
		t.Fatalf("PreviewMerge: %v", err)
	}
	if len(aOut) != 2 || len(bOut) != 2 {
		t.Fatalf("side counts = %d/%d, want 2/2", len(aOut), len(bOut))
	}
	// merged = union of u1,u2,u3 with A authoritative on u2.
	if len(merged) != 3 {
		t.Fatalf("merged count = %d, want 3", len(merged))
	}
	got := uids(merged)
	for _, u := range []string{"u1", "u2", "u3"} {
		if !got[u] {
			t.Errorf("merged missing %s", u)
		}
	}
	for _, e := range merged {
		if e.UID == "u2" && e.Title != "A Two" {
			t.Errorf("u2 title = %q, want A authoritative \"A Two\"", e.Title)
		}
	}
}

func TestPreviewMergeBToA(t *testing.T) {
	ctx := context.Background()
	a, b := newFake(), newFake()
	a.set("/a2", "1", vevent("u2", "A Two", "20260116"))
	b.set("/b2", "2", vevent("u2", "B Two", "20260116"))

	_, _, merged, err := PreviewMerge(ctx, a, b, Options{Direction: BToA, UID: CalendarUID}, "caldav")
	if err != nil {
		t.Fatalf("PreviewMerge: %v", err)
	}
	if len(merged) != 1 || merged[0].Title != "B Two" {
		t.Fatalf("merged = %+v, want single B-authoritative entry", merged)
	}
}

func TestPreviewMergeWindow(t *testing.T) {
	ctx := context.Background()
	a, b := newFake(), newFake()
	a.set("/in", "1", vevent("in", "In", "20260115"))
	a.set("/out", "2", vevent("out", "Out", "20260301"))

	start, end, _ := ParseWindow("2026-01-01", "2026-01-31")
	aOut, _, _, err := PreviewMerge(ctx, a, b, Options{Direction: AToB, UID: CalendarUID, WindowStart: start, WindowEnd: end}, "caldav")
	if err != nil {
		t.Fatalf("PreviewMerge: %v", err)
	}
	if len(aOut) != 1 || aOut[0].UID != "in" {
		t.Fatalf("windowed A = %+v, want only the in-window event", aOut)
	}
}

// A collection that suddenly lists nothing is treated as a broken endpoint, not
// as "the user deleted everything" — otherwise a moved collection URL wipes the
// other side.
func TestSyncRefusesWhenCollectionVanishes(t *testing.T) {
	ctx := context.Background()
	src, dst := newFake(), newFake()
	opts := Options{Direction: AToB, UID: CalendarUID}
	st := NewState()

	for i := 0; i < vanishGuardThreshold; i++ {
		uid := fmt.Sprintf("u%d", i)
		src.set("/"+uid, "1", vevent(uid, "Event "+uid, "20260115"))
	}
	if _, err := Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("initial sync: %v", err)
	}
	if len(dst.items) != vanishGuardThreshold {
		t.Fatalf("dst has %d items, want %d", len(dst.items), vanishGuardThreshold)
	}

	// The source now lists nothing at all.
	src.items = map[string]Item{}
	_, err := Sync(ctx, src, dst, st, opts)
	if !errors.Is(err, ErrCollectionVanished) {
		t.Fatalf("sync error = %v, want ErrCollectionVanished", err)
	}
	if len(dst.items) != vanishGuardThreshold {
		t.Errorf("destination was modified despite the guard: %d items left", len(dst.items))
	}

	// Deleting a handful by hand still propagates normally.
	small, smallDst := newFake(), newFake()
	smallState := NewState()
	small.set("/only", "1", vevent("only", "Only", "20260115"))
	if _, err := Sync(ctx, small, smallDst, smallState, opts); err != nil {
		t.Fatalf("small sync: %v", err)
	}
	small.items = map[string]Item{}
	if _, err := Sync(ctx, small, smallDst, smallState, opts); err != nil {
		t.Fatalf("small delete sync: %v", err)
	}
	if len(smallDst.items) != 0 {
		t.Error("a genuine small deletion should still propagate")
	}
}

func TestSyncWindowFiltersAndProtects(t *testing.T) {
	ctx := context.Background()
	src, dst := newFake(), newFake()
	start, end, _ := ParseWindow("2026-01-01", "2026-01-31")
	opts := Options{Direction: AToB, UID: CalendarUID, WindowStart: start, WindowEnd: end}

	src.set("/in", "1", vevent("in", "In", "20260115"))
	src.set("/out", "2", vevent("out", "Out", "20260301"))

	res, err := Sync(ctx, src, dst, NewState(), opts)
	if err != nil {
		t.Fatalf("sync: %v", err)
	}
	if (res != Result{Created: 1}) {
		t.Fatalf("result = %+v, want only the in-window event created", res)
	}
	if len(dst.items) != 1 {
		t.Fatalf("dst has %d items, want 1 (out-of-window skipped)", len(dst.items))
	}

	// Pre-seed dst with an out-of-window item lacking state: it must not be deleted.
	dst.set("/preexisting", "9", vevent("pre", "Pre", "20260401"))
	st := NewState()
	if _, err := Sync(ctx, src, dst, st, opts); err != nil {
		t.Fatalf("resync: %v", err)
	}
	if _, ok := dst.items["/preexisting"]; !ok {
		t.Error("out-of-window destination item was deleted; window must protect it")
	}
}
