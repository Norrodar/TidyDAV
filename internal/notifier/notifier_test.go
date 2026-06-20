package notifier

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"sync/atomic"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/notify"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
)

const feedICS = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//up//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:1@up\r\nDTSTAMP:20260101T000000Z\r\nDTSTART:20260115T090000Z\r\nSUMMARY:Spam offer\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func logger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

func newStore(t *testing.T) *store.Store {
	t.Helper()
	st, err := store.Open(context.Background(), filepath.Join(t.TempDir(), "n.db"), logger())
	if err != nil {
		t.Fatalf("store.Open: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(context.Background()); err != nil {
		t.Fatalf("Migrate: %v", err)
	}
	return st
}

func upstreamServer(t *testing.T) *httptest.Server {
	t.Helper()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(feedICS))
	}))
	t.Cleanup(srv.Close)
	return srv
}

func TestNotifierDispatchesOncePerEvent(t *testing.T) {
	upstream := upstreamServer(t)

	var hits int32
	target := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&hits, 1)
	}))
	defer target.Close()

	st := newStore(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cfgJSON, _ := json.Marshal(notify.FeedNotifications{WebhookURL: target.URL, Triggers: []string{"filter"}})
	if err := st.CreateFeed(ctx, &store.Feed{
		ID: "f", UserID: "u", Name: "Waste", Secret: "s", TTLSeconds: 0,
		Sources:       []store.FeedSource{{URL: upstream.URL}},
		Rules:         []byte(`[{"type":"filter","filterMode":"blacklist","matchMode":"substring","pattern":"spam"}]`),
		Notifications: cfgJSON,
	}); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	feeds := feed.NewService(proxy.NewFetcher(st, logger(), true), logger())
	n := New(st, feeds, logger())

	if err := n.Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Fatalf("notifications after first run = %d, want 1", got)
	}

	// Second run: the same matched event must be de-duplicated.
	if err := n.Run(ctx); err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if got := atomic.LoadInt32(&hits); got != 1 {
		t.Errorf("notifications after second run = %d, want 1 (dedup)", got)
	}
}

func TestNotifierSkipsDisabledFeeds(t *testing.T) {
	upstream := upstreamServer(t)

	st := newStore(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	// No notification config -> feed is skipped, Run is a no-op.
	if err := st.CreateFeed(ctx, &store.Feed{
		ID: "f", UserID: "u", Name: "x", Secret: "s",
		Sources: []store.FeedSource{{URL: upstream.URL}},
		Rules:   []byte(`[{"type":"filter","filterMode":"blacklist","matchMode":"substring","pattern":"spam"}]`),
	}); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	feeds := feed.NewService(proxy.NewFetcher(st, logger(), true), logger())
	if err := New(st, feeds, logger()).Run(ctx); err != nil {
		t.Fatalf("Run: %v", err)
	}
}
