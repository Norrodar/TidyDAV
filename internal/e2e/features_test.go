package e2e

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync/atomic"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/notifier"
)

type previewResult struct {
	Original []struct {
		Summary string `json:"summary"`
		Start   string `json:"start"`
	} `json:"original"`
	Transformed []struct {
		Summary string `json:"summary"`
	} `json:"transformed"`
}

// The preview shows the calendar before and after the rules, both for an
// unsaved draft and for a stored calendar.
func TestPreviewBeforeAndAfter(t *testing.T) {
	up := newUpstream(t, calendar(
		vevent("a@up", "Keep", "20260615T090000Z"),
		vevent("b@up", "Drop me", "20260616T090000Z"),
	))
	in := newInstance(t)
	c := in.newClient()
	c.register("preview@example.com")

	rules := []map[string]any{
		{"type": "filter", "filterMode": "blacklist", "matchMode": "substring",
			"pattern": "Drop", "fields": []string{"SUMMARY"}},
	}

	// Draft preview: nothing is stored yet.
	var draft previewResult
	c.post("/api/feeds/preview", map[string]any{
		"name": "Draft", "sources": []map[string]any{{"url": up.URL}}, "rules": rules,
	}).expect(http.StatusOK).decode(&draft)
	if len(draft.Original) != 2 || len(draft.Transformed) != 1 {
		t.Fatalf("draft preview = %d/%d, want 2 original and 1 transformed", len(draft.Original), len(draft.Transformed))
	}
	if draft.Transformed[0].Summary != "Keep" {
		t.Errorf("transformed[0] = %q, want Keep", draft.Transformed[0].Summary)
	}

	// Saved preview: same result, no request body needed.
	feed := c.createFeed(map[string]any{
		"name": "Saved", "sources": []map[string]any{{"url": up.URL}}, "rules": rules,
	})
	var saved previewResult
	c.get("/api/feeds/" + feed.ID + "/preview").expect(http.StatusOK).decode(&saved)
	if len(saved.Original) != 2 || len(saved.Transformed) != 1 {
		t.Errorf("saved preview = %d/%d, want 2/1", len(saved.Original), len(saved.Transformed))
	}
}

// Previewing a stored calendar must reuse its stored source credentials.
func TestPreviewUsesStoredCredentials(t *testing.T) {
	up := newAuthUpstream(t, calendar(vevent("p@up", "Guarded", "20260615T090000Z")), "u", "p")
	in := newInstance(t)
	c := in.newClient()
	c.register("previewauth@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Guarded",
		"sources": []map[string]any{{"url": up.URL, "username": "u", "password": "p"}},
		"rules":   []map[string]any{},
	})
	var saved previewResult
	c.get("/api/feeds/" + feed.ID + "/preview").expect(http.StatusOK).decode(&saved)
	if len(saved.Original) != 1 {
		t.Errorf("saved preview returned %d events; stored credentials were not reused", len(saved.Original))
	}

	// The editor re-previews without resending the password.
	var edited previewResult
	c.post("/api/feeds/preview", map[string]any{
		"id": feed.ID, "name": "Guarded",
		"sources": []map[string]any{{"url": up.URL, "username": "u"}},
		"rules":   []map[string]any{},
	}).expect(http.StatusOK).decode(&edited)
	if len(edited.Original) != 1 {
		t.Errorf("editor preview returned %d events; the stored password was not reused", len(edited.Original))
	}
}

type sourceCheck struct {
	OK     bool   `json:"ok"`
	Events int    `json:"events"`
	Error  string `json:"error"`
}

