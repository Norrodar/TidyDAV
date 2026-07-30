// Package e2e drives a fully wired TidyDAV instance over real HTTP.
//
// Unlike the per-package unit tests, these exercise the whole stack — router,
// middleware, auth, store, rule pipeline, upstream proxy — the way a browser
// and a calendar client do. They exist to catch regressions in behaviour users
// depend on, not in individual functions.
package e2e

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/cookiejar"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/app"
	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/server"
)

// instance is a running TidyDAV plus helpers to talk to it.
type instance struct {
	t      *testing.T
	server *httptest.Server
	app    *app.App
}

// newInstance boots TidyDAV on a temporary database. opts may adjust the
// configuration before start-up.
func newInstance(t *testing.T, opts ...func(*config.Config)) *instance {
	t.Helper()
	cfg := &config.Config{
		SecretKey:           "test-secret-key",
		DBPath:              filepath.Join(t.TempDir(), "e2e.db"),
		AccessMode:          config.AccessAuth,
		AllowRegistration:   true,
		AllowPrivateTargets: true, // upstreams are httptest servers on loopback
		BackgroundAnimation: true,
		SMTP:                config.SMTPConfig{Encryption: config.SMTPStartTLS},
	}
	for _, opt := range opts {
		opt(cfg)
	}

	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(context.Background(), cfg, logger, "e2e")
	if err != nil {
		t.Fatalf("app.New: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	srv, err := server.New(a)
	if err != nil {
		t.Fatalf("server.New: %v", err)
	}
	ts := httptest.NewServer(srv.Handler())
	t.Cleanup(ts.Close)

	// BaseURL is only known once the listener is up; it drives the ICS links
	// handed back by the API.
	a.Config.BaseURL = ts.URL

	return &instance{t: t, server: ts, app: a}
}

// client is an HTTP client with its own cookie jar, i.e. one browser session.
type client struct {
	t    *testing.T
	base string
	http *http.Client
}

func (in *instance) newClient() *client {
	in.t.Helper()
	jar, err := cookiejar.New(nil)
	if err != nil {
		in.t.Fatalf("cookiejar: %v", err)
	}
	return &client{t: in.t, base: in.server.URL, http: &http.Client{Jar: jar}}
}

// anonymous returns a client without a cookie jar, like a calendar app.
func (in *instance) anonymous() *client {
	return &client{t: in.t, base: in.server.URL, http: &http.Client{}}
}

type response struct {
	t      *testing.T
	Status int
	Body   []byte
	Header http.Header
}

// decode unmarshals the body into v, failing the test on malformed JSON.
func (r *response) decode(v any) {
	r.t.Helper()
	if err := json.Unmarshal(r.Body, v); err != nil {
		r.t.Fatalf("decode %q: %v", truncate(r.Body), err)
	}
}

// expect fails unless the status matches, quoting the body for diagnosis.
func (r *response) expect(status int) *response {
	r.t.Helper()
	if r.Status != status {
		r.t.Fatalf("status = %d, want %d (body: %s)", r.Status, status, truncate(r.Body))
	}
	return r
}

func (r *response) text() string { return string(r.Body) }

func truncate(b []byte) string {
	const max = 400
	if len(b) > max {
		return string(b[:max]) + "…"
	}
	return string(b)
}

func (c *client) do(method, path string, body any) *response {
	c.t.Helper()
	var reader io.Reader
	if body != nil {
		raw, err := json.Marshal(body)
		if err != nil {
			c.t.Fatalf("marshal body: %v", err)
		}
		reader = bytes.NewReader(raw)
	}
	req, err := http.NewRequest(method, c.base+path, reader)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	if body != nil {
		req.Header.Set("Content-Type", "application/json")
	}
	return c.send(req)
}

func (c *client) send(req *http.Request) *response {
	c.t.Helper()
	resp, err := c.http.Do(req)
	if err != nil {
		c.t.Fatalf("%s %s: %v", req.Method, req.URL.Path, err)
	}
	defer func() { _ = resp.Body.Close() }()
	raw, err := io.ReadAll(resp.Body)
	if err != nil {
		c.t.Fatalf("read body: %v", err)
	}
	return &response{t: c.t, Status: resp.StatusCode, Body: raw, Header: resp.Header}
}

func (c *client) get(path string) *response         { return c.do(http.MethodGet, path, nil) }
func (c *client) post(path string, b any) *response { return c.do(http.MethodPost, path, b) }
func (c *client) put(path string, b any) *response  { return c.do(http.MethodPut, path, b) }
func (c *client) delete(path string) *response      { return c.do(http.MethodDelete, path, nil) }

// register creates an account and leaves the client signed in.
func (c *client) register(email string) {
	c.t.Helper()
	c.post("/auth/register", map[string]string{
		"email": email, "password": "correct-horse-battery",
	}).expect(http.StatusCreated)
}

// getBasicAuth fetches a path with HTTP Basic Auth, as a calendar client would.
func (c *client) getBasicAuth(path, user, pass string) *response {
	c.t.Helper()
	req, err := http.NewRequest(http.MethodGet, c.base+path, nil)
	if err != nil {
		c.t.Fatalf("new request: %v", err)
	}
	req.SetBasicAuth(user, pass)
	return c.send(req)
}

// ── Test fixtures ───────────────────────────────────────────────────────────

// feedResult is the subset of the feed API response the tests assert on.
type feedResult struct {
	ID               string `json:"id"`
	Secret           string `json:"secret"`
	ICSURL           string `json:"icsUrl"`
	BasicAuthEnabled bool   `json:"basicAuthEnabled"`
	TTLSeconds       int    `json:"ttlSeconds"`
	ServeCount       int64  `json:"serveCount"`
	LastServedAt     string `json:"lastServedAt"`
	Sources          []struct {
		URL           string `json:"url"`
		HasPassword   bool   `json:"hasPassword"`
		LastFetchedAt string `json:"lastFetchedAt"`
	} `json:"sources"`
	Notifications struct {
		GotifyServer   string `json:"gotifyServer"`
		GotifyTokenSet bool   `json:"gotifyTokenSet"`
	} `json:"notifications"`
}

// createFeed posts a calendar definition and returns the created resource.
func (c *client) createFeed(body map[string]any) feedResult {
	c.t.Helper()
	var out feedResult
	c.post("/api/feeds", body).expect(http.StatusCreated).decode(&out)
	return out
}

// icsPath turns an absolute ICS URL into a server-relative path.
func icsPath(t *testing.T, icsURL string) string {
	t.Helper()
	i := strings.Index(icsURL, "/ics/")
	if i < 0 {
		t.Fatalf("unexpected ICS URL %q", icsURL)
	}
	return icsURL[i:]
}

// upstream serves a fixed ICS body and counts the requests it received.
type upstream struct {
	*httptest.Server
	hits atomic.Int32
}

func newUpstream(t *testing.T, body string) *upstream {
	t.Helper()
	up := &upstream{}
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		up.hits.Add(1)
		w.Header().Set("Content-Type", "text/calendar")
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(up.Close)
	return up
}

// newAuthUpstream serves the body only to the given credentials, else 401.
func newAuthUpstream(t *testing.T, body, user, pass string) *upstream {
	t.Helper()
	up := &upstream{}
	up.Server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		up.hits.Add(1)
		u, p, ok := r.BasicAuth()
		if !ok || u != user || p != pass {
			w.WriteHeader(http.StatusUnauthorized)
			return
		}
		_, _ = io.WriteString(w, body)
	}))
	t.Cleanup(up.Close)
	return up
}

// calendar assembles an ICS document from VEVENT blocks.
func calendar(events ...string) string {
	var b strings.Builder
	b.WriteString("BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//e2e//EN\r\n")
	for _, e := range events {
		b.WriteString(e)
	}
	b.WriteString("END:VCALENDAR\r\n")
	return b.String()
}

// vevent builds a VEVENT block; extra lines are appended verbatim.
func vevent(uid, summary, dtstart string, extra ...string) string {
	lines := []string{
		"BEGIN:VEVENT",
		"UID:" + uid,
		"DTSTAMP:20260101T000000Z",
		"SUMMARY:" + summary,
		"DTSTART:" + dtstart,
	}
	lines = append(lines, extra...)
	lines = append(lines, "END:VEVENT")
	return strings.Join(lines, "\r\n") + "\r\n"
}

// countLines reports how often a line prefix occurs in an ICS document.
func countLines(body, prefix string) int {
	return strings.Count(body, prefix)
}

func jsonBody(v any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

var _ = fmt.Sprintf // keep fmt available for ad-hoc debugging in tests
