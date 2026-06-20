package auth

import (
	"context"
	"errors"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/config"
)

type captureMailer struct{ lastBody string }

func (m *captureMailer) Send(_ context.Context, _, _, body string) error {
	m.lastBody = body
	return nil
}
func (m *captureMailer) Enabled() bool { return true }

func tokenFromBody(t *testing.T, body string) string {
	t.Helper()
	i := strings.Index(body, "token=")
	if i < 0 {
		t.Fatalf("no token in mail body: %q", body)
	}
	rest := body[i+len("token="):]
	return strings.TrimSpace(strings.SplitN(rest, "\n", 2)[0])
}

func TestPasswordResetFlow(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	cap := &captureMailer{}
	svc.mailer = cap
	ctx := context.Background()

	if _, err := svc.Register(ctx, "r@example.com", "oldpassword"); err != nil {
		t.Fatalf("Register: %v", err)
	}
	if err := svc.RequestPasswordReset(ctx, "r@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	token := tokenFromBody(t, cap.lastBody)

	if err := svc.ConfirmPasswordReset(ctx, token, "newpassword"); err != nil {
		t.Fatalf("ConfirmPasswordReset: %v", err)
	}
	if _, err := svc.Authenticate(ctx, "r@example.com", "oldpassword"); !errors.Is(err, ErrInvalidCredentials) {
		t.Error("old password still works after reset")
	}
	if _, err := svc.Authenticate(ctx, "r@example.com", "newpassword"); err != nil {
		t.Errorf("new password failed: %v", err)
	}
	// Token cannot be reused.
	if err := svc.ConfirmPasswordReset(ctx, token, "another"); !errors.Is(err, ErrInvalidResetToken) {
		t.Error("reset token was reusable")
	}
}

func TestRequestResetUnknownEmailIsSilent(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	cap := &captureMailer{}
	svc.mailer = cap
	if err := svc.RequestPasswordReset(context.Background(), "nobody@example.com"); err != nil {
		t.Fatalf("RequestPasswordReset: %v", err)
	}
	if cap.lastBody != "" {
		t.Error("should not send mail for an unknown email")
	}
}

func TestConfirmInvalidToken(t *testing.T) {
	svc := newService(t, config.AccessAuth, true)
	if err := svc.ConfirmPasswordReset(context.Background(), "bogus", "newpassword"); !errors.Is(err, ErrInvalidResetToken) {
		t.Errorf("err = %v, want ErrInvalidResetToken", err)
	}
}
