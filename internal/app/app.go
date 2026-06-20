// Package app wires the configured subsystems into a single initialised state
// container that the rest of the program is handed explicitly.
package app

import (
	"context"
	"fmt"
	"log/slog"

	"github.com/Norrodar/TidyDAV/internal/audit"
	"github.com/Norrodar/TidyDAV/internal/auth"
	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/proxy"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// App is the single initialised state container (see CLAUDE.md). It is passed
// explicitly; there is no other global mutable state.
type App struct {
	Config  *config.Config
	Log     *slog.Logger
	Store   *store.Store
	Auth    *auth.Service
	Feed    *feed.Service
	Audit   *audit.Logger
	Version string
}

// New opens the store, runs migrations and initialises authentication.
func New(ctx context.Context, cfg *config.Config, log *slog.Logger, version string) (*App, error) {
	st, err := store.Open(ctx, cfg.DBPath, log)
	if err != nil {
		return nil, fmt.Errorf("open store: %w", err)
	}
	if err := st.Migrate(ctx); err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("run migrations: %w", err)
	}
	authSvc, err := auth.New(ctx, cfg, st, log)
	if err != nil {
		_ = st.Close()
		return nil, fmt.Errorf("init auth: %w", err)
	}

	feedSvc := feed.NewService(proxy.NewFetcher(st, log, cfg.AllowPrivateTargets), log)

	return &App{
		Config:  cfg,
		Log:     log,
		Store:   st,
		Auth:    authSvc,
		Feed:    feedSvc,
		Audit:   audit.New(st, log),
		Version: version,
	}, nil
}

// Close releases resources held by the app.
func (a *App) Close() error {
	return a.Store.Close()
}
