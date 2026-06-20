package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// PasswordReset is a pending password-reset token (stored hashed).
type PasswordReset struct {
	TokenHash string
	UserID    string
	ExpiresAt time.Time
}

// CreatePasswordReset stores a reset token.
func (s *Store) CreatePasswordReset(ctx context.Context, pr *PasswordReset) error {
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO password_resets (token_hash, user_id, expires_at) VALUES (?, ?, ?)`,
		pr.TokenHash, pr.UserID, pr.ExpiresAt.UTC().Format(time.RFC3339))
	if err != nil {
		return fmt.Errorf("create password reset: %w", err)
	}
	return nil
}

// PasswordResetByTokenHash looks up a reset token by its hash, or ErrNotFound.
func (s *Store) PasswordResetByTokenHash(ctx context.Context, hash string) (*PasswordReset, error) {
	row := s.db.QueryRowContext(ctx,
		"SELECT token_hash, user_id, expires_at FROM password_resets WHERE token_hash = ?", hash)
	var (
		pr      PasswordReset
		expires string
	)
	err := row.Scan(&pr.TokenHash, &pr.UserID, &expires)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query password reset: %w", err)
	}
	pr.ExpiresAt = parseTime(expires)
	return &pr, nil
}

// DeletePasswordReset removes a reset token.
func (s *Store) DeletePasswordReset(ctx context.Context, hash string) error {
	if _, err := s.db.ExecContext(ctx, "DELETE FROM password_resets WHERE token_hash = ?", hash); err != nil {
		return fmt.Errorf("delete password reset: %w", err)
	}
	return nil
}

// DeleteExpiredPasswordResets prunes expired tokens and returns how many were removed.
func (s *Store) DeleteExpiredPasswordResets(ctx context.Context, now time.Time) (int64, error) {
	res, err := s.db.ExecContext(ctx,
		"DELETE FROM password_resets WHERE expires_at <= ?", now.UTC().Format(time.RFC3339))
	if err != nil {
		return 0, fmt.Errorf("prune password resets: %w", err)
	}
	n, err := res.RowsAffected()
	if err != nil {
		return 0, fmt.Errorf("rows affected: %w", err)
	}
	return n, nil
}

// UpdateUserPassword sets a user's password hash.
func (s *Store) UpdateUserPassword(ctx context.Context, userID, passwordHash string) error {
	res, err := s.db.ExecContext(ctx,
		"UPDATE users SET password_hash = ? WHERE id = ?", passwordHash, userID)
	if err != nil {
		return fmt.Errorf("update user password: %w", err)
	}
	return checkAffected(res)
}
