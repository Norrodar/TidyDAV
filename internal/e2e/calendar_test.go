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
	in.anonymous().getBasicAuth(path, "cal", "s3cret").expect(http.StatusOK)
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
