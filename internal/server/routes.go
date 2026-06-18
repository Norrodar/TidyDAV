package server

import (
	"fmt"
	"net/http"

	"github.com/Norrodar/TidyDAV/internal/ui"
)

func (s *Server) routes(mux *http.ServeMux) error {
	// Health probe.
	mux.HandleFunc("GET /health", s.handleHealth)

	// JSON API for the web UI (session-authenticated).
	mux.HandleFunc("GET /api/session", s.handleSession)

	// Authentication.
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/secret", s.handleSecret)
	mux.HandleFunc("GET /auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)

	// Everything else: the embedded SPA (static assets + index.html fallback).
	uiHandler, err := ui.Handler()
	if err != nil {
		return fmt.Errorf("init ui handler: %w", err)
	}
	mux.Handle("/", uiHandler)
	return nil
}
