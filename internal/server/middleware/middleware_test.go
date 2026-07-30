package middleware

import (
	"bytes"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

func okHandler() http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	})
}

// Chain must run the first-listed middleware outermost.
func TestChainOrder(t *testing.T) {
	var order []string
	mark := func(name string) Middleware {
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				order = append(order, "in:"+name)
				next.ServeHTTP(w, r)
				order = append(order, "out:"+name)
			})
		}
	}
	h := Chain(okHandler(), mark("first"), mark("second"))
	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/", nil))

	want := "in:first,in:second,out:second,out:first"
	if got := strings.Join(order, ","); got != want {
		t.Errorf("order = %s, want %s", got, want)
	}
}

func TestRequestIDGeneratedAndEchoed(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if seen == "" {
		t.Fatal("no request id was placed in the context")
	}
	if got := rec.Header().Get("X-Request-ID"); got != seen {
		t.Errorf("echoed id = %q, want the context id %q", got, seen)
	}
}

func TestRequestIDReusesIncomingHeader(t *testing.T) {
	var seen string
	h := RequestID()(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		seen = RequestIDFromContext(r.Context())
	}))

	req := httptest.NewRequest(http.MethodGet, "/", nil)
	req.Header.Set("X-Request-ID", "trace-me")
	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, req)

	if seen != "trace-me" {
		t.Errorf("context id = %q, want the incoming trace-me", seen)
	}
	if got := rec.Header().Get("X-Request-ID"); got != "trace-me" {
		t.Errorf("echoed id = %q, want trace-me", got)
	}
}

func TestRequestIDFromContextWithoutMiddleware(t *testing.T) {
	if got := RequestIDFromContext(httptest.NewRequest(http.MethodGet, "/", nil).Context()); got != "" {
		t.Errorf("id = %q, want empty when the middleware did not run", got)
	}
}

// A panic must become a 500 with a JSON body, not a dropped connection.
func TestRecoverTurnsPanicIntoError(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	h := Recover(log)(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		panic("boom")
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/explode", nil))

	if rec.Code != http.StatusInternalServerError {
		t.Fatalf("status = %d, want 500", rec.Code)
	}
	if ct := rec.Header().Get("Content-Type"); ct != "application/json" {
		t.Errorf("content-type = %q, want application/json", ct)
	}
	if !strings.Contains(rec.Body.String(), "internal server error") {
		t.Errorf("body = %q, want a generic error", rec.Body.String())
	}
	if !strings.Contains(logs.String(), "panic recovered") {
		t.Errorf("the panic was not logged: %s", logs.String())
	}
	// The panic value must not reach the client.
	if strings.Contains(rec.Body.String(), "boom") {
		t.Error("the panic value leaked into the response")
	}
}

func TestRecoverPassesThroughNormalResponses(t *testing.T) {
	log := slog.New(slog.NewTextHandler(&bytes.Buffer{}, nil))
	h := Recover(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusTeapot)
		_, _ = w.Write([]byte("fine"))
	}))

	rec := httptest.NewRecorder()
	h.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/", nil))
	if rec.Code != http.StatusTeapot || rec.Body.String() != "fine" {
		t.Errorf("response = %d %q, want 418 fine", rec.Code, rec.Body.String())
	}
}

// The ICS secret is a feed's only credential and must never be logged, even
// though calendar clients request it every few minutes.
func TestLoggerRedactsICSSecret(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	h := Logger(log)(okHandler())

	h.ServeHTTP(httptest.NewRecorder(),
		httptest.NewRequest(http.MethodGet, "/ics/super-secret-token", nil))

	out := logs.String()
	if strings.Contains(out, "super-secret-token") {
		t.Errorf("the feed secret was logged: %s", out)
	}
	if !strings.Contains(out, "/ics/[redacted]") {
		t.Errorf("expected a redacted path, got: %s", out)
	}
}

func TestLoggerKeepsOrdinaryPaths(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	h := Logger(log)(okHandler())

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/api/feeds", nil))
	if !strings.Contains(logs.String(), "/api/feeds") {
		t.Errorf("ordinary path was not logged: %s", logs.String())
	}
}

func TestLoggerRecordsStatus(t *testing.T) {
	var logs bytes.Buffer
	log := slog.New(slog.NewTextHandler(&logs, nil))
	h := Logger(log)(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNotFound)
	}))

	h.ServeHTTP(httptest.NewRecorder(), httptest.NewRequest(http.MethodGet, "/missing", nil))
	if !strings.Contains(logs.String(), "status=404") {
		t.Errorf("status was not logged: %s", logs.String())
	}
}

func TestRedactPath(t *testing.T) {
	tests := map[string]string{
		"/ics/abc123":      "/ics/[redacted]",
		"/ics/":            "/ics/[redacted]",
		"/api/feeds":       "/api/feeds",
		"/":                "/",
		"/icsx/not-a-feed": "/icsx/not-a-feed",
		"/auth/oidc/login": "/auth/oidc/login",
	}
	for in, want := range tests {
		if got := redactPath(in); got != want {
			t.Errorf("redactPath(%q) = %q, want %q", in, got, want)
		}
	}
}
