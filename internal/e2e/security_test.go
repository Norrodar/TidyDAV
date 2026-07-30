package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// One user's calendars, sync jobs and previews must be invisible to another.
func TestOwnershipIsolation(t *testing.T) {
	up := newUpstream(t, calendar(vevent("o@up", "Owned", "20260615T090000Z")))
	in := newInstance(t)

	owner := in.newClient()
	owner.register("owner@example.com")
	feed := owner.createFeed(map[string]any{
		"name": "Mine", "sources": []map[string]any{{"url": up.URL}}, "rules": []map[string]any{},
	})
	var job struct {
		ID string `json:"id"`
	}
	owner.post("/api/sync", map[string]any{
		"name": "Mine", "kind": "caldav", "direction": "a-to-b",
		"aUrl": "https://a.example.com/dav/", "bUrl": "https://b.example.com/dav/",
		"intervalSeconds": 900, "enabled": false,
	}).expect(http.StatusCreated).decode(&job)

	intruder := in.newClient()
	intruder.register("intruder@example.com")

	// Every owner-scoped route must answer 404, never leak the resource.
	for _, path := range []string{
		"/api/feeds/" + feed.ID,
		"/api/feeds/" + feed.ID + "/preview",
		"/api/sync/" + job.ID,
		"/api/sync/" + job.ID + "/preview",
	} {
		intruder.get(path).expect(http.StatusNotFound)
	}
	intruder.put("/api/feeds/"+feed.ID, map[string]any{
		"name": "Hijacked", "sources": []map[string]any{}, "rules": []map[string]any{},
	}).expect(http.StatusNotFound)
	intruder.delete("/api/feeds/" + feed.ID).expect(http.StatusNotFound)

	// The intruder's own list stays empty.
	var list []feedResult
	intruder.get("/api/feeds").expect(http.StatusOK).decode(&list)
	if len(list) != 0 {
		t.Errorf("intruder sees %d calendars, want 0", len(list))
	}

	// The owner's calendar is untouched.
	owner.get("/api/feeds/" + feed.ID).expect(http.StatusOK)
}

// Signed-out visitors must not reach the API at all.
func TestAPIRequiresAuthentication(t *testing.T) {
	in := newInstance(t)
	anon := in.anonymous()

	for _, tc := range []struct{ method, path string }{
		{http.MethodGet, "/api/feeds"},
		{http.MethodPost, "/api/feeds"},
		{http.MethodPost, "/api/feeds/preview"},
		{http.MethodPost, "/api/feeds/source-check"},
		{http.MethodGet, "/api/sync"},
		{http.MethodPost, "/api/sync"},
		{http.MethodPost, "/api/sync/preview"},
		{http.MethodPost, "/api/notify/test"},
		{http.MethodGet, "/api/audit"},
	} {
		var body any
		if tc.method != http.MethodGet {
			body = map[string]any{}
		}
		if got := anon.do(tc.method, tc.path, body); got.Status != http.StatusUnauthorized {
			t.Errorf("%s %s = %d, want 401", tc.method, tc.path, got.Status)
		}
	}
}

// A cached copy fetched with credentials must never be handed to somebody who
// supplies none — otherwise knowing the URL is enough to read a private feed.
func TestCachedPrivateFeedDoesNotLeakBetweenUsers(t *testing.T) {
	secret := calendar(vevent("priv@up", "Therapy appointment", "20260615T090000Z"))
	up := newAuthUpstream(t, secret, "alice", "s3cret")
	in := newInstance(t)

	// Alice caches the private calendar via a long TTL.
	alice := in.newClient()
	alice.register("alice@example.com")
	alice.createFeed(map[string]any{
		"name":       "Private",
		"sources":    []map[string]any{{"url": up.URL, "username": "alice", "password": "s3cret"}},
		"rules":      []map[string]any{},
		"ttlSeconds": 86400,
	})

	// Mallory asks for the very same URL without credentials.
	mallory := in.newClient()
	mallory.register("mallory@example.com")
	res := mallory.post("/api/feeds/preview", map[string]any{
		"name":       "Probe",
		"sources":    []map[string]any{{"url": up.URL}},
		"rules":      []map[string]any{},
		"ttlSeconds": 86400,
	})
	if res.Status == http.StatusOK && strings.Contains(res.text(), "Therapy appointment") {
		t.Fatalf("cached private calendar leaked to another user:\n%s", res.text())
	}

	// The same must hold for the served ICS of a feed defined without secrets.
	feed := mallory.createFeed(map[string]any{
		"name": "Probe feed", "sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{}, "ttlSeconds": 86400,
	})
	ics := in.anonymous().get(icsPath(t, feed.ICSURL))
	if strings.Contains(ics.text(), "Therapy appointment") {
		t.Fatalf("cached private calendar leaked through /ics:\n%s", ics.text())
	}

	// Alice still gets her calendar.
	var list []feedResult
	alice.get("/api/feeds").expect(http.StatusOK).decode(&list)
	body := in.anonymous().get(icsPath(t, list[0].ICSURL)).expect(http.StatusOK).text()
	if !strings.Contains(body, "Therapy appointment") {
		t.Errorf("owner lost access to her own calendar:\n%s", body)
	}
}

