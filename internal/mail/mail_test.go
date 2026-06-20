package mail

import (
	"context"
	"strings"
	"testing"

	"github.com/Norrodar/TidyDAV/internal/config"
)

func TestNewFromConfig(t *testing.T) {
	disabled := NewFromConfig(config.SMTPConfig{})
	if disabled.Enabled() {
		t.Error("empty config should give a disabled mailer")
	}
	if err := disabled.Send(context.Background(), "a@b", "s", "body"); err != nil {
		t.Errorf("noop Send error: %v", err)
	}
	if m := NewFromConfig(config.SMTPConfig{Host: "smtp.example.com", Port: 587}); !m.Enabled() {
		t.Error("configured SMTP should be enabled")
	}
}

func TestBuildMessage(t *testing.T) {
	msg := string(buildMessage("from@x", "to@y", "Hello", "Body here"))
	for _, want := range []string{"From: from@x", "To: to@y", "Subject: Hello", "Body here", "text/plain"} {
		if !strings.Contains(msg, want) {
			t.Errorf("message missing %q:\n%s", want, msg)
		}
	}
}
