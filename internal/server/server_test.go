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
)

func newTestServer(t *testing.T) *server.Server {
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
	return srv
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
