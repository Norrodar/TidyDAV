package auth

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/store"
)

func newSessionToken() (string, error) {
	var b [32]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate session token: %w", err)
	}
	return hex.EncodeToString(b[:]), nil
}

// StartSession creates a session for the user and writes the session cookie.
func (s *Service) StartSession(ctx context.Context, w http.ResponseWriter, user *store.User) error {
	token, err := newSessionToken()
	if err != nil {
		return err
	}
	now := time.Now().UTC()
	sess := &store.Session{
		ID:        token,
		UserID:    user.ID,
		CreatedAt: now,
		ExpiresAt: now.Add(sessionTTL),
	}
	if err := s.store.CreateSession(ctx, sess); err != nil {
		return err
	}
	http.SetCookie(w, s.sessionCookie(token, sess.ExpiresAt))
	return nil
}

// UserFromRequest returns the user for the request's session cookie, or
// store.ErrNotFound when there is no valid, unexpired session.
func (s *Service) UserFromRequest(ctx context.Context, r *http.Request) (*store.User, error) {
	c, err := r.Cookie(sessionCookieName)
	if err != nil {
		return nil, store.ErrNotFound
	}
	sess, err := s.store.SessionByID(ctx, c.Value)
	if err != nil {
		return nil, err
	}
	if time.Now().After(sess.ExpiresAt) {
		_ = s.store.DeleteSession(ctx, sess.ID)
		return nil, store.ErrNotFound
	}
	return s.store.UserByID(ctx, sess.UserID)
}

// Logout deletes the current session and clears the cookie.
func (s *Service) Logout(ctx context.Context, w http.ResponseWriter, r *http.Request) error {
	if c, err := r.Cookie(sessionCookieName); err == nil {
		if err := s.store.DeleteSession(ctx, c.Value); err != nil {
			return err
		}
	}
	http.SetCookie(w, s.sessionCookie("", time.Unix(0, 0)))
	return nil
}

func (s *Service) sessionCookie(value string, expires time.Time) *http.Cookie {
	c := &http.Cookie{
		Name:     sessionCookieName,
		Value:    value,
		Path:     "/",
		HttpOnly: true,
		Secure:   strings.HasPrefix(s.cfg.BaseURL, "https://"),
		SameSite: http.SameSiteLaxMode,
		Expires:  expires,
	}
	if value == "" {
		c.MaxAge = -1
	}
	return c
}
