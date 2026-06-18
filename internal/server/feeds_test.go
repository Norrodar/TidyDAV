package server_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/server"
)

func register(t *testing.T, srv *server.Server, email string) []*http.Cookie {
	t.Helper()
	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"`+email+`","password":"pw123456"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)
	if rec.Code != http.StatusCreated {
		t.Fatalf("register status %d: %s", rec.Code, rec.Body.String())
	}
	return rec.Result().Cookies()
}

func do(t *testing.T, srv *server.Server, method, path, body string, cookies []*http.Cookie) *httptest.ResponseRecorder {
	t.Helper()
	var req *http.Request
	if body == "" {
		req = httptest.NewRequest(method, path, nil)
	} else {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	}
	for _, c := range cookies {
		req.AddCookie(c)
	}
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, req)
	return rec
}

func TestFeedsAPIRequiresAuth(t *testing.T) {
	srv := newTestServer(t)
	if rec := do(t, srv, http.MethodGet, "/api/feeds", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("status = %d, want 401", rec.Code)
	}
}

func TestFeedsAPICRUD(t *testing.T) {
	srv := newTestServer(t)
	cookies := register(t, srv, "a@example.com")

	rec := do(t, srv, http.MethodPost, "/api/feeds",
		`{"name":"Müll","sources":[{"url":"https://up.example.com/feed.ics"}],"rules":[{"type":"dedup"}],"ttlSeconds":900}`, cookies)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	var created struct {
		ID     string `json:"id"`
		ICSURL string `json:"icsUrl"`
		Secret string `json:"secret"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	if created.ID == "" || !strings.HasSuffix(created.ICSURL, "/ics/"+created.Secret) {
		t.Fatalf("unexpected create response: %s", rec.Body.String())
	}

	rec = do(t, srv, http.MethodGet, "/api/feeds", "", cookies)
	var list []map[string]any
	if err := json.Unmarshal(rec.Body.Bytes(), &list); err != nil {
		t.Fatalf("decode list: %v", err)
	}
	if len(list) != 1 {
		t.Fatalf("list len = %d, want 1", len(list))
	}

	rec = do(t, srv, http.MethodPut, "/api/feeds/"+created.ID,
		`{"name":"Renamed","sources":[],"rules":[],"ttlSeconds":600}`, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("update status %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(t, srv, http.MethodGet, "/api/feeds/"+created.ID, "", cookies)
	if !strings.Contains(rec.Body.String(), "Renamed") {
		t.Errorf("get after update missing rename: %s", rec.Body.String())
	}

	rec = do(t, srv, http.MethodDelete, "/api/feeds/"+created.ID, "", cookies)
	if rec.Code != http.StatusNoContent {
		t.Fatalf("delete status %d", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/feeds/"+created.ID, "", cookies); rec.Code != http.StatusNotFound {
		t.Errorf("get after delete status %d, want 404", rec.Code)
	}
}

func TestFeedCreateValidation(t *testing.T) {
	srv := newTestServer(t)
	cookies := register(t, srv, "b@example.com")

	if rec := do(t, srv, http.MethodPost, "/api/feeds",
		`{"name":"x","sources":[],"rules":[{"type":"bogus"}]}`, cookies); rec.Code != http.StatusBadRequest {
		t.Errorf("bad rule status = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, http.MethodPost, "/api/feeds",
		`{"name":"","sources":[]}`, cookies); rec.Code != http.StatusBadRequest {
		t.Errorf("missing name status = %d, want 400", rec.Code)
	}
	if rec := do(t, srv, http.MethodPost, "/api/feeds",
		`{"name":"x","sources":[{"url":"not-a-url"}]}`, cookies); rec.Code != http.StatusBadRequest {
		t.Errorf("bad source url status = %d, want 400", rec.Code)
	}
}

func TestFeedOwnershipIsolation(t *testing.T) {
	srv := newTestServer(t)
	owner := register(t, srv, "owner@example.com")

	rec := do(t, srv, http.MethodPost, "/api/feeds", `{"name":"mine","sources":[]}`, owner)
	var created struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}

	intruder := register(t, srv, "intruder@example.com")
	if rec := do(t, srv, http.MethodGet, "/api/feeds/"+created.ID, "", intruder); rec.Code != http.StatusNotFound {
		t.Errorf("intruder GET status = %d, want 404", rec.Code)
	}
}

func TestFeedPreview(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(icsUpstream))
	}))
	defer upstream.Close()

	srv := newTestServer(t)
	cookies := register(t, srv, "p@example.com")

	body := `{"name":"p","sources":[{"url":"` + upstream.URL + `"}],"rules":[{"type":"strip","fields":["DESCRIPTION"]}]}`
	rec := do(t, srv, http.MethodPost, "/api/feeds/preview", body, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("preview status %d: %s", rec.Code, rec.Body.String())
	}
	var prev struct {
		Original    []map[string]any `json:"original"`
		Transformed []map[string]any `json:"transformed"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &prev); err != nil {
		t.Fatalf("decode preview: %v", err)
	}
	if len(prev.Original) != 1 || len(prev.Transformed) != 1 {
		t.Fatalf("preview counts orig=%d trans=%d, want 1/1", len(prev.Original), len(prev.Transformed))
	}
	if prev.Original[0]["description"] != "secret" {
		t.Errorf("original description = %v, want secret", prev.Original[0]["description"])
	}
	if prev.Transformed[0]["description"] != "" {
		t.Errorf("transformed description = %v, want empty", prev.Transformed[0]["description"])
	}
}

func TestAuditRequiresAdmin(t *testing.T) {
	srv := newTestServer(t)
	admin := register(t, srv, "admin@example.com") // first user => admin

	if rec := do(t, srv, http.MethodGet, "/api/audit", "", admin); rec.Code != http.StatusOK {
		t.Fatalf("admin audit status %d, want 200", rec.Code)
	}
	user := register(t, srv, "user@example.com")
	if rec := do(t, srv, http.MethodGet, "/api/audit", "", user); rec.Code != http.StatusForbidden {
		t.Errorf("non-admin audit status %d, want 403", rec.Code)
	}
	if rec := do(t, srv, http.MethodGet, "/api/audit", "", nil); rec.Code != http.StatusUnauthorized {
		t.Errorf("no-auth audit status %d, want 401", rec.Code)
	}
}

func TestFeedActionsAreAudited(t *testing.T) {
	srv := newTestServer(t)
	admin := register(t, srv, "admin@example.com") // first user => admin

	rec := do(t, srv, http.MethodPost, "/api/feeds", `{"name":"Audited","sources":[]}`, admin)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create status %d: %s", rec.Code, rec.Body.String())
	}
	rec = do(t, srv, http.MethodGet, "/api/audit", "", admin)
	if !strings.Contains(rec.Body.String(), "feed.create") || !strings.Contains(rec.Body.String(), "Audited") {
		t.Errorf("audit missing create entry: %s", rec.Body.String())
	}
}