// The audit log is admin-only; the first registered user becomes admin.
func TestAuditLogIsAdminOnly(t *testing.T) {
	in := newInstance(t)

	admin := in.newClient()
	admin.register("admin@example.com") // first user => admin
	up := newUpstream(t, calendar(vevent("a@up", "Audited", "20260615T090000Z")))
	admin.createFeed(map[string]any{
		"name": "Audited", "sources": []map[string]any{{"url": up.URL}}, "rules": []map[string]any{},
	})

	var entries []struct {
		Action string `json:"action"`
		Target string `json:"target"`
	}
	admin.get("/api/audit").expect(http.StatusOK).decode(&entries)
	var found bool
	for _, e := range entries {
		if e.Action == "feed.create" {
			found = true
		}
	}
	if !found {
		t.Errorf("feed.create was not audited: %+v", entries)
	}

	member := in.newClient()
	member.register("member@example.com")
	member.get("/api/audit").expect(http.StatusForbidden)
}

// Invalid payloads are rejected with 400 rather than stored or crashing.
func TestInputValidation(t *testing.T) {
	in := newInstance(t)
	c := in.newClient()
	c.register("validate@example.com")

	cases := []struct {
		name string
		path string
		body map[string]any
	}{
		{"calendar without name", "/api/feeds", map[string]any{
			"name": "", "sources": []map[string]any{}, "rules": []map[string]any{}}},
		{"non-http source", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{{"url": "ftp://example.com/f.ics"}}}},
		{"filter without pattern", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{},
			"rules": []map[string]any{{"type": "filter", "filterMode": "blacklist", "matchMode": "substring"}}}},
		{"strip without fields", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{},
			"rules": []map[string]any{{"type": "strip"}}}},
		{"unknown rule type", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{},
			"rules": []map[string]any{{"type": "teleport"}}}},
		{"expire without days", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{},
			"rules": []map[string]any{{"type": "expire", "days": 0}}}},
		{"unknown timezone", "/api/feeds", map[string]any{
			"name": "x", "sources": []map[string]any{},
			"rules": []map[string]any{{"type": "timezone", "target": "Mars/Phobos"}}}},
		{"sync with bad kind", "/api/sync", map[string]any{
			"name": "x", "kind": "imap", "direction": "a-to-b",
			"aUrl": "https://a/dav/", "bUrl": "https://b/dav/"}},
		{"sync with bad direction", "/api/sync", map[string]any{
			"name": "x", "kind": "caldav", "direction": "sideways",
			"aUrl": "https://a/dav/", "bUrl": "https://b/dav/"}},
		{"sync with bad date range", "/api/sync", map[string]any{
			"name": "x", "kind": "caldav", "direction": "a-to-b",
			"aUrl": "https://a/dav/", "bUrl": "https://b/dav/", "windowStart": "not-a-date"}},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			c.post(tc.path, tc.body).expect(http.StatusBadRequest)
		})
	}
}

// A blacklist that matches nothing in particular must not blank the calendar:
// an empty pattern matches everything and is therefore refused.
func TestEmptyFilterPatternIsRefused(t *testing.T) {
	in := newInstance(t)
	c := in.newClient()
	c.register("emptyfilter@example.com")
	c.post("/api/feeds", map[string]any{
		"name": "Blank", "sources": []map[string]any{},
		"rules": []map[string]any{
			{"type": "filter", "filterMode": "blacklist", "matchMode": "substring", "pattern": "  "},
		},
	}).expect(http.StatusBadRequest)
}

