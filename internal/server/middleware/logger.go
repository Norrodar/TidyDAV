package middleware

import (
	"log/slog"
	"net/http"
	"strings"
	"time"
)

// redactPath masks the secret in /ics/<secret>. That secret is the only thing
// guarding a feed, and calendar clients poll it every few minutes — logging it
// verbatim would spread it across every log aggregator.
func redactPath(path string) string {
	const prefix = "/ics/"
	if !strings.HasPrefix(path, prefix) {
		return path
	}
	return prefix + "[redacted]"
}

type statusRecorder struct {
	http.ResponseWriter
	status int
}

func (r *statusRecorder) WriteHeader(code int) {
	r.status = code
	r.ResponseWriter.WriteHeader(code)
}

// Logger logs one structured line per request.
func Logger(log *slog.Logger) Middleware {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			rec := &statusRecorder{ResponseWriter: w, status: http.StatusOK}
			next.ServeHTTP(rec, r)
			log.Info("request",
				"method", r.Method,
				"path", redactPath(r.URL.Path),
				"status", rec.status,
				"duration_ms", time.Since(start).Milliseconds(),
				"request_id", RequestIDFromContext(r.Context()),
			)
		})
	}
}
