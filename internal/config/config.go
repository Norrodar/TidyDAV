// Package config loads and validates TidyDAV configuration from TIDYDAV_* env vars.
package config

import (
	"errors"
	"fmt"
	"log/slog"
	"net/url"
	"os"
	"strconv"
	"strings"
	"time"
)

// AccessMode controls who may use the instance.
type AccessMode string

const (
	// AccessPublic gives every visitor an anonymous secret-id; no login required.
	AccessPublic AccessMode = "public"
	// AccessAuth allows only OIDC or email+password; no anonymous access.
	AccessAuth AccessMode = "auth"
	// AccessBoth allows anonymous secret-id and login simultaneously.
	AccessBoth AccessMode = "both"
)

// SMTPEncryption selects how the SMTP connection is secured.
type SMTPEncryption string

const (
	SMTPStartTLS SMTPEncryption = "starttls"
	SMTPTLS      SMTPEncryption = "tls"
	SMTPNone     SMTPEncryption = "none"
)

// Config is the fully validated runtime configuration.
type Config struct {
	SecretKey string
	BaseURL   string

	DBPath     string
	ListenAddr string
	LogLevel   slog.Level

	AccessMode        AccessMode
	AllowRegistration bool

	// AllowPrivateTargets permits the feed proxy to fetch loopback/private/
	// link-local addresses. Default true (self-hosted internal calendars); set
	// false on multi-user/public instances to mitigate SSRF.
	AllowPrivateTargets bool

	// NotifyInterval is how often the background notifier evaluates feeds.
	NotifyInterval time.Duration

	OIDC OIDCConfig
	SMTP SMTPConfig
}

// OIDCConfig holds optional OpenID Connect settings.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string
}

// Enabled reports whether OIDC is configured.
func (o OIDCConfig) Enabled() bool {
	return o.IssuerURL != "" && o.ClientID != ""
}

// SMTPConfig holds optional SMTP settings for password-reset mails.
type SMTPConfig struct {
	Host       string
	Port       int
	Username   string
	Password   string
	From       string
	Encryption SMTPEncryption
}

// Enabled reports whether SMTP is configured.
func (s SMTPConfig) Enabled() bool { return s.Host != "" }

// Load reads configuration from the environment and validates it.
func Load() (*Config, error) {
	level, err := parseLogLevel(envDefault("TIDYDAV_LOG_LEVEL", "info"))
	if err != nil {
		return nil, err
	}

	cfg := &Config{
		SecretKey:           os.Getenv("TIDYDAV_SECRET_KEY"),
		BaseURL:             strings.TrimRight(os.Getenv("TIDYDAV_BASE_URL"), "/"),
		DBPath:              envDefault("TIDYDAV_DB_PATH", "/data/tidydav.db"),
		ListenAddr:          envDefault("TIDYDAV_LISTEN_ADDR", ":8080"),
		LogLevel:            level,
		AccessMode:          AccessMode(strings.ToLower(envDefault("TIDYDAV_ACCESS_MODE", string(AccessAuth)))),
		AllowRegistration:   envBool("TIDYDAV_ALLOW_REGISTRATION", true),
		AllowPrivateTargets: envBool("TIDYDAV_ALLOW_PRIVATE_TARGETS", true),
		NotifyInterval:      envDuration("TIDYDAV_NOTIFY_INTERVAL", 15*time.Minute),
		OIDC: OIDCConfig{
			IssuerURL:    strings.TrimRight(os.Getenv("TIDYDAV_OIDC_ISSUER_URL"), "/"),
			ClientID:     os.Getenv("TIDYDAV_OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("TIDYDAV_OIDC_CLIENT_SECRET"),
			Scopes:       envFields("TIDYDAV_OIDC_SCOPES", []string{"openid", "profile", "email"}),
		},
		SMTP: SMTPConfig{
			Host:       os.Getenv("TIDYDAV_SMTP_HOST"),
			Port:       envInt("TIDYDAV_SMTP_PORT", 587),
			Username:   os.Getenv("TIDYDAV_SMTP_USERNAME"),
			Password:   os.Getenv("TIDYDAV_SMTP_PASSWORD"),
			From:       os.Getenv("TIDYDAV_SMTP_FROM"),
			Encryption: SMTPEncryption(strings.ToLower(envDefault("TIDYDAV_SMTP_ENCRYPTION", string(SMTPStartTLS)))),
		},
	}

	if err := cfg.validate(); err != nil {
		return nil, err
	}
	return cfg, nil
}

func (c *Config) validate() error {
	if c.SecretKey == "" {
		return errors.New("TIDYDAV_SECRET_KEY is required")
	}
	if c.BaseURL == "" {
		return errors.New("TIDYDAV_BASE_URL is required")
	}
	if u, err := url.Parse(c.BaseURL); err != nil || u.Scheme == "" || u.Host == "" {
		return fmt.Errorf("TIDYDAV_BASE_URL must be an absolute URL (e.g. https://dav.example.com), got %q", c.BaseURL)
	}

	switch c.AccessMode {
	case AccessPublic, AccessAuth, AccessBoth:
	default:
		return fmt.Errorf("TIDYDAV_ACCESS_MODE must be public, auth or both, got %q", c.AccessMode)
	}

	switch c.SMTP.Encryption {
	case SMTPStartTLS, SMTPTLS, SMTPNone:
	default:
		return fmt.Errorf("TIDYDAV_SMTP_ENCRYPTION must be starttls, tls or none, got %q", c.SMTP.Encryption)
	}

	return nil
}

func envDefault(key, def string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return def
}

func envBool(key string, def bool) bool {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	b, err := strconv.ParseBool(v)
	if err != nil {
		return def
	}
	return b
}

func envInt(key string, def int) int {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	n, err := strconv.Atoi(v)
	if err != nil {
		return def
	}
	return n
}

func envDuration(key string, def time.Duration) time.Duration {
	v := os.Getenv(key)
	if v == "" {
		return def
	}
	d, err := time.ParseDuration(v)
	if err != nil || d <= 0 {
		return def
	}
	return d
}

func envFields(key string, def []string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return def
	}
	return strings.Fields(v)
}

func parseLogLevel(s string) (slog.Level, error) {
	switch strings.ToLower(s) {
	case "debug":
		return slog.LevelDebug, nil
	case "info":
		return slog.LevelInfo, nil
	case "warn", "warning":
		return slog.LevelWarn, nil
	case "error":
		return slog.LevelError, nil
	default:
		return slog.LevelInfo, fmt.Errorf("TIDYDAV_LOG_LEVEL must be debug, info, warn or error, got %q", s)
	}
}