// The source check tells the user whether a URL can actually be used.
func TestSourceCheck(t *testing.T) {
	good := newUpstream(t, calendar(vevent("g@up", "Fine", "20260615T090000Z")))
	notCalendar := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("<html>not a calendar</html>"))
	}))
	defer notCalendar.Close()
	guarded := newAuthUpstream(t, calendar(vevent("a@up", "Guarded", "20260615T090000Z")), "u", "p")

	in := newInstance(t)
	c := in.newClient()
	c.register("check@example.com")

	var res sourceCheck
	c.post("/api/feeds/source-check", map[string]any{"url": good.URL}).expect(http.StatusOK).decode(&res)
	if !res.OK || res.Events != 1 {
		t.Errorf("valid source = %+v, want ok with 1 event", res)
	}

	c.post("/api/feeds/source-check", map[string]any{"url": notCalendar.URL}).expect(http.StatusOK).decode(&res)
	if res.OK || res.Error == "" {
		t.Errorf("non-calendar source = %+v, want a rejection with a reason", res)
	}

	c.post("/api/feeds/source-check", map[string]any{"url": "ftp://example.com/x.ics"}).expect(http.StatusOK).decode(&res)
	if res.OK {
		t.Errorf("non-http source reported as usable: %+v", res)
	}

	// Credentials are honoured: without them the same URL fails.
	c.post("/api/feeds/source-check", map[string]any{"url": guarded.URL}).expect(http.StatusOK).decode(&res)
	if res.OK {
		t.Errorf("guarded source without credentials = %+v, want a rejection", res)
	}
	c.post("/api/feeds/source-check", map[string]any{
		"url": guarded.URL, "username": "u", "password": "p",
	}).expect(http.StatusOK).decode(&res)
	if !res.OK {
		t.Errorf("guarded source with credentials = %+v, want ok", res)
	}
}

// The test button delivers a notification immediately over the chosen channel.
func TestNotificationTest(t *testing.T) {
	var received atomic.Int32
	var payload atomic.Value
	sink := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		received.Add(1)
		buf := make([]byte, 512)
		n, _ := r.Body.Read(buf)
		payload.Store(string(buf[:n]))
		w.WriteHeader(http.StatusOK)
	}))
	defer sink.Close()

	in := newInstance(t)
	c := in.newClient()
	c.register("notify@example.com")

	c.post("/api/notify/test", map[string]any{
		"channel": "webhook", "webhookUrl": sink.URL,
	}).expect(http.StatusNoContent)
	if received.Load() != 1 {
		t.Fatalf("sink received %d notifications, want 1", received.Load())
	}
	if body, _ := payload.Load().(string); !strings.Contains(body, "test") {
		t.Errorf("payload does not identify itself as a test: %s", body)
	}

	// Misconfiguration is reported rather than silently accepted.
	c.post("/api/notify/test", map[string]any{"channel": "webhook"}).expect(http.StatusBadRequest)
	c.post("/api/notify/test", map[string]any{"channel": "carrier-pigeon"}).expect(http.StatusBadRequest)
	c.post("/api/notify/test", map[string]any{
		"channel": "ntfy", "ntfyServer": "https://ntfy.example.com",
	}).expect(http.StatusBadRequest) // topic missing
}

