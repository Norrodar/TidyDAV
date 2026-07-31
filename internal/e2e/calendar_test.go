package e2e

import (
	"net/http"
	"strings"
	"testing"
)

// A calendar goes from creation to a working ICS link and back to deletion.
func TestCalendarLifecycle(t *testing.T) {
	up := newUpstream(t, calendar(
		vevent("a@up", "Team Meeting", "20260615T090000Z", "DESCRIPTION:internal"),
		vevent("b@up", "Spam Webinar", "20260616T090000Z"),
	))
	in := newInstance(t)
	c := in.newClient()
	c.register("owner@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Work",
		"sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{
			{"type": "filter", "filterMode": "blacklist", "matchMode": "substring",
				"pattern": "spam", "fields": []string{"SUMMARY"}},
			{"type": "strip", "fields": []string{"DESCRIPTION"}},
		},
	})
	if feed.ID == "" || feed.Secret == "" {
		t.Fatalf("create returned no identifiers: %+v", feed)
	}

	// The ICS link works without a session — that is how calendar apps use it.
	path := icsPath(t, feed.ICSURL)
	body := in.anonymous().get(path).expect(http.StatusOK).text()
	if !strings.Contains(body, "SUMMARY:Team Meeting") {
		t.Errorf("kept event missing:\n%s", body)
	}
	if strings.Contains(body, "Spam Webinar") {
		t.Errorf("blacklisted event still served:\n%s", body)
	}
	if strings.Contains(body, "internal") {
		t.Errorf("DESCRIPTION was not stripped:\n%s", body)
	}

	// It is listed, updatable and finally gone.
	var list []feedResult
	c.get("/api/feeds").expect(http.StatusOK).decode(&list)
	if len(list) != 1 || list[0].ID != feed.ID {
		t.Fatalf("list = %+v, want the created feed", list)
	}

	c.put("/api/feeds/"+feed.ID, map[string]any{
		"name":    "Work renamed",
		"sources": []map[string]any{{"url": up.URL}},
		"rules":   []map[string]any{},
	}).expect(http.StatusOK)

	c.delete("/api/feeds/" + feed.ID).expect(http.StatusNoContent)
	c.get("/api/feeds/" + feed.ID).expect(http.StatusNotFound)
	in.anonymous().get(path).expect(http.StatusNotFound)
}

