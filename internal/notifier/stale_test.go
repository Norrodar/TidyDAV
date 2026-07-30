package notifier

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/notify"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// sink is a counting webhook target: it records every delivered notification
// and can be made to fail on demand.
type sink struct {
	mu     sync.Mutex
	bodies []string
	status int // response status; 0 means 200
}

func newSink(t *testing.T) (*sink, string) {
	t.Helper()
	s := &sink{}
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		b, _ := io.ReadAll(r.Body)
		s.mu.Lock()
		s.bodies = append(s.bodies, string(b))
		code := s.status
		s.mu.Unlock()
		if code != 0 {
			w.WriteHeader(code)
		}
	}))
	t.Cleanup(srv.Close)
	return s, srv.URL
}

func (s *sink) count() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return len(s.bodies)
}

func (s *sink) all() []string {
	s.mu.Lock()
	defer s.mu.Unlock()
	return append([]string(nil), s.bodies...)
}

func (s *sink) fail(code int) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.status = code
}

// staleFixture wires a calendar whose ONLY notification trigger is the outage
// alert — no rule triggers at all, which is exactly the case that used to be
// skipped wholesale.
func staleFixture(t *testing.T, sourceURL, target string, hours int) *store.Store {
	t.Helper()
	st := newStore(t)
	ctx := context.Background()
	if err := st.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	cfgJSON, err := json.Marshal(notify.FeedNotifications{WebhookURL: target, SourceStaleHours: hours})
	if err != nil {
		t.Fatalf("marshal config: %v", err)
	}
	if err := st.CreateFeed(ctx, &store.Feed{
		ID: "f", UserID: "u", Name: "Abfall Rostock", Secret: "s",
		Sources:       []store.FeedSource{{URL: sourceURL}},
		Rules:         []byte(`[]`),
		Notifications: cfgJSON,
	}); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}
	return st
}

// fetchedAt records a successful upstream fetch at the given time.
func fetchedAt(t *testing.T, st *store.Store, url string, when time.Time) {
	t.Helper()
	if err := st.PutCachedFeed(context.Background(), &store.CachedFeed{
		Key: url, URL: url, Body: []byte(feedICS), FetchedAt: when,
	}); err != nil {
		t.Fatalf("PutCachedFeed: %v", err)
	}
}

// newAt builds a notifier reading a fake clock.
func newAt(st *store.Store, clock *time.Time) *Notifier {
	feeds := feed.NewService(proxy.NewFetcher(st, logger(), true), logger())
	n := New(st, feeds, logger(), true)
	n.now = func() time.Time { return *clock }
	return n
}

func TestStaleSourceNotifiesOnce(t *testing.T) {
	const src = "https://cal.example.org/waste.ics"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 24)
	fetchedAt(t, st, src, now.Add(-52*time.Hour))

	n := newAt(st, &now)
	for i := 0; i < 3; i++ {
		if err := n.Run(context.Background()); err != nil {
			t.Fatalf("Run %d: %v", i+1, err)
		}
	}
	if got := s.count(); got != 1 {
		t.Fatalf("notifications after 3 runs = %d, want 1", got)
	}
	body := s.all()[0]
	for _, want := range []string{"Abfall Rostock", src, "52h"} {
		if !strings.Contains(body, want) {
			t.Errorf("message does not mention %q: %s", want, body)
		}
	}
}

func TestStaleSourceMessageHidesCredentials(t *testing.T) {
	const src = "https://arne:hunter2@cal.example.org/waste.ics?token=supersecrettoken"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 24)
	fetchedAt(t, st, src, now.Add(-30*time.Hour))

	if err := newAt(st, &now).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := s.count(); got != 1 {
		t.Fatalf("notifications = %d, want 1", got)
	}
	body := s.all()[0]
	for _, secret := range []string{"hunter2", "supersecrettoken"} {
		if strings.Contains(body, secret) {
			t.Fatalf("notification leaked %q: %s", secret, body)
		}
	}
	if !strings.Contains(body, "cal.example.org/waste.ics") {
		t.Errorf("message does not identify the source: %s", body)
	}
}

