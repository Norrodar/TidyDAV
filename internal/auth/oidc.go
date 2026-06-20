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
	provider      *oidc.Provider
	verifier      *oidc.IDTokenVerifier
	oauth         oauth2.Config
	endSessionURL string // end_session_endpoint from discovery, may be empty
}

func newOIDCProvider(ctx context.Context, cfg *config.Config) (*oidcProvider, error) {
	provider, err := oidc.NewProvider(ctx, cfg.OIDC.IssuerURL)
	if err != nil {
		return nil, fmt.Errorf("oidc discovery: %w", err)
	}

	p := &oidcProvider{
		provider: provider,
		verifier: provider.Verifier(&oidc.Config{ClientID: cfg.OIDC.ClientID}),
		oauth: oauth2.Config{
			ClientID:     cfg.OIDC.ClientID,
			ClientSecret: cfg.OIDC.ClientSecret,
			Endpoint:     provider.Endpoint(),
			RedirectURL:  cfg.BaseURL + "/auth/oidc/callback",
			Scopes:       cfg.OIDC.Scopes,
		},
	}

	// Extract end_session_endpoint if the provider advertises it.
	var providerClaims struct {
		EndSessionEndpoint string `json:"end_session_endpoint"`
	}
	if err := provider.Claims(&providerClaims); err == nil {
		p.endSessionURL = providerClaims.EndSessionEndpoint
	}

	return p, nil
}

// OIDCAuthCodeURL returns the provider authorization URL with a PKCE S256
// challenge. The caller must supply a verifier generated via
// oauth2.GenerateVerifier() and store it (e.g. in a short-lived cookie) for use
// in OIDCExchange.
func (s *Service) OIDCAuthCodeURL(state, verifier string) (string, error) {
	if s.oidc == nil {
		return "", ErrOIDCNotConfigured
	}
	return s.oidc.oauth.AuthCodeURL(state, oauth2.S256ChallengeOption(verifier)), nil
}

// OIDCEndSessionURL returns the provider's end_session_endpoint, or "" when the
// provider does not advertise one.
func (s *Service) OIDCEndSessionURL() string {
	if s.oidc == nil {
		return ""
	}
	return s.oidc.endSessionURL
}

// OIDCExchange exchanges an authorization code (with PKCE verifier), verifies the
// ID token, and returns the matching user — creating or updating one as needed.
func (s *Service) OIDCExchange(ctx context.Context, code, verifier string) (*store.User, error) {
	if s.oidc == nil {
		return nil, ErrOIDCNotConfigured
	}

	token, err := s.oidc.oauth.Exchange(ctx, code, oauth2.VerifierOption(verifier))
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

	// Parse standard + provider-specific claims.
	groupClaim := s.cfg.OIDC.GroupClaim
	if groupClaim == "" {
		groupClaim = "groups"
	}
	var claims struct {
		Email   string   `json:"email"`
		Picture string   `json:"picture"`
		Groups  []string // populated from the configured claim name below
	}
	// Use a map to extract the dynamic group claim name.
	var raw map[string]any
	if err := idToken.Claims(&raw); err != nil {
		return nil, fmt.Errorf("parse id token claims: %w", err)
	}
	if e, ok := raw["email"].(string); ok {
		claims.Email = e
	}
	if p, ok := raw["picture"].(string); ok {
		claims.Picture = p
	}
	if gv, ok := raw[groupClaim]; ok {
		switch v := gv.(type) {
		case []any:
			for _, g := range v {
				if gs, ok := g.(string); ok {
					claims.Groups = append(claims.Groups, gs)
				}
			}
		case []string:
			claims.Groups = v
		}
	}

	isAdmin, allowed := s.groupsCheck(claims.Groups)
	if !allowed {
		return nil, ErrGroupNotAllowed
	}

	return s.upsertOIDCUser(ctx, idToken.Subject, claims.Email, claims.Picture, isAdmin)
}

// groupsCheck returns (isAdmin, allowed) based on the configured group rules.
// When no group rules are configured every user is allowed and isAdmin is false
// (bootstrap logic in the store handles the very-first-user case).
func (s *Service) groupsCheck(userGroups []string) (isAdmin bool, allowed bool) {
	cfg := s.cfg.OIDC

	// Fast path: no group restrictions configured.
	if len(cfg.AdminGroups) == 0 && len(cfg.UserGroups) == 0 {
		return false, true
	}

	if hasGroupOverlap(userGroups, cfg.AdminGroups) {
		return true, true
	}
	if len(cfg.UserGroups) > 0 {
		return false, hasGroupOverlap(userGroups, cfg.UserGroups)
	}
	// AdminGroups configured but user is not in any of them, and no UserGroups
	// restriction → allow as regular user.
	return false, true
}

func hasGroupOverlap(userGroups, required []string) bool {
	for _, r := range required {
		for _, u := range userGroups {
			if r == u {
				return true
			}
		}
	}
	return false
}

func (s *Service) upsertOIDCUser(ctx context.Context, subject, email, avatarURL string, groupAdmin bool) (*store.User, error) {
	groupsConfigured := len(s.cfg.OIDC.AdminGroups) > 0 || len(s.cfg.OIDC.UserGroups) > 0

	u, err := s.store.UserByOIDCSubject(ctx, subject)
	if err == nil {
		// Existing user: sync avatar and admin status.
		newAdmin := u.IsAdmin
		if groupsConfigured {
			newAdmin = groupAdmin
		}
		if err := s.store.UpdateUserAvatarAndAdmin(ctx, u.ID, avatarURL, newAdmin); err != nil {
			return nil, err
		}
		u.AvatarURL = avatarURL
		u.IsAdmin = newAdmin
		return u, nil
	}
	if !errors.Is(err, store.ErrNotFound) {
		return nil, err
	}

	// New user.
	id, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	u = &store.User{
		ID:          id,
		Kind:        "oidc",
		OIDCSubject: sql.NullString{String: subject, Valid: true},
		AvatarURL:   avatarURL,
		IsAdmin:     groupAdmin,
	}
	if email != "" {
		u.Email = sql.NullString{String: strings.ToLower(email), Valid: true}
	}

	if groupsConfigured {
		// Groups are source of truth — use simple insert (no bootstrap).
		return u, s.store.CreateUser(ctx, u)
	}
	// No group config: bootstrap the very first user as admin.
	return u, s.store.CreateUserBootstrapAdmin(ctx, u)
}
