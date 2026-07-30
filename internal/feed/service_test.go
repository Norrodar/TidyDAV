package feed

import (
	"bytes"
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/ics"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
)

const upstreamICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//up//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:1@up\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Keep\r\nDESCRIPTION:secret\r\nEND:VEVENT\r\n" +
	"BEGIN:VEVENT\r\nUID:2@up\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Spam\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func newSvc(t *testing.T) *Service {
	t.Helper()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "feed.db"), logger)
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return NewService(proxy.NewFetcher(st, logger, true), logger)
}

func upstreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(upstreamICS))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestRenderAppliesPipeline(t *testing.T) {
	srv := upstreamServer(t)
	f := &store.Feed{
		ID: "f1", Secret: "s1", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: srv.URL}},
		Rules: []byte(`[
			{"type":"filter","filterMode":"blacklist","matchMode":"substring","pattern":"spam"},
			{"type":"strip","fields":["DESCRIPTION"]}
		]`),
	}
	out, err := newSvc(t).Render(context.Background(), f)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "SUMMARY:Keep") {
		t.Errorf("kept event missing:\n%s", s)
	}
	if strings.Contains(s, "Spam") {
		t.Errorf("blacklisted event not removed:\n%s", s)
	}
	if strings.Contains(s, "secret") {
		t.Errorf("DESCRIPTION not stripped:\n%s", s)
	}
}

func TestRenderMergeDedupByUID(t *testing.T) {
	srv := upstreamServer(t)
	f := &store.Feed{
		ID: "f2", Secret: "s2", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: srv.URL}, {URL: srv.URL}},
	}
	out, err := newSvc(t).Render(context.Background(), f)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if n := strings.Count(string(out), "UID:1@up"); n != 1 {
		t.Errorf("UID 1@up appears %d times, want 1 (merge dedup)", n)
	}
}

func TestRenderEmptyWhenAllFiltered(t *testing.T) {
	srv := upstreamServer(t)
	f := &store.Feed{
		ID: "f3", Secret: "s3", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: srv.URL}},
		Rules:   []byte(`[{"type":"filter","filterMode":"whitelist","matchMode":"substring","pattern":"no-such-event"}]`),
	}
	out, err := newSvc(t).Render(context.Background(), f)
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "BEGIN:VCALENDAR") || strings.Contains(s, "BEGIN:VEVENT") {
		t.Errorf("expected a valid empty calendar, got:\n%s", s)
	}
}

// Some real-world feeds (e.g. municipal waste calendars) omit UID and DTSTAMP,
// which go-ical requires on encode. Render must synthesize them: stable across
// fetches, distinct for exact duplicates.
func TestRenderSynthesizesMissingUIDAndDTStamp(t *testing.T) {
	noUID := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nSUMMARY:Bin day\r\nDTSTART;VALUE=DATE:20260102\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nSUMMARY:Bin day\r\nDTSTART;VALUE=DATE:20260102\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(noUID))
	}))
	t.Cleanup(srv.Close)

	svc := newSvc(t)
	f := &store.Feed{
		ID: "f5", Secret: "s5", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: srv.URL}},
	}
	out, err := svc.Render(context.Background(), f)
	if err != nil {
		t.Fatalf("Render with UID-less events: %v", err)
	}
	s := string(out)
	if n := strings.Count(s, "BEGIN:VEVENT"); n != 2 {
		t.Fatalf("event count = %d, want 2 (duplicates kept)", n)
	}
	if n := strings.Count(s, "UID:"); n != 2 {
		t.Fatalf("UID count = %d, want 2 (synthesized)", n)
	}
	if n := strings.Count(s, "DTSTAMP:"); n != 2 {
		t.Fatalf("DTSTAMP count = %d, want 2 (synthesized)", n)
	}

	// UIDs must be stable across fetches and distinct within a fetch.
	out2, err := svc.Render(context.Background(), f)
	if err != nil {
		t.Fatalf("second Render: %v", err)
	}
	uids := func(s string) []string {
		var got []string
		for _, line := range strings.Split(s, "\r\n") {
			if strings.HasPrefix(line, "UID:") {
				got = append(got, line)
			}
		}
		return got
	}
	first, second := uids(s), uids(string(out2))
	if len(first) != 2 || first[0] == first[1] {
		t.Errorf("duplicate events got same UID: %v", first)
	}
	if first[0] != second[0] || first[1] != second[1] {
		t.Errorf("synthetic UIDs not stable across fetches:\n%v\n%v", first, second)
	}
}

