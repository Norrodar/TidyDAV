// Package notify sends notifications (webhook, ntfy, Gotify) when something of
// interest happens, e.g. a feed rule matches.
package notify

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"
)

// Event describes what to notify about.
type Event struct {
	Feed    string    `json:"feed"`
	Rule    string    `json:"rule"`
	Summary string    `json:"summary"`
	Message string    `json:"message"`
	Time    time.Time `json:"time"`
}

// Notifier delivers a single notification.
type Notifier interface {
	Notify(ctx context.Context, ev Event) error
	Kind() string
}

func newClient() *http.Client { return &http.Client{Timeout: 10 * time.Second} }

func post(ctx context.Context, client *http.Client, url, contentType string, body []byte, headers map[string]string) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(body))
	if err != nil {
		return err
	}
	if contentType != "" {
		req.Header.Set("Content-Type", contentType)
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}
	resp, err := client.Do(req)
	if err != nil {
		return err
	}
	defer func() { _ = resp.Body.Close() }()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return fmt.Errorf("notify: %s returned status %d", redactURL(url), resp.StatusCode)
	}
	return nil
}

// redactURL strips the query string (e.g. a Gotify ?token=) and masks any
// userinfo password so credentials never reach logs.
func redactURL(raw string) string {
	u, err := url.Parse(raw)
	if err != nil {
		return "[redacted]"
	}
	u.RawQuery = ""
	return u.Redacted()
}

func title(ev Event) string {
	if ev.Summary != "" {
		return ev.Summary
	}
	return "TidyDAV"
}

// ── Webhook ──────────────────────────────────────────────────────────────────

// WebhookNotifier POSTs the event as JSON to a URL.
type WebhookNotifier struct {
	url    string
	client *http.Client
}

// NewWebhookNotifier creates a webhook notifier.
func NewWebhookNotifier(url string) *WebhookNotifier {
	return &WebhookNotifier{url: url, client: newClient()}
}

// Kind implements Notifier.
func (n *WebhookNotifier) Kind() string { return "webhook" }

// Notify implements Notifier.
func (n *WebhookNotifier) Notify(ctx context.Context, ev Event) error {
	body, err := json.Marshal(ev)
	if err != nil {
		return err
	}
	return post(ctx, n.client, n.url, "application/json", body, nil)
}

// ── ntfy ─────────────────────────────────────────────────────────────────────

// NtfyNotifier posts to an ntfy server/topic.
type NtfyNotifier struct {
	url    string
	client *http.Client
}

// NewNtfyNotifier creates an ntfy notifier for server (e.g. https://ntfy.sh) and topic.
func NewNtfyNotifier(server, topic string) *NtfyNotifier {
	return &NtfyNotifier{url: strings.TrimRight(server, "/") + "/" + topic, client: newClient()}
}

// Kind implements Notifier.
func (n *NtfyNotifier) Kind() string { return "ntfy" }

// Notify implements Notifier.
func (n *NtfyNotifier) Notify(ctx context.Context, ev Event) error {
	return post(ctx, n.client, n.url, "text/plain", []byte(ev.Message), map[string]string{"Title": title(ev)})
}

// ── Gotify ───────────────────────────────────────────────────────────────────

// GotifyNotifier posts to a Gotify server using an application token.
type GotifyNotifier struct {
	url    string
	client *http.Client
}

// NewGotifyNotifier creates a Gotify notifier for server and application token.
func NewGotifyNotifier(server, token string) *GotifyNotifier {
	return &GotifyNotifier{url: strings.TrimRight(server, "/") + "/message?token=" + token, client: newClient()}
}

// Kind implements Notifier.
func (n *GotifyNotifier) Kind() string { return "gotify" }

// Notify implements Notifier.
func (n *GotifyNotifier) Notify(ctx context.Context, ev Event) error {
	body, err := json.Marshal(map[string]string{"title": title(ev), "message": ev.Message})
	if err != nil {
		return err
	}
	return post(ctx, n.client, n.url, "application/json", body, nil)
}

// ── Dispatcher ───────────────────────────────────────────────────────────────

// Dispatcher fans an event out to several notifiers, logging (not returning)
// individual failures so one bad target never blocks the others.
type Dispatcher struct {
	notifiers []Notifier
	log       *slog.Logger
}

// NewDispatcher creates an empty dispatcher.
func NewDispatcher(log *slog.Logger) *Dispatcher {
	return &Dispatcher{log: log}
}

// Add registers a notifier.
func (d *Dispatcher) Add(n Notifier) { d.notifiers = append(d.notifiers, n) }

// Len returns the number of registered notifiers.
func (d *Dispatcher) Len() int { return len(d.notifiers) }

// Dispatch sends ev to every notifier, logging failures.
func (d *Dispatcher) Dispatch(ctx context.Context, ev Event) {
	for _, n := range d.notifiers {
		if err := n.Notify(ctx, ev); err != nil {
			d.log.Warn("notification failed", "kind", n.Kind(), "error", err)
		}
	}
}

// Config configures which notifiers a dispatcher should contain.
type Config struct {
	WebhookURL   string
	NtfyServer   string
	NtfyTopic    string
	GotifyServer string
	GotifyToken  string
}

// NewFromConfig builds a dispatcher from configuration.
func NewFromConfig(cfg Config, log *slog.Logger) *Dispatcher {
	d := NewDispatcher(log)
	if cfg.WebhookURL != "" {
		d.Add(NewWebhookNotifier(cfg.WebhookURL))
	}
	if cfg.NtfyServer != "" && cfg.NtfyTopic != "" {
		d.Add(NewNtfyNotifier(cfg.NtfyServer, cfg.NtfyTopic))
	}
	if cfg.GotifyServer != "" && cfg.GotifyToken != "" {
		d.Add(NewGotifyNotifier(cfg.GotifyServer, cfg.GotifyToken))
	}
	return d
}

// FeedNotifications is the per-feed notification configuration (stored as JSON on
// the feed). Triggers names the rule types whose matches fire notifications.
type FeedNotifications struct {
	WebhookURL   string   `json:"webhookUrl,omitempty"`
	NtfyServer   string   `json:"ntfyServer,omitempty"`
	NtfyTopic    string   `json:"ntfyTopic,omitempty"`
	GotifyServer string   `json:"gotifyServer,omitempty"`
	GotifyToken  string   `json:"gotifyToken,omitempty"`
	Triggers     []string `json:"triggers,omitempty"`
}

func (f FeedNotifications) config() Config {
	return Config{
		WebhookURL:   f.WebhookURL,
		NtfyServer:   f.NtfyServer,
		NtfyTopic:    f.NtfyTopic,
		GotifyServer: f.GotifyServer,
		GotifyToken:  f.GotifyToken,
	}
}

// HasTarget reports whether at least one delivery target is configured.
func (f FeedNotifications) HasTarget() bool {
	return f.WebhookURL != "" ||
		(f.NtfyServer != "" && f.NtfyTopic != "") ||
		(f.GotifyServer != "" && f.GotifyToken != "")
}

// Enabled reports whether notifications should fire (a target and a trigger).
func (f FeedNotifications) Enabled() bool {
	return f.HasTarget() && len(f.Triggers) > 0
}

// Triggered reports whether ruleType is configured to fire notifications.
func (f FeedNotifications) Triggered(ruleType string) bool {
	for _, t := range f.Triggers {
		if t == ruleType {
			return true
		}
	}
	return false
}

// Dispatcher builds a dispatcher for this feed's configured targets.
func (f FeedNotifications) Dispatcher(log *slog.Logger) *Dispatcher {
	return NewFromConfig(f.config(), log)
}
