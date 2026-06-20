// Package middleware holds reusable net/http middleware.
package middleware

import (
	"context"
	"net/http"
)

// Middleware wraps an http.Handler with additional behavior.
type Middleware func(http.Handler) http.Handler

// Chain applies middlewares so that the first listed runs outermost (first on
// the way in, last on the way out).
func Chain(h http.Handler, middlewares ...Middleware) http.Handler {
	for i := len(middlewares) - 1; i >= 0; i-- {
		h = middlewares[i](h)
	}
	return h
}

type contextKey string

const requestIDKey contextKey = "request-id"

// RequestIDFromContext returns the request id stored in ctx, if any.
func RequestIDFromContext(ctx context.Context) string {
	id, _ := ctx.Value(requestIDKey).(string)
	return id
}
