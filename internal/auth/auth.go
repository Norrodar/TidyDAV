// Package auth provides sessions, password and OIDC authentication, and the
// anonymous secret-id mechanism.
package auth

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"time"

	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/mail"
	"github.com/Norrodar/TidyDAV/internal/store"
)

// Sentinel errors returned by the service.
var (
	ErrInvalidCredentials = errors.New("auth: invalid credentials")
	ErrRegistrationClosed = errors.New("auth: registration is disabled")
	ErrEmailTaken         = errors.New("auth: email already registered")
	ErrAnonymousDisabled  = errors.New("auth: anonymous access is disabled")
	ErrOIDCNotConfigured  = errors.New("auth: oidc is not configured")
	ErrGroupNotAllowed    = errors.New("auth: user is not in an allowed group")
	ErrInvalidResetToken  = errors.New("auth: invalid or expired reset token")
)

const (
	sessionCookieName = "tidydav_session"
	sessionTTL        = 30 * 24 * time.Hour
)

// Service bundles authentication behavior over the store.
type Service struct {
	cfg    *config.Config
	store  *store.Store
	log    *slog.Logger
	mailer mail.Mailer
	oidc   *oidcProvider // nil when OIDC is not configured
}

// New creates the auth service. When OIDC is configured it performs provider
// discovery, so it requires network access and may fail at startup.
func New(ctx context.Context, cfg *config.Config, st *store.Store, log *slog.Logger) (*Service, error) {
	s := &Service{cfg: cfg, store: st, log: log, mailer: mail.NewFromConfig(cfg.SMTP)}
	if cfg.OIDC.Enabled() {
		p, err := newOIDCProvider(ctx, cfg)
		if err != nil {
			return nil, fmt.Errorf("init oidc provider: %w", err)
		}
		s.oidc = p
	}
	return s, nil
}

// OIDCEnabled reports whether OIDC login is available.
func (s *Service) OIDCEnabled() bool { return s.oidc != nil }

// RegistrationEnabled reports whether email+password self-registration is open.
func (s *Service) RegistrationEnabled() bool {
	if s.cfg.OIDC.Only {
		return false
	}
	return s.cfg.AllowRegistration && s.cfg.AccessMode != config.AccessPublic
}

// OIDCOnly reports whether only OIDC login is accepted (no passwords/registration).
func (s *Service) OIDCOnly() bool { return s.cfg.OIDC.Only }

// OIDCDisplayName returns the configured display name for the OIDC button.
func (s *Service) OIDCDisplayName() string { return s.cfg.OIDC.DisplayName }

// anonymousAllowed reports whether anonymous secret-id access is permitted.
func (s *Service) anonymousAllowed() bool {
	return s.cfg.AccessMode == config.AccessPublic || s.cfg.AccessMode == config.AccessBoth
}
