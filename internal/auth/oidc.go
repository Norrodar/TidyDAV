package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/store"
	"github.com/coreos/go-oidc/v3/oidc"
	"golang.org/x/oauth2"
)

type oidcProvider struct {
	provider *oidc.Provider
	verifier *oidc.IDTokenVerifier
	oauth    oauth2.Config
}

func newOIDCProvider(ctx context.Context, cfg *config.Config) (*oidcProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OIDC.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}
	return &oidcProvider{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.OIDC.ClientID}),
		oauth: oauth2.Config{
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.BaseURL + "/auth/oidc/callback",
			Scopes:       cfg.OIDC.Scopes,
		},
	}, nil
}

// OIDCAuthCodeURL returns the provider authorization URL to redirect to, using
// the given opaque state value (CSRF protection).
func (s *Service) OIDCAuthCodeURL(state string) (string, error) {
	if s.oidc == nil {
		return "", ErrOIDCNotConfigured
	}
	return s.oidc.oauth.AuthCodeURL(state), nil
}

// OIDCExchange exchanges an authorization code, verifies the ID token and
// returns the matching user, creating one on first login.
func (s *Service) OIDCExchange(ctx context.Context, code string) (*store.User, error) {
	if s.oidc == nil {
		return nil, ErrOIDCNotConfigured
	}
	token, err := s.oidc.oauth.Exchange(ctx, code)
	if err != nil {
		return nil, fmt.Errorf("oidc exchange: %w", err)
	}
	rawID, ok := token.Extra("id_token").(string)
	if !ok {
		return nil, errors.New("oidc: response had no id_token")
	}
	idToken, err := s.oidc.verifier.Verify(ctx, rawID)
	if err != nil {
		return nil, fmt.Errorf("verify id token: %w", err)
	}
	var claims struct {
		Email string `json:"email"`
	}
	if err := idToken.Claims(&claims); err != nil {
		return nil, fmt.Errorf("parse id token claims: %w", err)
	}
	return s.upsertOIDCUser(ctx, idToken.Subject, claims.Email)
}

func (s *Service) upsertOIDCUser(ctx context.Context, subject, email string) (*store.User, error) {
	u, err := s.store.UserByOIDCSubject(ctx, subject)
	if err == nil {
		return u, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	id, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	u = &store.User{
		ID:          id,
		Kind:        "oidc",
		OIDCSubject: sql.NullString{String: subject, Valid: true},
	}
	if email != "" {
		u.Email = sql.NullString{String: strings.ToLower(email), Valid: true}
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}
