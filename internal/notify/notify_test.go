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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if ct := r.Header.Get("Content-Type"); ct != "application/json" {
			t.Errorf("content-type = %q", ct)
		}
		_ = json.NewDecoder(r.Body).Decode(&got)
	}))
	defer srv.Close()

	if err := NewWebhookNotifier(srv.URL).Notify(context.Background(), sampleEvent); err != nil {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		titleHdr = r.Header.Get("Title")
		b, _ := io.ReadAll(r.Body)
		body = string(b)
	}))
	defer srv.Close()

	if err := NewNtfyNotifier(srv.URL, "mytopic").Notify(context.Background(), sampleEvent); err != nil {
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
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path = r.URL.Path
		token = r.URL.Query().Get("token")
		_ = json.NewDecoder(r.Body).Decode(&payload)
	}))
	defer srv.Close()

	if err := NewGotifyNotifier(srv.URL, "tok123").Notify(context.Background(), sampleEvent); err != nil {
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
	if err := NewWebhookNotifier(srv.URL).Notify(context.Background(), sampleEvent); err == nil {
		t.Fatal("expected error on 500 response")
	}
}

func TestDispatcherToleratesFailure(t *testing.T) {
	var goodHits int32
	good := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		atomic.AddInt32(&goodHits, 1)
	}))
	defer good.Close()
	bad := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadGateway)
	}))
	defer bad.Close()

	d := NewDispatcher(testLogger())
	d.Add(NewWebhookNotifier(bad.URL))
	d.Add(NewWebhookNotifier(good.URL))
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
	}, testLogger())
	if d.Len() != 3 {
		t.Errorf("Len = %d, want 3", d.Len())
	}
	if NewFromConfig(Config{NtfyServer: "https://ntfy.sh"}, testLogger()).Len() != 0 {
		t.Error("ntfy without topic should not register")
	}
}

func TestErrorDoesNotLeakToken(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	err := NewGotifyNotifier(srv.URL, "supersecret").Notify(context.Background(), sampleEvent)
	if err == nil {
		t.Fatal("expected error")
	}
	if strings.Contains(err.Error(), "supersecret") {
		t.Errorf("error leaked the gotify token: %v", err)
	}
}