// Every TZID referenced in the output must have a VTIMEZONE: upstream
// definitions are reused, and zones introduced by the timezone rule are
// generated from tzdata.
func TestRenderAttachesTimezones(t *testing.T) {
	upstream := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//up//EN\r\n" +
		"BEGIN:VTIMEZONE\r\nTZID:Europe/Brussels\r\n" +
		"BEGIN:STANDARD\r\nDTSTART:19701025T030000\r\nTZOFFSETFROM:+0200\r\nTZOFFSETTO:+0100\r\nEND:STANDARD\r\n" +
		"END:VTIMEZONE\r\n" +
		"BEGIN:VEVENT\r\nUID:tz1@up\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Talk\r\n" +
		"DTSTART;TZID=Europe/Brussels:20260706T100000\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(upstream))
	}))
	t.Cleanup(srv.Close)
	svc := newSvc(t)

	// Passthrough: the upstream VTIMEZONE must be preserved.
	out, err := svc.Render(context.Background(), &store.Feed{
		ID: "tz1", Secret: "tz1", Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if s := string(out); !strings.Contains(s, "TZID:Europe/Brussels") || !strings.Contains(s, "BEGIN:VTIMEZONE") {
		t.Errorf("upstream VTIMEZONE not preserved:\n%s", s)
	}

	// Timezone rule: the new target zone must get a generated VTIMEZONE.
	out, err = svc.Render(context.Background(), &store.Feed{
		ID: "tz2", Secret: "tz2", Sources: []store.FeedSource{{URL: srv.URL}},
		Rules: []byte(`[{"type":"timezone","target":"Europe/Berlin"}]`),
	})
	if err != nil {
		t.Fatalf("Render with timezone rule: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "TZID:Europe/Berlin") {
		t.Errorf("generated VTIMEZONE for rule target missing:\n%s", s)
	}
	if strings.Contains(s, "TZID:Europe/Brussels\r\n") {
		t.Errorf("unreferenced upstream VTIMEZONE should not be attached:\n%s", s)
	}
}

// A recurrence override (a moved or cancelled instance) carries the series
// UID plus a RECURRENCE-ID. Merging must keep both, or clients show the
// instance at its original time / resurrect a cancelled one.
func TestRenderKeepsRecurrenceOverrides(t *testing.T) {
	upstream := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//up//EN\r\n" +
		"BEGIN:VEVENT\r\nUID:series@up\r\nDTSTAMP:20260101T000000Z\r\n" +
		"DTSTART:20260105T090000Z\r\nRRULE:FREQ=WEEKLY\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:series@up\r\nDTSTAMP:20260101T000000Z\r\n" +
		"RECURRENCE-ID:20260112T090000Z\r\nDTSTART:20260112T140000Z\r\nSUMMARY:Standup (moved)\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nUID:series@up\r\nDTSTAMP:20260101T000000Z\r\n" +
		"RECURRENCE-ID:20260119T090000Z\r\nDTSTART:20260119T090000Z\r\nSTATUS:CANCELLED\r\nSUMMARY:Standup\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(upstream))
	}))
	t.Cleanup(srv.Close)

	out, err := newSvc(t).Render(context.Background(), &store.Feed{
		ID: "rec", Secret: "rec", Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if n := strings.Count(s, "BEGIN:VEVENT"); n != 3 {
		t.Fatalf("event count = %d, want 3 (master + two overrides)", n)
	}
	if !strings.Contains(s, "Standup (moved)") || !strings.Contains(s, "STATUS:CANCELLED") {
		t.Errorf("overrides were dropped by UID dedup:\n%s", s)
	}

	// The same source twice must still collapse to one copy of each.
	out, err = newSvc(t).Render(context.Background(), &store.Feed{
		ID: "rec2", Secret: "rec2",
		Sources: []store.FeedSource{{URL: srv.URL}, {URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render (duplicate source): %v", err)
	}
	if n := strings.Count(string(out), "BEGIN:VEVENT"); n != 3 {
		t.Errorf("duplicate source produced %d events, want 3", n)
	}
}

// The served calendar must identify itself: subscribers of /ics/<secret>
// otherwise get a nameless calendar and guess their own poll interval.
func TestRenderPublishesCalendarName(t *testing.T) {
	srv := upstreamServer(t)
	out, err := newSvc(t).Render(context.Background(), &store.Feed{
		ID: "n1", Secret: "n1", Name: "Waste Rostock",
		Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "X-WR-CALNAME:Waste Rostock") {
		t.Errorf("X-WR-CALNAME missing:\n%s", s)
	}
	if !strings.Contains(s, "NAME:Waste Rostock") {
		t.Errorf("NAME missing:\n%s", s)
	}
}

// The name is a TEXT value: separators must be escaped on the way out and come
// back unchanged when a client parses the feed.
func TestRenderEscapesCalendarName(t *testing.T) {
	const name = `a,b;c\d`
	srv := upstreamServer(t)

	for _, tc := range []struct {
		label   string
		sources []store.FeedSource
	}{
		{"with events", []store.FeedSource{{URL: srv.URL}}},
		{"without events", nil},
	} {
		t.Run(tc.label, func(t *testing.T) {
			out, err := newSvc(t).Render(context.Background(), &store.Feed{
				ID: "esc", Secret: "esc", Name: name, Sources: tc.sources,
			})
			if err != nil {
				t.Fatalf("Render: %v", err)
			}
			if !strings.Contains(string(out), `X-WR-CALNAME:a\,b\;c\\d`) {
				t.Errorf("name not RFC 5545 escaped:\n%s", out)
			}
			cal, err := ics.Parse(bytes.NewReader(out))
			if err != nil {
				t.Fatalf("re-parse rendered feed: %v", err)
			}
			for _, prop := range []string{"X-WR-CALNAME", "NAME"} {
				got, err := cal.Props.Text(prop)
				if err != nil {
					t.Fatalf("read %s: %v", prop, err)
				}
				if got != name {
					t.Errorf("%s round-trip = %q, want %q", prop, got, name)
				}
			}
		})
	}
}

// A blank name must not produce empty properties — some clients render those as
// a nameless calendar rather than falling back to the URL.
func TestRenderOmitsBlankCalendarName(t *testing.T) {
	srv := upstreamServer(t)
	out, err := newSvc(t).Render(context.Background(), &store.Feed{
		ID: "blank", Secret: "blank", Name: "   ",
		Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if s := string(out); strings.Contains(s, "X-WR-CALNAME") || strings.Contains(s, "\r\nNAME") {
		t.Errorf("blank name should emit no name property:\n%s", s)
	}
}

// The cache TTL is what the feed can actually deliver, so it is what clients are
// asked to poll at. Without a TTL nothing is claimed at all.
func TestRenderPublishesRefreshInterval(t *testing.T) {
	srv := upstreamServer(t)
	out, err := newSvc(t).Render(context.Background(), &store.Feed{
		ID: "ttl1", Secret: "ttl1", Name: "Hourly", TTLSeconds: 3600,
		Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if !strings.Contains(s, "REFRESH-INTERVAL;VALUE=DURATION:PT1H") {
		t.Errorf("REFRESH-INTERVAL missing:\n%s", s)
	}
	if !strings.Contains(s, "X-PUBLISHED-TTL:PT1H") {
		t.Errorf("X-PUBLISHED-TTL missing:\n%s", s)
	}

	out, err = newSvc(t).Render(context.Background(), &store.Feed{
		ID: "ttl0", Secret: "ttl0", Name: "Uncached", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: srv.URL}},
	})
	if err != nil {
		t.Fatalf("Render without TTL: %v", err)
	}
	s = string(out)
	if strings.Contains(s, "REFRESH-INTERVAL") || strings.Contains(s, "X-PUBLISHED-TTL") {
		t.Errorf("TTL 0 must emit no refresh properties at all:\n%s", s)
	}
}

// Filtering every event away must not cost the calendar its identity.
func TestRenderEventlessKeepsIdentity(t *testing.T) {
	srv := upstreamServer(t)
	out, err := newSvc(t).Render(context.Background(), &store.Feed{
		ID: "e1", Secret: "e1", Name: "Nothing here", TTLSeconds: 5400,
		Sources: []store.FeedSource{{URL: srv.URL}},
		Rules:   []byte(`[{"type":"filter","filterMode":"whitelist","matchMode":"substring","pattern":"no-such-event"}]`),
	})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	s := string(out)
	if strings.Contains(s, "BEGIN:VEVENT") {
		t.Fatalf("expected an event-less calendar:\n%s", s)
	}
	for _, want := range []string{
		"BEGIN:VCALENDAR", "END:VCALENDAR", "PRODID:-//TidyDAV//EN", "VERSION:2.0",
		"X-WR-CALNAME:Nothing here", "NAME:Nothing here",
		"REFRESH-INTERVAL;VALUE=DURATION:PT1H30M", "X-PUBLISHED-TTL:PT1H30M",
	} {
		if !strings.Contains(s, want) {
			t.Errorf("event-less calendar is missing %q:\n%s", want, s)
		}
	}
	if _, err := ics.Parse(bytes.NewReader(out)); err != nil {
		t.Errorf("event-less calendar does not parse: %v\n%s", err, s)
	}
}

func TestRenderNoSourcesIsEmpty(t *testing.T) {
	out, err := newSvc(t).Render(context.Background(), &store.Feed{ID: "f4", Secret: "s4"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), "BEGIN:VCALENDAR") {
		t.Errorf("expected empty calendar, got:\n%s", out)
	}
}