func TestNeverFetchedSourceIsStale(t *testing.T) {
	const src = "https://cal.example.org/never.ics"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 24) // no cache entry at all

	n := newAt(st, &now)
	for i := 0; i < 2; i++ {
		if err := n.Run(context.Background()); err != nil {
			t.Fatalf("Run %d: %v", i+1, err)
		}
	}
	if got := s.count(); got != 1 {
		t.Fatalf("notifications = %d, want 1", got)
	}
	if body := s.all()[0]; !strings.Contains(body, "never been fetched") {
		t.Errorf("message does not explain that the source was never fetched: %s", body)
	}
}

// stale, stale, ok, ok, stale must produce exactly three messages: one outage,
// one all-clear, one outage again.
func TestStaleRecoveryAndRelapse(t *testing.T) {
	const src = "https://cal.example.org/waste.ics"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 24)
	n := newAt(st, &now)
	ctx := context.Background()

	run := func(step string) {
		t.Helper()
		if err := n.Run(ctx); err != nil {
			t.Fatalf("Run (%s): %v", step, err)
		}
	}

	fetchedAt(t, st, src, now.Add(-52*time.Hour))
	run("stale 1")
	run("stale 2")
	fetchedAt(t, st, src, now.Add(-1*time.Minute)) // source recovered
	run("ok 1")
	run("ok 2")
	fetchedAt(t, st, src, now.Add(-72*time.Hour)) // and died again
	run("stale 3")

	msgs := s.all()
	if len(msgs) != 3 {
		t.Fatalf("notifications = %d, want 3: %v", len(msgs), msgs)
	}
	if !strings.Contains(msgs[0], "has not updated") {
		t.Errorf("first message is not an outage warning: %s", msgs[0])
	}
	if !strings.Contains(msgs[1], "is updating again") {
		t.Errorf("second message is not an all-clear: %s", msgs[1])
	}
	if !strings.Contains(msgs[2], "has not updated") {
		t.Errorf("third message is not an outage warning: %s", msgs[2])
	}
}

// A notification nobody could receive must not count as announced.
func TestStaleNotificationRetriedAfterDeliveryFailure(t *testing.T) {
	const src = "https://cal.example.org/waste.ics"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 24)
	fetchedAt(t, st, src, now.Add(-52*time.Hour))
	n := newAt(st, &now)
	ctx := context.Background()

	s.fail(http.StatusInternalServerError)
	if err := n.Run(ctx); err != nil {
		t.Fatalf("Run 1: %v", err)
	}
	if got := s.count(); got != 1 {
		t.Fatalf("delivery attempts = %d, want 1", got)
	}
	if marked, err := st.IsNotified(ctx, "f", staleKey(src)); err != nil || marked {
		t.Fatalf("ledger entry survived a failed delivery (marked=%v, err=%v)", marked, err)
	}

	s.fail(0) // target is back
	if err := n.Run(ctx); err != nil {
		t.Fatalf("Run 2: %v", err)
	}
	if got := s.count(); got != 2 {
		t.Fatalf("delivery attempts after retry = %d, want 2", got)
	}
	if err := n.Run(ctx); err != nil {
		t.Fatalf("Run 3: %v", err)
	}
	if got := s.count(); got != 2 {
		t.Errorf("delivery attempts after a successful send = %d, want 2 (dedup)", got)
	}
}

// The outage alert must not disturb calendars that do not use it.
func TestStaleDisabledByDefault(t *testing.T) {
	const src = "https://cal.example.org/waste.ics"
	now := time.Now().UTC()
	s, target := newSink(t)
	st := staleFixture(t, src, target, 0) // threshold absent => alert off

	if err := newAt(st, &now).Run(context.Background()); err != nil {
		t.Fatalf("Run: %v", err)
	}
	if got := s.count(); got != 0 {
		t.Errorf("notifications = %d, want 0 while the alert is off", got)
	}
}