// Secrets the API accepts must never be echoed back.
func TestSecretsAreWriteOnly(t *testing.T) {
	up := newUpstream(t, calendar(vevent("s@up", "Secret", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("secrets@example.com")

	res := c.post("/api/feeds", map[string]any{
		"name":              "With secrets",
		"sources":           []map[string]any{{"url": up.URL, "username": "u", "password": "source-password"}},
		"rules":             []map[string]any{},
		"basicAuthUser":     "cal",
		"basicAuthPassword": "link-password",
		"notifications":     map[string]any{"gotifyServer": "https://gotify.example.com", "gotifyToken": "gotify-token"},
	}).expect(http.StatusCreated)

	for _, secret := range []string{"source-password", "link-password", "gotify-token"} {
		if strings.Contains(res.text(), secret) {
			t.Errorf("response echoed the secret %q:\n%s", secret, res.text())
		}
	}

	var created feedResult
	res.decode(&created)
	if !created.Sources[0].HasPassword || !created.Notifications.GotifyTokenSet {
		t.Errorf("stored secrets are not reported as set: %+v", created)
	}

	// Re-reading must not expose them either.
	again := c.get("/api/feeds/" + created.ID).expect(http.StatusOK)
	for _, secret := range []string{"source-password", "link-password", "gotify-token"} {
		if strings.Contains(again.text(), secret) {
			t.Errorf("GET echoed the secret %q", secret)
		}
	}
}

// Updating without re-sending write-only secrets keeps them; asking to clear
// them removes them.
func TestSecretLifecycle(t *testing.T) {
	up := newAuthUpstream(t, calendar(vevent("s@up", "Guarded", "20260615T090000Z")), "u", "p")
	in := newInstance(t)
	c := in.newClient()
	c.register("lifecycle@example.com")

	feed := c.createFeed(map[string]any{
		"name":          "Guarded",
		"sources":       []map[string]any{{"url": up.URL, "username": "u", "password": "p"}},
		"rules":         []map[string]any{},
		"notifications": map[string]any{"gotifyServer": "https://gotify.example.com", "gotifyToken": "tok"},
	})

	// An update that omits the secrets preserves them.
	var kept feedResult
	c.put("/api/feeds/"+feed.ID, map[string]any{
		"name":          "Guarded renamed",
		"sources":       []map[string]any{{"url": up.URL, "username": "u"}},
		"rules":         []map[string]any{},
		"notifications": map[string]any{"gotifyServer": "https://gotify.example.com"},
	}).expect(http.StatusOK).decode(&kept)
	if !kept.Sources[0].HasPassword {
		t.Error("source password was lost by an update that omitted it")
	}
	if !kept.Notifications.GotifyTokenSet {
		t.Error("gotify token was lost by an update that omitted it")
	}

	// Editing the URL of an authenticated source keeps its password.
	other := newAuthUpstream(t, calendar(vevent("s2@up", "Moved", "20260615T090000Z")), "u", "p")
	var moved feedResult
	c.put("/api/feeds/"+feed.ID, map[string]any{
		"name":    "Moved",
		"sources": []map[string]any{{"url": other.URL, "username": "u"}},
		"rules":   []map[string]any{},
	}).expect(http.StatusOK).decode(&moved)
	if !moved.Sources[0].HasPassword {
		t.Error("changing a source URL silently dropped its password")
	}

	// Asking to clear them actually removes them.
	var cleared feedResult
	c.put("/api/feeds/"+feed.ID, map[string]any{
		"name":          "Cleared",
		"sources":       []map[string]any{{"url": other.URL, "clearPassword": true}},
		"rules":         []map[string]any{},
		"notifications": map[string]any{},
	}).expect(http.StatusOK).decode(&cleared)
	if cleared.Sources[0].HasPassword {
		t.Error("clearPassword did not remove the stored source password")
	}
	if cleared.Notifications.GotifyTokenSet {
		t.Error("dropping the gotify server did not remove its token")
	}
}
