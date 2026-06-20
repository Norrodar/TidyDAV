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
	// SyncTick is how often the DAV sync runner checks for due jobs.
	SyncTick time.Duration

	// AccentColor is an optional hex color (e.g. "#ff6b35") that replaces the
	// default --accent CSS variable in the UI. Empty means use the default.
	AccentColor string

	OIDC OIDCConfig
	SMTP SMTPConfig
}

// OIDCConfig holds optional OpenID Connect settings.
type OIDCConfig struct {
	IssuerURL    string
	ClientID     string
	ClientSecret string
	Scopes       []string

	// DisplayName is shown in the "Sign in with <DisplayName>" button.
	// Defaults to "SSO".
	DisplayName string

	// AdminGroups lists group names (from the groups claim) that grant admin.
	// Comma-separated. Empty means no group-based admin assignment.
	AdminGroups []string

	// UserGroups lists group names required for any access. When non-empty, a
	// user not in AdminGroups AND not in UserGroups is rejected at login.
	// Comma-separated.
	UserGroups []string

	// GroupClaim is the ID-token claim name that carries the user's groups.
	// Defaults to "groups".
	GroupClaim string

	// Only disables password/email login entirely; only OIDC is allowed.
	// Requires OIDC to be configured (IssuerURL + ClientID).
	Only bool
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
		SyncTick:            envDuration("TIDYDAV_SYNC_TICK", time.Minute),
		AccentColor:         envHexColor("TIDYDAV_ACCENT_COLOR"),
		OIDC: OIDCConfig{
			IssuerURL:    strings.TrimRight(os.Getenv("TIDYDAV_OIDC_ISSUER_URL"), "/"),
			ClientID:     os.Getenv("TIDYDAV_OIDC_CLIENT_ID"),
			ClientSecret: os.Getenv("TIDYDAV_OIDC_CLIENT_SECRET"),
			Scopes:       envFields("TIDYDAV_OIDC_SCOPES", []string{"openid", "profile", "email"}),
			DisplayName:  envDefault("TIDYDAV_OIDC_DISPLAY_NAME", "SSO"),
			AdminGroups:  envComma("TIDYDAV_OIDC_ADMIN_GROUPS"),
			UserGroups:   envComma("TIDYDAV_OIDC_USER_GROUPS"),
			GroupClaim:   envDefault("TIDYDAV_OIDC_GROUP_CLAIM", "groups"),
			Only:         envBool("TIDYDAV_OIDC_ONLY", false),
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

	if c.OIDC.Only && !c.OIDC.Enabled() {
		return errors.New("TIDYDAV_OIDC_ONLY requires TIDYDAV_OIDC_ISSUER_URL and TIDYDAV_OIDC_CLIENT_ID")
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

// envComma splits a comma-separated env var, trimming whitespace from each item.
// Returns nil when the variable is unset or empty.
func envComma(key string) []string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return nil
	}
	parts := strings.Split(v, ",")
	result := make([]string, 0, len(parts))
	for _, p := range parts {
		if s := strings.TrimSpace(p); s != "" {
			result = append(result, s)
		}
	}
	return result
}

// envHexColor reads a hex color like "#0a84ff" or "0a84ff". Returns "" for
// invalid/missing values so callers can use it as "no override".
func envHexColor(key string) string {
	v := strings.TrimSpace(os.Getenv(key))
	if v == "" {
		return ""
	}
	if !strings.HasPrefix(v, "#") {
		v = "#" + v
	}
	if len(v) != 4 && len(v) != 7 {
		return ""
	}
	return strings.ToLower(v)
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
