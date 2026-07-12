package feed

import (
	"context"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

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

func TestRenderNoSourcesIsEmpty(t *testing.T) {
	out, err := newSvc(t).Render(context.Background(), &store.Feed{ID: "f4", Secret: "s4"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), "BEGIN:VCALENDAR") {
		t.Errorf("expected empty calendar, got:\n%s", out)
	}
}
