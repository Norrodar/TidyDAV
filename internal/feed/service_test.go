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
	return NewService(proxy.NewFetcher(st, logger), logger)
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

func TestRenderNoSourcesIsEmpty(t *testing.T) {
	out, err := newSvc(t).Render(context.Background(), &store.Feed{ID: "f4", Secret: "s4"})
	if err != nil {
		t.Fatalf("Render: %v", err)
	}
	if !strings.Contains(string(out), "BEGIN:VCALENDAR") {
		t.Errorf("expected empty calendar, got:\n%s", out)
	}
}
