package notify

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func testLogger() *slog.Logger { return slog.New(slog.NewTextHandler(io.Discard, nil)) }

var sampleEvent = Event{Feed: "Müll", Rule: "filter", Summary: "Schwarze Tonne", Message: "matched", Time: time.Now()}

func TestWebhookNotify(t *testing.T) {
	var got Event
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	if err := NewWebhookNotifier(srv.URL, true).Notify(context.Background(), sampleEvent); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if got.Feed != "Müll" || got.Rule != "filter" {
		t.Errorf("webhook payload = %+v", got)
	}
}

func TestNtfyNotify(t *testing.T) {
	var (
		path, titleHdr, body string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		titleHdr = r.Header.Get("Title")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()

	if err := NewNtfyNotifier(srv.URL, "mytopic", true).Notify(context.Background(), sampleEvent); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if path != "/mytopic" {
		t.Errorf("path = %q, want /mytopic", path)
	}
	if titleHdr != "Schwarze Tonne" {
		t.Errorf("Title = %q", titleHdr)
	}
	if body != "matched" {
		t.Errorf("body = %q", body)
	}
}

func TestGotifyNotify(t *testing.T) {
	var (
		path, token string
		payload     map[string]string
	)
	srv := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		token = r.URL.Query().Get("token")
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}))
	defer srv.Close()

	if err := NewGotifyNotifier(srv.URL, "tok123", true).Notify(context.Background(), sampleEvent); err != nil {
		t.Fatalf("Notify: %v", err)
	}
	if path != "/message" || token != "tok123" {
		t.Errorf("path=%q token=%q", path, token)
	}
	if payload["message"] != "matched" || payload["title"] != "Schwarze Tonne" {
		t.Errorf("payload = %v", payload)
	}
}

func TestNotifyNon2xx(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()
	if err := NewWebhookNotifier(srv.URL, true).Notify(context.Background(), sampleEvent); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestDispatcherToleratesFailure(t *testing.T) {
	var goodHits int32
	good := httptest.NewServer(http.HandlerFunc(func(_ http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&goodHits, 1)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()

	d := NewDispatcher(testLogger())
	d.Add(NewWebhookNotifier(bad.URL, true))
	d.Add(NewWebhookNotifier(good.URL, true))
	if d.Len() != 2 {
		t.Fatalf("Len = %d, want 2", d.Len())
	}
	d.Dispatch(context.Background(), sampleEvent) // must not panic despite bad target
	if atomic.LoadInt32(&goodHits) != 1 {
		t.Errorf("good notifier hit %d times, want 1", goodHits)
	}
}

func TestNewFromConfig(t *testing.T) {
	d := NewFromConfig(Config{
		WebhookURL:   "https://hook",
		NtfyServer:   "https://ntfy.sh",
		NtfyTopic:    "t",
		GotifyServer: "https://gotify",
		GotifyToken:  "tok",
	}, testLogger(), true)
	if d.Len() != 3 {
		t.Errorf("Len = %d, want 3", d.Len())
	}
	if NewFromConfig(Config{NtfyServer: "https://ntfy.sh"}, testLogger(), true).Len() != 0 {
		t.Error("ntfy without topic should not register")
	}
}

func TestErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := NewGotifyNotifier(srv.URL, "supersecret", true).Notify(context.Background(), sampleEvent)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Errorf("error leaked the gotify token: %v", err)
	}
}

// A feed stored before the outage alert existed must decode to "off" and keep
// behaving exactly as before.
func TestFeedNotificationsLegacyJSON(t *testing.T) {
	const stored = `{"webhookUrl":"https://hook.example.com/x","ntfyServer":"https://ntfy.sh","ntfyTopic":"waste","triggers":["filter","rename"]}`

	var cfg FeedNotifications
	if err := json.Unmarshal([]byte(stored), &cfg); err != nil {
		t.Fatalf("Unmarshal: %v", err)
	}
	if cfg.SourceStaleHours != 0 {
		t.Errorf("SourceStaleHours = %d, want 0 for a config without the field", cfg.SourceStaleHours)
	}
	if !cfg.HasTarget() || !cfg.Enabled() {
		t.Error("existing behaviour changed: HasTarget/Enabled must still be true")
	}
	if cfg.StaleEnabled() {
		t.Error("outage alert must stay off when the threshold is absent")
	}
	if !cfg.Triggered("filter") || cfg.Triggered("dedup") {
		t.Error("triggers decoded incorrectly")
	}
}

func TestStaleEnabled(t *testing.T) {
	tests := []struct {
		name string
		cfg  FeedNotifications
		want bool
	}{
		{"threshold without target", FeedNotifications{SourceStaleHours: 24}, false},
		{"target without threshold", FeedNotifications{WebhookURL: "https://hook"}, false},
		{"negative threshold", FeedNotifications{WebhookURL: "https://hook", SourceStaleHours: -1}, false},
		{"both", FeedNotifications{WebhookURL: "https://hook", SourceStaleHours: 1}, true},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if got := tc.cfg.StaleEnabled(); got != tc.want {
				t.Errorf("StaleEnabled() = %v, want %v", got, tc.want)
			}
		})
	}
}

func TestRedactURL(t *testing.T) {
	got := RedactURL("https://user:hunter2@cal.example.org/waste.ics?token=supersecrettoken")
	for _, secret := range []string{"hunter2", "supersecrettoken"} {
		if strings.Contains(got, secret) {
			t.Errorf("RedactURL leaked %q: %s", secret, got)
		}
	}
	if !strings.Contains(got, "cal.example.org/waste.ics") {
		t.Errorf("RedactURL dropped the identifying part: %s", got)
	}
	if got := RedactURL("://not a url"); strings.Contains(got, "not a url") {
		t.Errorf("unparsable URL should be fully redacted, got %q", got)
	}
}
