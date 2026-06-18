// Package server wires HTTP routing and middleware around the app state.
package server

import (
	"net/http"

	"github.com/Norrodar/TidyDAV/internal/app"
	"github.com/Norrodar/TidyDAV/internal/server/middleware"
)

// Server holds the fully assembled HTTP handler.
type Server struct {
	app     *app.App
	handler http.Handler
}

// New builds the server: routes plus the middleware chain.
func New(a *app.App) (*Server, error) {
	s := &Server{app: a}

	mux := http.NewServeMux()
	if err := s.routes(mux); err != nil {
		return nil, err
	}

	s.handler = middleware.Chain(mux,
		middleware.RequestID(),
		middleware.Logger(a.Log),
		middleware.Recover(a.Log),
	)
	return s, nil
}

// Handler returns the root HTTP handler.
func (s *Server) Handler() http.Handler { return s.handler }
