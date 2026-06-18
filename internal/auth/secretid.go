package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"fmt"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/store"
)

// newUUIDv4 returns a random RFC 4122 version-4 UUID string.
func newUUIDv4() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", fmt.Errorf("generate uuid: %w", err)
	}
	b[6] = (b[6] & 0x0f) | 0x40 // version 4
	b[8] = (b[8] & 0x3f) | 0x80 // variant 10
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:16]), nil
}

// hashSecret returns the hex SHA-256 of a secret-id. Secret-ids are
// high-entropy UUIDs, so a fast unsalted hash is sufficient for lookup while
// avoiding storing the plaintext.
func hashSecret(secret string) string {
	sum := sha256.Sum256([]byte(secret))
	return hex.EncodeToString(sum[:])
}

// CreateSecretUser mints a new anonymous user and returns it together with the
// plaintext secret-id. The secret is shown to the visitor once; only its hash
// is stored.
func (s *Service) CreateSecretUser(ctx context.Context) (user *store.User, secret string, err error) {
	if !s.anonymousAllowed() {
		return nil, "", ErrAnonymousDisabled
	}
	secret, err = newUUIDv4()
	if err != nil {
		return nil, "", err
	}
	id, err := newUUIDv4()
	if err != nil {
		return nil, "", err
	}
	u := &store.User{
		ID:         id,
		Kind:       "secret",
		SecretHash: sql.NullString{String: hashSecret(secret), Valid: true},
	}
	if err := s.store.CreateUser(ctx, u); err != nil {
		return nil, "", err
	}
	return u, secret, nil
}

// UserBySecret resolves a plaintext secret-id to its user.
func (s *Service) UserBySecret(ctx context.Context, secret string) (*store.User, error) {
	secret = strings.TrimSpace(secret)
	if secret == "" {
		return nil, store.ErrNotFound
	}
	return s.store.UserBySecretHash(ctx, hashSecret(secret))
}
