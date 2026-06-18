package auth

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/store"
	"golang.org/x/crypto/bcrypt"
)

func hashPassword(password string) (string, error) {
	b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", fmt.Errorf("hash password: %w", err)
	}
	return string(b), nil
}

func checkPassword(hash, password string) bool {
	return bcrypt.CompareHashAndPassword([]byte(hash), []byte(password)) == nil
}

// Register creates a new email+password user. It fails if registration is
// disabled or the email is already taken.
func (s *Service) Register(ctx context.Context, email, password string) (*store.User, error) {
	if !s.RegistrationEnabled() {
		return nil, ErrRegistrationClosed
	}
	email = strings.TrimSpace(strings.ToLower(email))
	if email == "" || password == "" {
		return nil, ErrInvalidCredentials
	}

	_, err := s.store.UserByEmail(ctx, email)
	switch {
	case err == nil:
		return nil, ErrEmailTaken
	case !errors.Is(err, store.ErrNotFound):
		return nil, err
	}

	hash, err := hashPassword(password)
	if err != nil {
		return nil, err
	}
	id, err := newUUIDv4()
	if err != nil {
		return nil, err
	}
	u := &store.User{
		ID:           id,
		Kind:         "password",
		Email:        sql.NullString{String: email, Valid: true},
		PasswordHash: sql.NullString{String: hash, Valid: true},
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, err
	}
	return u, nil
}

// Authenticate verifies email+password credentials.
func (s *Service) Authenticate(ctx context.Context, email, password string) (*store.User, error) {
	email = strings.TrimSpace(strings.ToLower(email))
	u, err := s.store.UserByEmail(ctx, email)
	if errors.Is(err, store.ErrNotFound) {
		return nil, ErrInvalidCredentials
	}
	if err != nil {
		return nil, err
	}
	if !u.PasswordHash.Valid || !checkPassword(u.PasswordHash.String, password) {
		return nil, ErrInvalidCredentials
	}
	return u, nil
}
