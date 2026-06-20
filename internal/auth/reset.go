package auth

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/store"
)

const resetTTL = time.Hour

// MailEnabled reports whether password-reset email is available.
func (s *Service) MailEnabled() bool { return s.mailer.Enabled() }

// RequestPasswordReset issues a reset token and emails it. To avoid leaking which
// emails exist, it returns nil when no matching email+password account is found.
func (s *Service) RequestPasswordReset(ctx context.Context, email string) error {
	email = strings.TrimSpace(strings.ToLower(email))
	user, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil
	}
	if err != nil {
		return err
	}
	if !user.PasswordHash.Valid {
		return nil // OIDC/secret account, not resettable by email
	}

	token, err := newUUIDv4()
	if err != nil {
		return err
	}
	if err := s.store.CreatePasswordReset(ctx, &store.PasswordReset{
		TokenHash: hashSecret(token),
		UserID:    user.ID,
		ExpiresAt: time.Now().Add(resetTTL),
	}); err != nil {
		return err
	}

	link := s.cfg.BaseURL + "/reset?token=" + token
	body := fmt.Sprintf("Reset your TidyDAV password by opening this link:\n\n%s\n\n"+
		"The link expires in 1 hour. If you didn't request a reset, ignore this email.", link)
	if err := s.mailer.Send(ctx, email, "Reset your TidyDAV password", body); err != nil {
		s.log.Warn("send reset email failed", "error", err)
		return err
	}
	return nil
}

// ConfirmPasswordReset validates a token and sets a new password.
func (s *Service) ConfirmPasswordReset(ctx context.Context, token, newPassword string) error {
	if strings.TrimSpace(newPassword) == "" {
		return ErrInvalidCredentials
	}
	pr, err := s.store.PasswordResetByTokenHash(ctx, hashSecret(token))
	if errors.Is(err, store.ErrNotFound) {
		return ErrInvalidResetToken
	}
	if err != nil {
		return err
	}
	if time.Now().After(pr.ExpiresAt) {
		_ = s.store.DeletePasswordReset(ctx, pr.TokenHash)
		return ErrInvalidResetToken
	}

	hash, err := hashPassword(newPassword)
	if err != nil {
		return err
	}
	if err := s.store.UpdateUserPassword(ctx, pr.UserID, hash); err != nil {
		return err
	}
	_ = s.store.DeletePasswordReset(ctx, pr.TokenHash)
	return nil
}