// Serving the ICS link records who is actually subscribed to it.
func TestSubscriberStatsAndSourceHealth(t *testing.T) {
	up := newUpstream(t, calendar(vevent("s@up", "Tracked", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("stats@example.com")

	feed := c.createFeed(map[string]any{
		"name": "Tracked", "sources": []map[string]any{{"url": up.URL}}, "rules": []map[string]any{},
	})
	if feed.ServeCount != 0 || feed.LastServedAt != "" {
		t.Fatalf("a fresh calendar already has serve stats: %+v", feed)
	}

	path := icsPath(t, feed.ICSURL)
	in.anonymous().get(path).expect(http.StatusOK)
	in.anonymous().get(path).expect(http.StatusOK)

	var after feedResult
	c.get("/api/feeds/" + feed.ID).expect(http.StatusOK).decode(&after)
	if after.ServeCount != 2 {
		t.Errorf("serveCount = %d after two fetches, want 2", after.ServeCount)
	}
	if after.LastServedAt == "" {
		t.Error("lastServedAt was not recorded")
	}
	if len(after.Sources) != 1 || after.Sources[0].LastFetchedAt == "" {
		t.Errorf("source health missing lastFetchedAt: %+v", after.Sources)
	}
}

// A cached calendar is served without hitting the upstream again.
func TestCacheAvoidsRefetchAndSurvivesOutage(t *testing.T) {
	body := calendar(vevent("c@up", "Cached", "20260615T090000Z"))
	up := newUpstream(t, body)
	in := newInstance(t)
	c := in.newClient()
	c.register("cache@example.com")

	feed := c.createFeed(map[string]any{
		"name": "Cached", "sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{}, "ttlSeconds": 3600,
	})
	path := icsPath(t, feed.ICSURL)

	in.anonymous().get(path).expect(http.StatusOK)
	afterFirst := up.hits.Load()
	in.anonymous().get(path).expect(http.StatusOK)
	if up.hits.Load() != afterFirst {
		t.Errorf("upstream was re-fetched within the TTL (%d -> %d)", afterFirst, up.hits.Load())
	}

	// With the upstream gone, the last good copy is still served.
	up.Close()
	stale := in.anonymous().get(path).expect(http.StatusOK).text()
	if !strings.Contains(stale, "Cached") {
		t.Errorf("stale-on-error did not serve the cached copy:\n%s", stale)
	}
}

// Sync jobs are configurable, and their preview reports unreachable servers
// instead of hanging or pretending to succeed.
func TestSyncJobLifecycle(t *testing.T) {
	in := newInstance(t)
	c := in.newClient()
	c.register("sync@example.com")

	var job struct {
		ID           string `json:"id"`
		APasswordSet bool   `json:"aPasswordSet"`
		WindowStart  string `json:"windowStart"`
		Direction    string `json:"direction"`
		Enabled      bool   `json:"enabled"`
	}
	c.post("/api/sync", map[string]any{
		"name": "Mirror", "kind": "caldav", "direction": "a-to-b", "conflict": "newest-wins",
		"aUrl": "https://a.example.com/dav/", "aUsername": "u", "aPassword": "secret",
		"bUrl":            "https://b.example.com/dav/",
		"intervalSeconds": 900, "enabled": true,
		"windowStart": "2026-01-01", "windowEnd": "2026-12-31",
	}).expect(http.StatusCreated).decode(&job)

	if !job.APasswordSet || job.WindowStart != "2026-01-01" {
		t.Fatalf("job was not stored as configured: %+v", job)
	}

	// Update without resending the password keeps it.
	var updated struct {
		APasswordSet bool   `json:"aPasswordSet"`
		Direction    string `json:"direction"`
	}
	c.put("/api/sync/"+job.ID, map[string]any{
		"name": "Mirror", "kind": "caldav", "direction": "bidirectional", "conflict": "newest-wins",
		"aUrl": "https://a.example.com/dav/", "aUsername": "u",
		"bUrl":            "https://b.example.com/dav/",
		"intervalSeconds": 900, "enabled": false,
	}).expect(http.StatusOK).decode(&updated)
	if !updated.APasswordSet {
		t.Error("sync password was lost by an update that omitted it")
	}
	if updated.Direction != "bidirectional" {
		t.Errorf("direction = %q, want bidirectional", updated.Direction)
	}

	// The preview reaches out to the (non-existent) servers and reports failure.
	c.get("/api/sync/" + job.ID + "/preview").expect(http.StatusBadGateway)
	c.get("/api/sync/" + job.ID + "/preview?week=nonsense").expect(http.StatusBadRequest)

	c.delete("/api/sync/" + job.ID).expect(http.StatusNoContent)
	c.get("/api/sync/" + job.ID).expect(http.StatusNotFound)
}

// The SPA is served for unknown paths so client-side routing works on reload.
func TestSPAFallbackAndHealth(t *testing.T) {
	in := newInstance(t)
	anon := in.anonymous()

	var health struct {
		Status  string `json:"status"`
		Version string `json:"version"`
	}
	anon.get("/health").expect(http.StatusOK).decode(&health)
	if health.Status != "ok" || health.Version != "e2e" {
		t.Errorf("health = %+v, want ok/e2e", health)
	}

	res := anon.get("/feeds/some-id").expect(http.StatusOK)
	if ct := res.Header.Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("SPA fallback content-type = %q, want text/html", ct)
	}
}

// Session cookies gate the API and survive across requests until logout.
func TestSessionLifecycle(t *testing.T) {
	in := newInstance(t)
	c := in.newClient()

	var anon struct {
		Authenticated bool `json:"authenticated"`
	}
	c.get("/api/session").expect(http.StatusOK).decode(&anon)
	if anon.Authenticated {
		t.Fatal("a fresh client is already authenticated")
	}

	c.register("session@example.com")
	var signedIn struct {
		Authenticated bool `json:"authenticated"`
		User          struct {
			Email   string `json:"email"`
			IsAdmin bool   `json:"isAdmin"`
		} `json:"user"`
	}
	c.get("/api/session").expect(http.StatusOK).decode(&signedIn)
	if !signedIn.Authenticated || signedIn.User.Email != "session@example.com" {
		t.Fatalf("session after register = %+v", signedIn)
	}
	if !signedIn.User.IsAdmin {
		t.Error("the first registered user should be admin")
	}

	c.post("/auth/logout", nil).expect(http.StatusNoContent)
	c.get("/api/feeds").expect(http.StatusUnauthorized)

	// Signing back in restores access.
	c.post("/auth/login", map[string]string{
		"email": "session@example.com", "password": "correct-horse-battery",
	}).expect(http.StatusOK)
	c.get("/api/feeds").expect(http.StatusOK)

	// Wrong credentials are refused.
	other := in.newClient()
	other.post("/auth/login", map[string]string{
		"email": "session@example.com", "password": "wrong",
	}).expect(http.StatusUnauthorized)
}

// A source that stops delivering is announced once, and its recovery once —
// otherwise the proxy keeps serving the last good copy and the outage stays
// invisible.
func TestSourceOutageAlertAndRecovery(t *testing.T) {
	var messages atomic.Int32
	alerts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		messages.Add(1)
		w.WriteHeader(http.StatusOK)
	}))
	defer alerts.Close()

	up := newUpstream(t, calendar(vevent("o@up", "Bin day", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("outage@example.com")

	// The calendar arms the outage alert only — no rule triggers at all.
	feed := c.createFeed(map[string]any{
		"name": "Waste", "sources": []map[string]any{{"url": up.URL}}, "rules": []map[string]any{},
		"notifications": map[string]any{"webhookUrl": alerts.URL, "sourceStaleHours": 1},
	})

	n := notifier.New(in.app.Store, in.app.Feed, in.app.Log, true)
	ctx := context.Background()

	// Nothing has ever been fetched: that is an outage, announced exactly once.
	for i := 0; i < 2; i++ {
		if err := n.Run(ctx); err != nil {
			t.Fatalf("notifier run %d: %v", i+1, err)
		}
	}
	if got := messages.Load(); got != 1 {
		t.Fatalf("outage messages = %d, want 1", got)
	}

	// A calendar client fetching the ICS link refreshes the source.
	in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK)

	if err := n.Run(ctx); err != nil {
		t.Fatalf("notifier run after recovery: %v", err)
	}
	if got := messages.Load(); got != 2 {
		t.Fatalf("messages after recovery = %d, want 2 (outage + all-clear)", got)
	}
	if err := n.Run(ctx); err != nil {
		t.Fatalf("notifier run after all-clear: %v", err)
	}
	if got := messages.Load(); got != 2 {
		t.Errorf("messages after a healthy run = %d, want 2 (no repeats)", got)
	}
}
