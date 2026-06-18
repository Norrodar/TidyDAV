package server_test

import (
	"context"
	"encoding/json"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/app"
	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/server"
	"github.com/Norrodar/TidyDAV/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func newTestServer(t *testing.T) *server.Server {
	srv, _ := newTestServerWithApp(t)
	return srv
}

func newTestServerWithApp(t *testing.T) (*server.Server, *app.App) {
	t.Helper()
	cfg := &config.Config{
		SecretKey:         "k",
		BaseURL:           "https://x.example.com",
		DBPath:            filepath.Join(t.TempDir(), "srv.db"),
		AccessMode:        config.AccessAuth,
		AllowRegistration: true,
		SMTP:              config.SMTPConfig{Encryption: config.SMTPStartTLS},
	}
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))
	a, err := app.New(context.Background(), cfg, logger, "test")
	if err != nil {
		t.Fatalf("app.New() error: %v", err)
	}
	t.Cleanup(func() { _ = a.Close() })

	srv, err := server.New(a)
	if err != nil {
		t.Fatalf("server.New() error: %v", err)
	}
	return srv, a
}

func TestHealth(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/health", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body map[string]string
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body["status"] != "ok" || body["version"] != "test" {
		t.Errorf("unexpected health body: %v", body)
	}
}

func TestSessionAnonymous(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/api/session", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	var body struct {
		Authenticated bool   `json:"authenticated"`
		AccessMode    string `json:"accessMode"`
	}
	if err := json.Unmarshal(rec.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Authenticated {
		t.Error("authenticated = true, want false")
	}
	if body.AccessMode != "auth" {
		t.Errorf("accessMode = %q, want auth", body.AccessMode)
	}
}

func TestRegisterThenSession(t *testing.T) {
	srv := newTestServer(t)

	rec := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodPost, "/auth/register",
		strings.NewReader(`{"email":"a@example.com","password":"pw123456"}`))
	req.Header.Set("Content-Type", "application/json")
	srv.Handler().ServeHTTP(rec, req)

	if rec.Code != http.StatusCreated {
		t.Fatalf("register status = %d, want 201 (body %s)", rec.Code, rec.Body.String())
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("register set no session cookie")
	}

	rec2 := httptest.NewRecorder()
	req2 := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	for _, c := range cookies {
		req2.AddCookie(c)
	}
	srv.Handler().ServeHTTP(rec2, req2)

	var body struct {
		Authenticated bool `json:"authenticated"`
		User          struct {
			Email string `json:"email"`
			Kind  string `json:"kind"`
		} `json:"user"`
	}
	if err := json.Unmarshal(rec2.Body.Bytes(), &body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if !body.Authenticated {
		t.Error("authenticated = false after register")
	}
	if body.User.Email != "a@example.com" || body.User.Kind != "password" {
		t.Errorf("unexpected user: %+v", body.User)
	}
}

func TestSPAFallback(t *testing.T) {
	srv := newTestServer(t)
	rec := httptest.NewRecorder()
	// An unknown, non-API path should serve the embedded index.html.
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/some/spa/route", nil))

	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/html") {
		t.Errorf("content-type = %q, want text/html", ct)
	}
}

const icsUpstream = "BEGIN:VCALENDAR\r\nVERSION:2.0\r\nPRODID:-//up//EN\r\n" +
	"BEGIN:VEVENT\r\nUID:1@up\r\nDTSTAMP:20260101T000000Z\r\nSUMMARY:Keep\r\nDESCRIPTION:secret\r\nEND:VEVENT\r\n" +
	"END:VCALENDAR\r\n"

func TestICSEndpoint(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(icsUpstream))
	}))
	defer upstream.Close()

	srv, a := newTestServerWithApp(t)
	ctx := context.Background()
	if err := a.Store.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	if err := a.Store.CreateFeed(ctx, &store.Feed{
		ID: "f", UserID: "u", Name: "n", Secret: "topsecret", TTLSeconds: 0,
		Sources: []store.FeedSource{{URL: upstream.URL}},
		Rules:   []byte(`[{"type":"strip","fields":["DESCRIPTION"]}]`),
	}); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ics/topsecret", nil))
	if rec.Code != http.StatusOK {
		t.Fatalf("status = %d, want 200 (%s)", rec.Code, rec.Body.String())
	}
	if ct := rec.Header().Get("Content-Type"); !strings.HasPrefix(ct, "text/calendar") {
		t.Errorf("content-type = %q, want text/calendar", ct)
	}
	if body := rec.Body.String(); !strings.Contains(body, "SUMMARY:Keep") || strings.Contains(body, "secret") {
		t.Errorf("unexpected ICS body:\n%s", body)
	}

	// Unknown secret -> 404.
	rec404 := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec404, httptest.NewRequest(http.MethodGet, "/ics/nope", nil))
	if rec404.Code != http.StatusNotFound {
		t.Errorf("unknown secret status = %d, want 404", rec404.Code)
	}
}

func TestICSEndpointBasicAuth(t *testing.T) {
	upstream := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(icsUpstream))
	}))
	defer upstream.Close()

	srv, a := newTestServerWithApp(t)
	ctx := context.Background()
	if err := a.Store.CreateUser(ctx, &store.User{ID: "u", Kind: "password"}); err != nil {
		t.Fatalf("CreateUser: %v", err)
	}
	hash, _ := bcrypt.GenerateFromPassword([]byte("pw"), bcrypt.MinCost)
	if err := a.Store.CreateFeed(ctx, &store.Feed{
		ID: "f", UserID: "u", Name: "n", Secret: "protected", TTLSeconds: 0,
		Sources:       []store.FeedSource{{URL: upstream.URL}},
		BasicAuthUser: "cal",
		BasicAuthHash: string(hash),
	}); err != nil {
		t.Fatalf("CreateFeed: %v", err)
	}

	// No credentials -> 401.
	rec := httptest.NewRecorder()
	srv.Handler().ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/ics/protected", nil))
	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("without auth status = %d, want 401", rec.Code)
	}

	// Correct credentials -> 200.
	rec2 := httptest.NewRecorder()
	req := httptest.NewRequest(http.MethodGet, "/ics/protected", nil)
	req.SetBasicAuth("cal", "pw")
	srv.Handler().ServeHTTP(rec2, req)
	if rec2.Code != http.StatusOK {
		t.Fatalf("with auth status = %d, want 200 (%s)", rec2.Code, rec2.Body.String())
	}
}
