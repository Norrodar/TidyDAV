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
	mux.HandleFunc("GET /api/feeds", s.handleListFeeds)
	mux.HandleFunc("POST /api/feeds", s.handleCreateFeed)
	mux.HandleFunc("POST /api/feeds/preview", s.handlePreviewFeed)
	mux.HandleFunc("GET /api/feeds/{id}", s.handleGetFeed)
	mux.HandleFunc("PUT /api/feeds/{id}", s.handleUpdateFeed)
	mux.HandleFunc("DELETE /api/feeds/{id}", s.handleDeleteFeed)

	// Authentication.
	mux.HandleFunc("POST /auth/register", s.handleRegister)
	mux.HandleFunc("POST /auth/login", s.handleLogin)
	mux.HandleFunc("POST /auth/logout", s.handleLogout)
	mux.HandleFunc("POST /auth/secret", s.handleSecret)
	mux.HandleFunc("GET /auth/oidc/login", s.handleOIDCLogin)
	mux.HandleFunc("GET /auth/oidc/callback", s.handleOIDCCallback)

	// Transformed ICS output, secured by secret-id (no session).
	mux.HandleFunc("GET /ics/{secret}", s.handleICS)

	// Everything else: the embedded SPA (static assets + index.html fallback).
	uiHandler, err := ui.Handler()
	if err != nil {
		return fmt.Errorf("init ui handler: %w", err)
	}
	mux.Handle("/", uiHandler)
	return nil
}
