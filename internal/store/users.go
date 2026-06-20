package store

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"time"
)

// User is an account. Email/PasswordHash/OIDCSubject/SecretHash are populated
// depending on Kind ("password", "oidc" or "secret").
type User struct {
	ID           string
	Kind         string
	Email        sql.NullString
	PasswordHash sql.NullString
	OIDCSubject  sql.NullString
	SecretHash   sql.NullString
	IsAdmin      bool
	CreatedAt    time.Time
}

const userColumns = "id, kind, email, password_hash, oidc_subject, secret_hash, is_admin, created_at"

// CreateUser inserts a new user. CreatedAt defaults to now when zero.
func (s *Store) CreateUser(ctx context.Context, u *User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	_, err := s.db.ExecContext(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Kind, u.Email, u.PasswordHash, u.OIDCSubject, u.SecretHash,
		boolToInt(u.IsAdmin), u.CreatedAt.UTC().Format(time.RFC3339),
	)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return nil
}

// UserByID returns the user with the given id, or ErrNotFound.
func (s *Store) UserByID(ctx context.Context, id string) (*User, error) {
	return s.userBy(ctx, "id", id)
}

// UserByEmail returns the user with the given email, or ErrNotFound.
func (s *Store) UserByEmail(ctx context.Context, email string) (*User, error) {
	return s.userBy(ctx, "email", email)
}

// UserByOIDCSubject returns the user with the given OIDC subject, or ErrNotFound.
func (s *Store) UserByOIDCSubject(ctx context.Context, subject string) (*User, error) {
	return s.userBy(ctx, "oidc_subject", subject)
}

// UserBySecretHash returns the user with the given secret hash, or ErrNotFound.
func (s *Store) UserBySecretHash(ctx context.Context, secretHash string) (*User, error) {
	return s.userBy(ctx, "secret_hash", secretHash)
}

// userBy looks up a single user by an internal (non-user-supplied) column name.
func (s *Store) userBy(ctx context.Context, column, value string) (*User, error) {
	row := s.db.QueryRowContext(ctx, "SELECT "+userColumns+" FROM users WHERE "+column+" = ?", value)
	u, err := scanUser(row)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrNotFound
	}
	if err != nil {
		return nil, fmt.Errorf("query user by %s: %w", column, err)
	}
	return u, nil
}

// CreateUserBootstrapAdmin inserts a user, atomically marking it admin only when
// it is the very first user. The count and insert run in one transaction so two
// concurrent first registrations cannot both become admin. On success u.IsAdmin
// reflects what was stored.
func (s *Store) CreateUserBootstrapAdmin(ctx context.Context, u *User) error {
	if u.CreatedAt.IsZero() {
		u.CreatedAt = time.Now().UTC()
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var count int
	if err := tx.QueryRowContext(ctx, "SELECT COUNT(1) FROM users").Scan(&count); err != nil {
		return fmt.Errorf("count users: %w", err)
	}
	u.IsAdmin = count == 0

	if _, err := tx.ExecContext(ctx,
		`INSERT INTO users (`+userColumns+`) VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		u.ID, u.Kind, u.Email, u.PasswordHash, u.OIDCSubject, u.SecretHash,
		boolToInt(u.IsAdmin), u.CreatedAt.UTC().Format(time.RFC3339),
	); err != nil {
		return fmt.Errorf("create user: %w", err)
	}
	return tx.Commit()
}

func scanUser(sc rowScanner) (*User, error) {
	var (
		u         User
		isAdmin   int64
		createdAt string
	)
	if err := sc.Scan(&u.ID, &u.Kind, &u.Email, &u.PasswordHash, &u.OIDCSubject,
		&u.SecretHash, &isAdmin, &createdAt); err != nil {
		return nil, err
	}
	u.IsAdmin = isAdmin != 0
	u.CreatedAt = parseTime(createdAt)
	return &u, nil
}
