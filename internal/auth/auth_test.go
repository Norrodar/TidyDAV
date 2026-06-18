package auth

import (
	"context"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/store"
)

func newService(t *testing.T, mode config.AccessMode, allowRegistration bool) *Service {
	t.Helper()
	ctx := context.Background()
	logger := slog.New(slog.NewTextHandler(io.Discard, nil))

	st, err := store.Open(ctx, filepath.Join(t.TempDir(), "auth.db"), logger)
	if err != nil {
		t.Fatalf("store.Open() error: %v", err)
	}
	t.Cleanup(func() { _ = st.Close() })
	if err := st.Migrate(ctx); err != nil {
		t.Fatalf("Migrate() error: %v", err)
	}

	cfg := &config.Config{
		BaseURL:           "https://dav.example.com",
		AccessMode:        mode,
		AllowRegistration: allowRegistration,
	}
	svc, err := New(ctx, cfg, st, logger)
	if err != nil {
		t.Fatalf("auth.New() error: %v", err)
	}
	return svc
}

func TestNewUUIDv4(t *testing.T) {
	id, err := newUUIDv4()
	if err != nil {
		t.Fatalf("newUUIDv4() error: %v", err)
	}
	if len(id) != 36 {
		t.Fatalf("uuid length = %d, want 36 (%q)", len(id), id)
	}
	if id[14] != '4' {
		t.Errorf("version nibble = %q, want '4' (%q)", id[14], id)
	}
	switch id[19] {
	case '8', '9', 'a', 'b':
	default:
		t.Errorf("variant nibble = %q, want one of 8,9,a,b (%q)", id[19], id)
	}

	other, _ := newUUIDv4()
	if id == other {
		t.Error("two generated UUIDs are identical")
	}
}

func TestHashSecret(t *testing.T) {
	h1 := hashSecret("abc")
	h2 := hashSecret("abc")
	h3 := hashSecret("abd")
	if h1 != h2 {
		t.Error("hashSecret is not deterministic")
	}
	if h1 == h3 {
		t.Error("different secrets produced the same hash")
	}
	if len(h1) != 64 {
		t.Errorf("hash length = %d, want 64 hex chars", len(h1))
	}
}

func TestSecretUserRoundTrip(t *testing.T) {
	svc := newService(t, config.AccessBoth, true)
	ctx := context.Background()

	user, secret, err := svc.CreateSecretUser(ctx)
	if err != nil {
		t.Fatalf("CreateSecretUser() error: %v", err)
	}
	if user.Kind != "secret" {
		t.Errorf("kind = %q, want secret", user.Kind)
	}

	got, err := svc.UserBySecret(ctx, secret)
	if err != nil {
		t.Fatalf("UserBySecret() error: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("resolved user %q, want %q", got.ID, user.ID)
	}

	if _, err := svc.UserBySecret(ctx, "wrong-secret"); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("UserBySecret(wrong) error = %v, want ErrNotFound", err)
	}
}

func TestSecretUserDisabledInAuthMode(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	if _, _, err := svc.CreateSecretUser(context.Background()); !errors.Is(err, ErrAnonymousDisabled) {
		t.Fatalf("error = %v, want ErrAnonymousDisabled", err)
	}
}

func TestRegisterAndAuthenticate(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	ctx := context.Background()

	u, err := svc.Register(ctx, "User@Example.com", "s3cret-pw")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}
	if u.Email.String != "user@example.com" {
		t.Errorf("email = %q, want lowercased", u.Email.String)
	}

	if _, err := svc.Register(ctx, "user@example.com", "other"); !errors.Is(err, ErrEmailTaken) {
		t.Errorf("duplicate Register error = %v, want ErrEmailTaken", err)
	}

	got, err := svc.Authenticate(ctx, "user@example.com", "s3cret-pw")
	if err != nil {
		t.Fatalf("Authenticate() error: %v", err)
	}
	if got.ID != u.ID {
		t.Errorf("authenticated user %q, want %q", got.ID, u.ID)
	}

	if _, err := svc.Authenticate(ctx, "user@example.com", "wrong"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("wrong password error = %v, want ErrInvalidCredentials", err)
	}
	if _, err := svc.Authenticate(ctx, "nobody@example.com", "x"); !errors.Is(err, ErrInvalidCredentials) {
		t.Errorf("unknown user error = %v, want ErrInvalidCredentials", err)
	}
}

func TestRegistrationClosed(t *testing.T) {
	svc := newService(t, config.AccessAuth, false)
	if svc.RegistrationEnabled() {
		t.Error("RegistrationEnabled() = true, want false")
	}
	if _, err := svc.Register(context.Background(), "x@example.com", "pw"); !errors.Is(err, ErrRegistrationClosed) {
		t.Fatalf("error = %v, want ErrRegistrationClosed", err)
	}
}

func TestSessionRoundTrip(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	ctx := context.Background()

	user, err := svc.Register(ctx, "s@example.com", "pw12345")
	if err != nil {
		t.Fatalf("Register() error: %v", err)
	}

	rec := httptest.NewRecorder()
	if err := svc.StartSession(ctx, rec, user); err != nil {
		t.Fatalf("StartSession() error: %v", err)
	}
	cookies := rec.Result().Cookies()
	if len(cookies) == 0 {
		t.Fatal("StartSession set no cookie")
	}

	req := httptest.NewRequest(http.MethodGet, "/api/session", nil)
	for _, c := range cookies {
		req.AddCookie(c)
	}

	got, err := svc.UserFromRequest(ctx, req)
	if err != nil {
		t.Fatalf("UserFromRequest() error: %v", err)
	}
	if got.ID != user.ID {
		t.Errorf("session user %q, want %q", got.ID, user.ID)
	}

	// A request without the cookie has no session.
	if _, err := svc.UserFromRequest(ctx, httptest.NewRequest(http.MethodGet, "/", nil)); !errors.Is(err, store.ErrNotFound) {
		t.Errorf("no-cookie error = %v, want ErrNotFound", err)
	}
}

func TestFirstUserIsAdmin(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	ctx := context.Background()

	first, err := svc.Register(ctx, "first@example.com", "pw123456")
	if err != nil {
		t.Fatalf("Register first: %v", err)
	}
	if !first.IsAdmin {
		t.Error("first user should be admin")
	}

	second, err := svc.Register(ctx, "second@example.com", "pw123456")
	if err != nil {
		t.Fatalf("Register second: %v", err)
	}
	if second.IsAdmin {
		t.Error("second user should not be admin")
	}
}
