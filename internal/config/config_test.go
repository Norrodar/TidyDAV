package config

import (
	"log/slog"
	"testing"
)

// setMinimalEnv sets only the two required variables and clears the optional
// ones that would otherwise leak in from the host environment.
func setMinimalEnv(t *testing.T) {
	t.Helper()
	t.Setenv("TIDYDAV_SECRET_KEY", "test-secret")
	t.Setenv("TIDYDAV_BASE_URL", "https://dav.example.com")
	for _, k := range []string{
		"TIDYDAV_DB_PATH", "TIDYDAV_LISTEN_ADDR", "TIDYDAV_LOG_LEVEL",
		"TIDYDAV_ACCESS_MODE", "TIDYDAV_ALLOW_REGISTRATION",
		"TIDYDAV_OIDC_ISSUER_URL", "TIDYDAV_OIDC_CLIENT_ID", "TIDYDAV_OIDC_CLIENT_SECRET",
		"TIDYDAV_OIDC_SCOPES", "TIDYDAV_SMTP_HOST", "TIDYDAV_SMTP_PORT",
		"TIDYDAV_SMTP_ENCRYPTION",
	} {
		t.Setenv(k, "")
	}
}

func TestLoadDefaults(t *testing.T) {
	setMinimalEnv(t)

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}

	if cfg.DBPath != "/data/tidydav.db" {
		t.Errorf("DBPath = %q, want /data/tidydav.db", cfg.DBPath)
	}
	if cfg.ListenAddr != ":8080" {
		t.Errorf("ListenAddr = %q, want :8080", cfg.ListenAddr)
	}
	if cfg.LogLevel != slog.LevelInfo {
		t.Errorf("LogLevel = %v, want info", cfg.LogLevel)
	}
	if cfg.AccessMode != AccessAuth {
		t.Errorf("AccessMode = %q, want auth", cfg.AccessMode)
	}
	if !cfg.AllowRegistration {
		t.Error("AllowRegistration = false, want true")
	}
	if cfg.SMTP.Port != 587 {
		t.Errorf("SMTP.Port = %d, want 587", cfg.SMTP.Port)
	}
	if cfg.OIDC.Enabled() {
		t.Error("OIDC.Enabled() = true, want false")
	}
	if cfg.SMTP.Enabled() {
		t.Error("SMTP.Enabled() = true, want false")
	}
	if got, want := cfg.OIDC.Scopes, []string{"openid", "profile", "email"}; !equalStrings(got, want) {
		t.Errorf("OIDC.Scopes = %v, want %v", got, want)
	}
}

func TestLoadTrimsBaseURL(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("TIDYDAV_BASE_URL", "https://dav.example.com/")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if cfg.BaseURL != "https://dav.example.com" {
		t.Errorf("BaseURL = %q, want trailing slash trimmed", cfg.BaseURL)
	}
}

func TestLoadOIDCEnabled(t *testing.T) {
	setMinimalEnv(t)
	t.Setenv("TIDYDAV_OIDC_ISSUER_URL", "https://id.example.com")
	t.Setenv("TIDYDAV_OIDC_CLIENT_ID", "client-123")
	t.Setenv("TIDYDAV_OIDC_SCOPES", "openid email")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() returned error: %v", err)
	}
	if !cfg.OIDC.Enabled() {
		t.Fatal("OIDC.Enabled() = false, want true")
	}
	if !equalStrings(cfg.OIDC.Scopes, []string{"openid", "email"}) {
		t.Errorf("OIDC.Scopes = %v, want [openid email]", cfg.OIDC.Scopes)
	}
}

func TestLoadErrors(t *testing.T) {
	tests := []struct {
		name    string
		env     map[string]string
		wantErr bool
	}{
		{
			name:    "missing secret key",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "", "TIDYDAV_BASE_URL": "https://x.example.com"},
			wantErr: true,
		},
		{
			name:    "missing base url",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "s", "TIDYDAV_BASE_URL": ""},
			wantErr: true,
		},
		{
			name:    "relative base url",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "s", "TIDYDAV_BASE_URL": "dav.example.com"},
			wantErr: true,
		},
		{
			name:    "invalid access mode",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "s", "TIDYDAV_BASE_URL": "https://x.example.com", "TIDYDAV_ACCESS_MODE": "open"},
			wantErr: true,
		},
		{
			name:    "invalid log level",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "s", "TIDYDAV_BASE_URL": "https://x.example.com", "TIDYDAV_LOG_LEVEL": "loud"},
			wantErr: true,
		},
		{
			name:    "valid both mode",
			env:     map[string]string{"TIDYDAV_SECRET_KEY": "s", "TIDYDAV_BASE_URL": "https://x.example.com", "TIDYDAV_ACCESS_MODE": "both"},
			wantErr: false,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			setMinimalEnv(t)
			for k, v := range tt.env {
				t.Setenv(k, v)
			}
			_, err := Load()
			if (err != nil) != tt.wantErr {
				t.Fatalf("Load() error = %v, wantErr %v", err, tt.wantErr)
			}
		})
	}
}

func equalStrings(a, b []string) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
