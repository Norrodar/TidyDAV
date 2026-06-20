// Command tidydav is the TidyDAV server entrypoint.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/Norrodar/TidyDAV/internal/app"
	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/notifier"
	"github.com/Norrodar/TidyDAV/internal/scheduler"
	"github.com/Norrodar/TidyDAV/internal/server"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	healthcheck := flag.Bool("healthcheck", false, "probe the local /health endpoint and exit (used by the container HEALTHCHECK)")
	flag.Parse()

	if *healthcheck {
		os.Exit(runHealthcheck())
	}

	if err := run(); err != nil {
		fmt.Fprintln(os.Stderr, "fatal:", err)
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}
	log := newLogger(cfg.LogLevel)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	a, err := app.New(ctx, cfg, log, version)
	if err != nil {
		return err
	}
	defer func() { _ = a.Close() }()

	srv, err := server.New(a)
	if err != nil {
		return err
	}

	sched := scheduler.New(log)
	sched.Add(scheduler.Job{
		Name:     "cleanup",
		Interval: time.Hour,
		Run: func(ctx context.Context) error {
			if n, err := a.Store.DeleteExpiredSessions(ctx, time.Now()); err != nil {
				return err
			} else if n > 0 {
				log.Info("purged expired sessions", "count", n)
			}
			if _, err := a.Store.DeleteExpiredPasswordResets(ctx, time.Now()); err != nil {
				return err
			}
			return nil
		},
	})
	notif := notifier.New(a.Store, a.Feed, log)
	sched.Add(scheduler.Job{Name: "notifications", Interval: cfg.NotifyInterval, Run: notif.Run})
	sched.Add(scheduler.Job{Name: "dav-sync", Interval: cfg.SyncTick, Run: a.Sync.Run})
	sched.Start(ctx)
	defer sched.Stop()

	httpServer := &http.Server{
		Addr:              cfg.ListenAddr,
		Handler:           srv.Handler(),
		ReadHeaderTimeout: 10 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		log.Info("starting server",
			"addr", cfg.ListenAddr,
			"version", version,
			"access_mode", string(cfg.AccessMode),
			"oidc", a.Auth.OIDCEnabled(),
		)
		if err := httpServer.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			errCh <- err
		}
	}()

	select {
	case <-ctx.Done():
		log.Info("shutdown signal received")
	case err := <-errCh:
		return fmt.Errorf("server error: %w", err)
	}

	shutdownCtx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
	defer cancel()
	if err := httpServer.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("graceful shutdown: %w", err)
	}
	log.Info("server stopped")
	return nil
}

func newLogger(level slog.Level) *slog.Logger {
	return slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: level}))
}

// runHealthcheck probes the local /health endpoint and returns a process exit
// code. It is invoked as `tidydav -healthcheck` by the container HEALTHCHECK.
func runHealthcheck() int {
	addr := os.Getenv("TIDYDAV_LISTEN_ADDR")
	if addr == "" {
		addr = ":8080"
	}
	_, port, err := net.SplitHostPort(addr)
	if err != nil || port == "" {
		port = "8080"
	}

	client := &http.Client{Timeout: 3 * time.Second}
	resp, err := client.Get(fmt.Sprintf("http://127.0.0.1:%s/health", port))
	if err != nil {
		fmt.Fprintln(os.Stderr, "healthcheck:", err)
		return 1
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintln(os.Stderr, "healthcheck: status", resp.StatusCode)
		return 1
	}
	return 0
}