// Rotating the link revokes the old URL and hands out a new one that serves the
// very same calendar — the escape hatch for a link shared by mistake.
func TestCalendarLinkRotation(t *testing.T) {
	up := newUpstream(t, calendar(vevent("r@up", "Bioabfall", "20260615T000000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("rotate@example.com") // first user => admin, so the audit log is readable

	feed := c.createFeed(map[string]any{
		"name": "Abfall", "sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{}, "ttlSeconds": 86400,
	})
	oldPath := icsPath(t, feed.ICSURL)
	before := in.anonymous().get(oldPath).expect(http.StatusOK).text()

	var rotated feedResult
	c.post("/api/feeds/"+feed.ID+"/rotate-secret", nil).expect(http.StatusOK).decode(&rotated)
	if rotated.Secret == feed.Secret {
		t.Fatalf("secret unchanged after rotation: %q", rotated.Secret)
	}
	newPath := icsPath(t, rotated.ICSURL)
	if newPath == oldPath {
		t.Fatalf("icsUrl still points at the old link: %q", rotated.ICSURL)
	}

	// The link that leaked is dead, the new one serves byte-identical content.
	in.anonymous().get(oldPath).expect(http.StatusNotFound)
	after := in.anonymous().get(newPath).expect(http.StatusOK).text()
	if after != before {
		t.Errorf("calendar changed across rotation:\nbefore:\n%s\nafter:\n%s", before, after)
	}

	// The rotation is audited by feed id — and the audit log must not carry the
	// secret it just replaced, nor the one it handed out.
	var entries []struct {
		Action string `json:"action"`
		Target string `json:"target"`
		Detail string `json:"detail"`
	}
	c.get("/api/audit").expect(http.StatusOK).decode(&entries)
	var found bool
	for _, e := range entries {
		if e.Action != "feed.rotate-secret" {
			continue
		}
		found = true
		if e.Target != feed.ID {
			t.Errorf("audit target = %q, want the feed id %q", e.Target, feed.ID)
		}
		if strings.Contains(e.Detail, feed.Secret) || strings.Contains(e.Detail, rotated.Secret) {
			t.Errorf("audit detail leaked a secret: %q", e.Detail)
		}
	}
	if !found {
		t.Errorf("rotation was not audited: %+v", entries)
	}
}

// Every rule type must still change the served calendar the way it claims to.
func TestRulePipelineEndToEnd(t *testing.T) {
	up := newUpstream(t, calendar(
		vevent("dup1@up", "Braune Tonne, Bioabfall", "20260615T000000Z"),
		vevent("dup2@up", "Braune Tonne, Bioabfall", "20260615T000000Z"),
		vevent("keep@up", "Schwarze Hausmülltonne", "20260616T000000Z", "LOCATION:Karlstr. 50"),
		vevent("old@up", "Ancient", "20200101T090000Z"),
		vevent("series@up", "Weekly Standup", "20200106T090000Z", "RRULE:FREQ=WEEKLY"),
	))
	in := newInstance(t)
	c := in.newClient()
	c.register("rules@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Waste",
		"sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{
			{"type": "dedup", "keyFields": []string{"SUMMARY", "DATE"}},
			{"type": "rename", "field": "SUMMARY", "matchMode": "substring",
				"pattern": "Hausmülltonne", "replacement": "Restmüll"},
			{"type": "expire", "days": 90},
			{"type": "alarm", "minutesBefore": 360, "alarmText": "Tonne rausstellen"},
		},
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()

	// dedup collapsed the identical pair, expire dropped the stale one-off …
	if got := countLines(body, "BEGIN:VEVENT"); got != 3 {
		t.Fatalf("event count = %d, want 3 (dedup + expire applied):\n%s", got, body)
	}
	if strings.Contains(body, "Ancient") {
		t.Error("expire did not drop the stale event")
	}
	// … but never a recurring series that still runs.
	if !strings.Contains(body, "Weekly Standup") {
		t.Error("expire dropped an open-ended recurring series")
	}
	// A title whose comma the source left unescaped must not be truncated —
	// either escaped form is fine, losing the tail is not.
	if !strings.Contains(body, "Braune Tonne, Bioabfall") &&
		!strings.Contains(body, "Braune Tonne\\, Bioabfall") {
		t.Errorf("title was truncated at the unescaped comma:\n%s", body)
	}
	if !strings.Contains(body, "Schwarze Restmüll") {
		t.Errorf("rename did not apply:\n%s", body)
	}
	// The reminder rule attached exactly one alarm per event.
	if got := countLines(body, "BEGIN:VALARM"); got != 3 {
		t.Errorf("VALARM count = %d, want one per event:\n%s", got, body)
	}
	if !strings.Contains(body, "TRIGGER:-PT6H") || !strings.Contains(body, "DESCRIPTION:Tonne rausstellen") {
		t.Errorf("alarm not configured as requested:\n%s", body)
	}
}

// Disabled rules are stored but must not affect the output.
func TestDisabledRuleIsSkipped(t *testing.T) {
	up := newUpstream(t, calendar(vevent("x@up", "Keep me", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("disabled@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Disabled rule",
		"sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{
			{"type": "filter", "filterMode": "blacklist", "matchMode": "substring",
				"pattern": "Keep", "enabled": false},
		},
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()
	if !strings.Contains(body, "Keep me") {
		t.Errorf("a disabled blacklist still removed the event:\n%s", body)
	}
}

// Feeds whose events carry no UID (municipal waste calendars) must still be
// servable, with identities that stay stable across fetches.
func TestFeedWithoutUIDsIsServable(t *testing.T) {
	noUID := "BEGIN:VCALENDAR\r\nVERSION:2.0\r\n" +
		"BEGIN:VEVENT\r\nSUMMARY:Bin day\r\nDTSTART;VALUE=DATE:20260702\r\nEND:VEVENT\r\n" +
		"BEGIN:VEVENT\r\nSUMMARY:Bin day\r\nDTSTART;VALUE=DATE:20260702\r\nEND:VEVENT\r\n" +
		"END:VCALENDAR\r\n"
	up := newUpstream(t, noUID)
	in := newInstance(t)
	c := in.newClient()
	c.register("nouid@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Bins",
		"sources": []map[string]any{{"url": up.URL}},
		"rules":   []map[string]any{},
	})
	path := icsPath(t, feed.ICSURL)
	first := in.anonymous().get(path).expect(http.StatusOK).text()
	if got := countLines(first, "UID:"); got != 2 {
		t.Fatalf("UID count = %d, want 2 synthesized:\n%s", got, first)
	}

	second := in.anonymous().get(path).expect(http.StatusOK).text()
	if uidLines(first) != uidLines(second) {
		t.Errorf("synthetic UIDs changed between fetches:\n%s\nvs\n%s", uidLines(first), uidLines(second))
	}
}

// A moved or cancelled occurrence shares the series UID and must survive.
func TestRecurrenceOverridesSurvive(t *testing.T) {
	up := newUpstream(t, calendar(
		vevent("s@up", "Standup", "20260105T090000Z", "RRULE:FREQ=WEEKLY"),
		vevent("s@up", "Standup (moved)", "20260112T140000Z", "RECURRENCE-ID:20260112T090000Z"),
	))
	in := newInstance(t)
	c := in.newClient()
	c.register("recurrence@example.com")

	feed := c.createFeed(map[string]any{
		"name": "Series", "sources": []map[string]any{{"url": up.URL}}, "rules": []map[string]any{},
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()
	if got := countLines(body, "BEGIN:VEVENT"); got != 2 {
		t.Fatalf("event count = %d, want master + override:\n%s", got, body)
	}
	if !strings.Contains(body, "Standup (moved)") {
		t.Error("the recurrence override was dropped by UID dedup")
	}
}

// Timezone conversion must emit the VTIMEZONE its events reference.
func TestTimezoneRuleAttachesDefinition(t *testing.T) {
	up := newUpstream(t, calendar(vevent("tz@up", "Call", "20260702T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("tz@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "TZ",
		"sources": []map[string]any{{"url": up.URL}},
		"rules":   []map[string]any{{"type": "timezone", "target": "Europe/Berlin"}},
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()
	if !strings.Contains(body, "TZID=Europe/Berlin") {
		t.Errorf("event was not converted:\n%s", body)
	}
	if !strings.Contains(body, "BEGIN:VTIMEZONE") || !strings.Contains(body, "TZID:Europe/Berlin") {
		t.Errorf("referenced TZID has no VTIMEZONE definition:\n%s", body)
	}
}

// Several sources are merged into one calendar, de-duplicated by UID.
func TestMultipleSourcesAreMerged(t *testing.T) {
	a := newUpstream(t, calendar(vevent("shared@up", "Shared", "20260615T090000Z"),
		vevent("only-a@up", "Only A", "20260616T090000Z")))
	b := newUpstream(t, calendar(vevent("shared@up", "Shared", "20260615T090000Z"),
		vevent("only-b@up", "Only B", "20260617T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("merge@example.com")

	feed := c.createFeed(map[string]any{
		"name":    "Merged",
		"sources": []map[string]any{{"url": a.URL}, {"url": b.URL}},
		"rules":   []map[string]any{},
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()
	if got := countLines(body, "BEGIN:VEVENT"); got != 3 {
		t.Fatalf("event count = %d, want 3 (shared collapsed):\n%s", got, body)
	}
	for _, want := range []string{"Shared", "Only A", "Only B"} {
		if !strings.Contains(body, want) {
			t.Errorf("merged calendar is missing %q", want)
		}
	}
}

// Basic auth turns the public link into a protected one.
func TestICSBasicAuth(t *testing.T) {
	up := newUpstream(t, calendar(vevent("p@up", "Private", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("protected@example.com")

	feed := c.createFeed(map[string]any{
		"name":              "Protected",
		"sources":           []map[string]any{{"url": up.URL}},
		"rules":             []map[string]any{},
		"basicAuthUser":     "cal",
		"basicAuthPassword": "s3cret",
	})
	if !feed.BasicAuthEnabled {
		t.Fatal("basicAuthEnabled = false after configuring credentials")
	}
	path := icsPath(t, feed.ICSURL)

	in.anonymous().get(path).expect(http.StatusUnauthorized)
	in.anonymous().getBasicAuth(path, "cal", "wrong").expect(http.StatusUnauthorized)
	ok := in.anonymous().getBasicAuth(path, "cal", "s3cret").expect(http.StatusOK)

	// Conditional GET must sit behind the auth gate: a valid ETag is no
	// substitute for credentials, and a 401 must never turn into a 304.
	etag := ok.Header.Get("ETag")
	if etag == "" {
		t.Fatal("protected calendar carried no ETag")
	}
	denied := in.anonymous().getConditional(path, etag).expect(http.StatusUnauthorized)
	if got := denied.Header.Get("ETag"); got != "" {
		t.Errorf("401 leaked an ETag: %q", got)
	}
	in.anonymous().getConditional(path, "*").expect(http.StatusUnauthorized)
	in.anonymous().getBasicAuthConditional(path, "cal", "wrong", etag).expect(http.StatusUnauthorized)
	in.anonymous().getBasicAuthConditional(path, "cal", "s3cret", etag).expect(http.StatusNotModified)
}

// A password without a username used to save silently and leave the link open.
func TestBasicAuthRequiresUsername(t *testing.T) {
	up := newUpstream(t, calendar(vevent("p@up", "Private", "20260615T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("halfauth@example.com")

	c.post("/api/feeds", map[string]any{
		"name":              "Half configured",
		"sources":           []map[string]any{{"url": up.URL}},
		"rules":             []map[string]any{},
		"basicAuthUser":     "",
		"basicAuthPassword": "s3cret",
	}).expect(http.StatusBadRequest)
}

// The served ICS must introduce itself: a calendar app subscribing to the link
// shows the configured name, and — when a cached copy exists — is told how
// often to look for new entries.
func TestServedCalendarCarriesNameAndRefreshInterval(t *testing.T) {
	up := newUpstream(t, calendar(vevent("i@up", "Bin day", "20260702T090000Z")))
	in := newInstance(t)
	c := in.newClient()
	c.register("identity@example.com")

	// A name with the three RFC 5545 separators must survive escaping.
	const name = `Waste, Rostock; "north"\east`
	feed := c.createFeed(map[string]any{
		"name":       name,
		"sources":    []map[string]any{{"url": up.URL}},
		"rules":      []map[string]any{},
		"ttlSeconds": 3600,
	})
	body := in.anonymous().get(icsPath(t, feed.ICSURL)).expect(http.StatusOK).text()

	for _, want := range []string{
		`X-WR-CALNAME:Waste\, Rostock\; "north"\\east`,
		`NAME:Waste\, Rostock\; "north"\\east`,
		"REFRESH-INTERVAL;VALUE=DURATION:PT1H",
		"X-PUBLISHED-TTL:PT1H",
	} {
		if !strings.Contains(body, want) {
			t.Errorf("served feed is missing %q:\n%s", want, body)
		}
	}

	// Without a cached copy nothing is promised about the poll rate.
	noTTL := c.createFeed(map[string]any{
		"name": "Uncached", "sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{}, "ttlSeconds": 0,
	})
	body = in.anonymous().get(icsPath(t, noTTL.ICSURL)).expect(http.StatusOK).text()
	if !strings.Contains(body, "X-WR-CALNAME:Uncached") {
		t.Errorf("name missing without TTL:\n%s", body)
	}
	if strings.Contains(body, "REFRESH-INTERVAL") || strings.Contains(body, "X-PUBLISHED-TTL") {
		t.Errorf("TTL 0 must publish no refresh interval:\n%s", body)
	}

	// A calendar whose events are all filtered away keeps its identity.
	emptied := c.createFeed(map[string]any{
		"name": "Nothing left", "sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{
			{"type": "filter", "filterMode": "whitelist", "matchMode": "substring",
				"pattern": "no-such-event", "fields": []string{"SUMMARY"}},
		},
		"ttlSeconds": 86400,
	})
	body = in.anonymous().get(icsPath(t, emptied.ICSURL)).expect(http.StatusOK).text()
	if countLines(body, "BEGIN:VEVENT") != 0 {
		t.Fatalf("expected an event-less calendar:\n%s", body)
	}
	for _, want := range []string{"X-WR-CALNAME:Nothing left", "X-PUBLISHED-TTL:P1D", "END:VCALENDAR"} {
		if !strings.Contains(body, want) {
			t.Errorf("event-less feed is missing %q:\n%s", want, body)
		}
	}
}

// uidLines extracts the UID lines of an ICS document for comparison.
func uidLines(body string) string {
	var out []string
	for _, line := range strings.Split(body, "\r\n") {
		if strings.HasPrefix(line, "UID:") {
			out = append(out, line)
		}
	}
	return strings.Join(out, "|")
}

// Calendar clients poll the ICS link every few minutes. When nothing changed
// they must get a 304 without a body — and when a rule changed, the very next
// request with the same If-None-Match must return the new calendar.
func TestConditionalGetOnICS(t *testing.T) {
	up := newUpstream(t, calendar(
		vevent("keep@up", "Bin day", "20260702T090000Z"),
		vevent("drop@up", "Spam Webinar", "20260703T090000Z"),
	))
	in := newInstance(t)
	c := in.newClient()
	c.register("polling@example.com")

	feed := c.createFeed(map[string]any{
		"name":       "Polled",
		"sources":    []map[string]any{{"url": up.URL}},
		"rules":      []map[string]any{},
		"ttlSeconds": 3600,
	})
	path := icsPath(t, feed.ICSURL)

	first := in.anonymous().get(path).expect(http.StatusOK)
	etag := first.Header.Get("ETag")
	if etag == "" {
		t.Fatal("served calendar carried no ETag")
	}
	if !strings.Contains(first.text(), "SUMMARY:Bin day") {
		t.Fatalf("unexpected calendar:\n%s", first.text())
	}
	hitsAfterFirst := up.hits.Load()

	// Unchanged: 304, no body, and no headers that only belong to a body.
	notModified := in.anonymous().getConditional(path, etag).expect(http.StatusNotModified)
	if len(notModified.Body) != 0 {
		t.Errorf("304 carried %d bytes of body", len(notModified.Body))
	}
	if ct := notModified.Header.Get("Content-Type"); ct != "" {
		t.Errorf("304 kept Content-Type %q", ct)
	}
	if cl := notModified.Header.Get("Content-Length"); cl != "" {
		t.Errorf("304 kept Content-Length %q", cl)
	}
	if got := notModified.Header.Get("ETag"); got != etag {
		t.Errorf("304 ETag = %q, want %q", got, etag)
	}
	if up.hits.Load() != hitsAfterFirst {
		t.Errorf("cached feed hit the upstream again: %d -> %d", hitsAfterFirst, up.hits.Load())
	}

	// A rule change must invalidate the tag immediately.
	c.put("/api/feeds/"+feed.ID, map[string]any{
		"name":    "Polled",
		"sources": []map[string]any{{"url": up.URL}},
		"rules": []map[string]any{
			{"type": "filter", "filterMode": "blacklist", "matchMode": "substring",
				"pattern": "bin day", "fields": []string{"SUMMARY"}},
		},
		"ttlSeconds": 3600,
	}).expect(http.StatusOK)

	changed := in.anonymous().getConditional(path, etag).expect(http.StatusOK)
	newETag := changed.Header.Get("ETag")
	if newETag == "" || newETag == etag {
		t.Fatalf("ETag did not change after the rule change: %q", newETag)
	}
	if strings.Contains(changed.text(), "SUMMARY:Bin day") {
		t.Errorf("blacklisted event still served:\n%s", changed.text())
	}
	if !strings.Contains(changed.text(), "SUMMARY:Spam Webinar") {
		t.Errorf("remaining event missing:\n%s", changed.text())
	}

	// The new tag conditions again, weakened form included.
	in.anonymous().getConditional(path, newETag).expect(http.StatusNotModified)
	in.anonymous().getConditional(path, "W/"+newETag).expect(http.StatusNotModified)

	// Every one of those five requests is a fetch, 304s included.
	var after feedResult
	c.get("/api/feeds/" + feed.ID).expect(http.StatusOK).decode(&after)
	if after.ServeCount != 5 {
		t.Errorf("serveCount = %d, want 5 (304s count as fetches)", after.ServeCount)
	}
	if after.LastServedAt == "" {
		t.Error("lastServedAt not set")
	}
}
