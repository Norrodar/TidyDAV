package server_test

import (
	"encoding/json"
	"net/http"
	"strings"
	"testing"
)

func TestSyncJobsAPI(t *testing.T) {
	srv := newTestServer(t)
	cookies := register(t, srv, "s@example.com")

	if rec := do(t, srv, http.MethodGet, "/api/sync", "", nil); rec.Code != http.StatusUnauthorized {
		t.Fatalf("no-auth list = %d, want 401", rec.Code)
	}

	body := `{"name":"Cal","kind":"caldav","direction":"a-to-b","conflict":"newest-wins",` +
		`"aUrl":"https://a.example.com/cal","bUrl":"https://b.example.com/cal","aPassword":"supersecret",` +
		`"intervalSeconds":900,"enabled":true}`
	rec := do(t, srv, http.MethodPost, "/api/sync", body, cookies)
	if rec.Code != http.StatusCreated {
		t.Fatalf("create %d: %s", rec.Code, rec.Body.String())
	}
	if strings.Contains(rec.Body.String(), "supersecret") {
		t.Errorf("password leaked in response: %s", rec.Body.String())
	}
	var created struct {
		ID           string `json:"id"`
		APasswordSet bool   `json:"aPasswordSet"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &created); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if created.ID == "" || !created.APasswordSet {
		t.Fatalf("unexpected create response: %s", rec.Body.String())
	}

	if rec := do(t, srv, http.MethodGet, "/api/sync", "", cookies); rec.Code != http.StatusOK {
		t.Fatalf("list status %d", rec.Code)
	}

	// Update without resending the password preserves it.
	upd := `{"name":"Cal2","kind":"caldav","direction":"bidirectional","conflict":"source-wins",` +
		`"aUrl":"https://a.example.com/cal","bUrl":"https://b.example.com/cal","enabled":false}`
	rec = do(t, srv, http.MethodPut, "/api/sync/"+created.ID, upd, cookies)
	if rec.Code != http.StatusOK {
		t.Fatalf("update %d: %s", rec.Code, rec.Body.String())
	}
	if !strings.Contains(rec.Body.String(), `"aPasswordSet":true`) {
		t.Errorf("password not preserved on update: %s", rec.Body.String())
	}

	// Validation.
	if rec := do(t, srv, http.MethodPost, "/api/sync",
		`{"name":"x","kind":"bad","direction":"a-to-b","aUrl":"https://a.example.com","bUrl":"https://b.example.com"}`,
		cookies); rec.Code != http.StatusBadRequest {
		t.Errorf("bad-kind status %d, want 400", rec.Code)
	}

	// Ownership isolation.
	intruder := register(t, srv, "intruder2@example.com")
	if rec := do(t, srv, http.MethodGet, "/api/sync/"+created.ID, "", intruder); rec.Code != http.StatusNotFound {
		t.Errorf("intruder get %d, want 404", rec.Code)
	}

	if rec := do(t, srv, http.MethodDelete, "/api/sync/"+created.ID, "", cookies); rec.Code != http.StatusNoContent {
		t.Errorf("delete status %d", rec.Code)
	}
}
