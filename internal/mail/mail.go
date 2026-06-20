// Package mail sends transactional email (password resets) over SMTP.
package mail

import (
	"context"
	"crypto/tls"
	"fmt"
	"net"
	"net/smtp"
	"strconv"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/config"
)

// Mailer sends a plain-text email.
type Mailer interface {
	Send(ctx context.Context, to, subject, body string) error
	Enabled() bool
}

// NewFromConfig returns an SMTP mailer when SMTP is configured, otherwise a noop
// mailer that reports Enabled() == false.
func NewFromConfig(cfg config.SMTPConfig) Mailer {
	if !cfg.Enabled() {
		return noopMailer{}
	}
	return &smtpMailer{cfg: cfg}
}

type noopMailer struct{}

func (noopMailer) Send(context.Context, string, string, string) error { return nil }
func (noopMailer) Enabled() bool                                      { return false }

type smtpMailer struct {
	cfg config.SMTPConfig
}

func (m *smtpMailer) Enabled() bool { return true }

func (m *smtpMailer) Send(ctx context.Context, to, subject, body string) error {
	addr := net.JoinHostPort(m.cfg.Host, strconv.Itoa(m.cfg.Port))
	tlsConf := &tls.Config{ServerName: m.cfg.Host}
	dialer := &net.Dialer{Timeout: 10 * time.Second}

	var client *smtp.Client
	var err error
	switch m.cfg.Encryption {
	case config.SMTPTLS:
		conn, e := tls.DialWithDialer(dialer, "tcp", addr, tlsConf)
		if e != nil {
			return fmt.Errorf("mail: tls dial: %w", e)
		}
		client, err = smtp.NewClient(conn, m.cfg.Host)
	default:
		conn, e := dialer.DialContext(ctx, "tcp", addr)
		if e != nil {
			return fmt.Errorf("mail: dial: %w", e)
		}
		client, err = smtp.NewClient(conn, m.cfg.Host)
		if err == nil && m.cfg.Encryption == config.SMTPStartTLS {
			err = client.StartTLS(tlsConf)
		}
	}
	if err != nil {
		return fmt.Errorf("mail: connect: %w", err)
	}
	defer func() { _ = client.Close() }()

	if m.cfg.Username != "" {
		if err := client.Auth(smtp.PlainAuth("", m.cfg.Username, m.cfg.Password, m.cfg.Host)); err != nil {
			return fmt.Errorf("mail: auth: %w", err)
		}
	}
	if err := client.Mail(m.cfg.From); err != nil {
		return fmt.Errorf("mail: from: %w", err)
	}
	if err := client.Rcpt(to); err != nil {
		return fmt.Errorf("mail: rcpt: %w", err)
	}
	wc, err := client.Data()
	if err != nil {
		return fmt.Errorf("mail: data: %w", err)
	}
	if _, err := wc.Write(buildMessage(m.cfg.From, to, subject, body)); err != nil {
		return fmt.Errorf("mail: write: %w", err)
	}
	if err := wc.Close(); err != nil {
		return fmt.Errorf("mail: close data: %w", err)
	}
	return client.Quit()
}

func buildMessage(from, to, subject, body string) []byte {
	var b strings.Builder
	b.WriteString("From: " + from + "\r\n")
	b.WriteString("To: " + to + "\r\n")
	b.WriteString("Subject: " + subject + "\r\n")
	b.WriteString("MIME-Version: 1.0\r\n")
	b.WriteString("Content-Type: text/plain; charset=utf-8\r\n")
	b.WriteString("\r\n")
	b.WriteString(body)
	return []byte(b.String())
}
