package server

import (
	"crypto/rand"
	"crypto/subtle"
	"encoding/hex"
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"

	"github.com/Norrodar/TidyDAV/internal/auth"
	"github.com/Norrodar/TidyDAV/internal/config"
	"github.com/Norrodar/TidyDAV/internal/store"
	"golang.org/x/crypto/bcrypt"
	"golang.org/x/oauth2"
)

const (
	oidcStateCookie    = "tidydav_oidc_state"
	oidcVerifierCookie = "tidydav_oidc_verifier"
)

// ── Response/request shapes (kept in sync with web/src/lib/api.ts) ──────────

type healthResponse struct {
	Status  string `json:"status"`
	Version string `json:"version"`
}

type userResponse struct {
	ID        string  `json:"id"`
	Email     *string `json:"email"`
	Kind      string  `json:"kind"`
	IsAdmin   bool    `json:"isAdmin"`
	AvatarURL string  `json:"avatarUrl"`
}

type sessionResponse struct {
	Authenticated       bool          `json:"authenticated"`
	User                *userResponse `json:"user"`
	AccessMode          string        `json:"accessMode"`
	OIDCEnabled         bool          `json:"oidcEnabled"`
	OIDCDisplayName     string        `json:"oidcDisplayName"`
	OIDCOnly            bool          `json:"oidcOnly"`
	RegistrationEnabled bool          `json:"registrationEnabled"`
	MailEnabled         bool          `json:"mailEnabled"`
	AccentColor         string        `json:"accentColor,omitempty"`
	BackgroundAnimation bool          `json:"backgroundAnimation"`
}

type credentialsRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

type secretResponse struct {
	Secret string        `json:"secret"`
	User   *userResponse `json:"user"`
}

// ── Handlers ────────────────────────────────────────────────────────────────

func (s *Server) handleHealth(w http.ResponseWriter, _ *http.Request) {
	writeJSON(w, http.StatusOK, healthResponse{Status: "ok", Version: s.app.Version})
}

func (s *Server) handleSession(w http.ResponseWriter, r *http.Request) {
	u, err := s.app.Auth.UserFromRequest(r.Context(), r)
	if err != nil && !errors.Is(err, store.ErrNotFound) {
		s.app.Log.Error("session lookup failed", "error", err)
	}
	writeJSON(w, http.StatusOK, s.sessionPayload(u))
}

func (s *Server) handleRegister(w http.ResponseWriter, r *http.Request) {
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.app.Auth.Register(r.Context(), req.Email, req.Password)
	switch {
	case errors.Is(err, auth.ErrRegistrationClosed):
		writeError(w, http.StatusForbidden, "registration is disabled")
		return
	case errors.Is(err, auth.ErrEmailTaken):
		writeError(w, http.StatusConflict, "email already registered")
		return
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusBadRequest, "email and password are required")
		return
	case err != nil:
		s.serverError(w, "register", err)
		return
	}
	if err := s.app.Auth.StartSession(r.Context(), w, u); err != nil {
		s.serverError(w, "start session", err)
		return
	}
	writeJSON(w, http.StatusCreated, s.sessionPayload(u))
}

func (s *Server) handleLogin(w http.ResponseWriter, r *http.Request) {
	if s.app.Auth.OIDCOnly() {
		writeError(w, http.StatusForbidden, "password login is disabled; use OIDC")
		return
	}
	var req credentialsRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	u, err := s.app.Auth.Authenticate(r.Context(), req.Email, req.Password)
	if errors.Is(err, auth.ErrInvalidCredentials) {
		writeError(w, http.StatusUnauthorized, "invalid email or password")
		return
	}
	if err != nil {
		s.serverError(w, "authenticate", err)
		return
	}
	if err := s.app.Auth.StartSession(r.Context(), w, u); err != nil {
		s.serverError(w, "start session", err)
		return
	}
	writeJSON(w, http.StatusOK, s.sessionPayload(u))
}

func (s *Server) handleLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Auth.Logout(r.Context(), w, r); err != nil {
		s.serverError(w, "logout", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleSecret(w http.ResponseWriter, r *http.Request) {
	u, secret, err := s.app.Auth.CreateSecretUser(r.Context())
	if errors.Is(err, auth.ErrAnonymousDisabled) {
		writeError(w, http.StatusForbidden, "anonymous access is disabled")
		return
	}
	if err != nil {
		s.serverError(w, "create secret user", err)
		return
	}
	if err := s.app.Auth.StartSession(r.Context(), w, u); err != nil {
		s.serverError(w, "start session", err)
		return
	}
	writeJSON(w, http.StatusOK, secretResponse{Secret: secret, User: toUserResponse(u)})
}

func (s *Server) handleOIDCLogin(w http.ResponseWriter, r *http.Request) {
	state, err := randomToken()
	if err != nil {
		s.serverError(w, "oidc state", err)
		return
	}

	// Generate PKCE verifier; the challenge is embedded in the auth URL.
	verifier := oauth2.GenerateVerifier()

	authURL, err := s.app.Auth.OIDCAuthCodeURL(state, verifier)
	if errors.Is(err, auth.ErrOIDCNotConfigured) {
		writeError(w, http.StatusNotFound, "oidc is not configured")
		return
	}
	if err != nil {
		s.serverError(w, "oidc auth url", err)
		return
	}

	secure := secureCookies(s.app.Config)
	http.SetCookie(w, &http.Cookie{
		Name:     oidcStateCookie,
		Value:    state,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.SetCookie(w, &http.Cookie{
		Name:     oidcVerifierCookie,
		Value:    verifier,
		Path:     "/",
		HttpOnly: true,
		Secure:   secure,
		SameSite: http.SameSiteLaxMode,
		MaxAge:   600,
	})
	http.Redirect(w, r, authURL, http.StatusFound)
}

func (s *Server) handleOIDCCallback(w http.ResponseWriter, r *http.Request) {
	stateCookie, err := r.Cookie(oidcStateCookie)
	if err != nil || stateCookie.Value == "" || stateCookie.Value != r.URL.Query().Get("state") {
		writeError(w, http.StatusBadRequest, "invalid oauth state")
		return
	}
	verifierCookie, err := r.Cookie(oidcVerifierCookie)
	if err != nil || verifierCookie.Value == "" {
		writeError(w, http.StatusBadRequest, "missing pkce verifier")
		return
	}
	code := r.URL.Query().Get("code")
	if code == "" {
		writeError(w, http.StatusBadRequest, "missing authorization code")
		return
	}

	u, err := s.app.Auth.OIDCExchange(r.Context(), code, verifierCookie.Value)
	if errors.Is(err, auth.ErrGroupNotAllowed) {
		writeError(w, http.StatusForbidden, "your account is not in an allowed group")
		return
	}
	if err != nil {
		s.serverError(w, "oidc exchange", err)
		return
	}
	if err := s.app.Auth.StartSession(r.Context(), w, u); err != nil {
		s.serverError(w, "start session", err)
		return
	}
	// Clear PKCE/state cookies.
	http.SetCookie(w, &http.Cookie{Name: oidcStateCookie, Value: "", Path: "/", MaxAge: -1})
	http.SetCookie(w, &http.Cookie{Name: oidcVerifierCookie, Value: "", Path: "/", MaxAge: -1})
	http.Redirect(w, r, "/", http.StatusFound)
}

// handleOIDCLogout clears the TidyDAV session and redirects to the OIDC
// provider's end_session_endpoint when available, falling back to the home page.
func (s *Server) handleOIDCLogout(w http.ResponseWriter, r *http.Request) {
	if err := s.app.Auth.Logout(r.Context(), w, r); err != nil {
		s.serverError(w, "oidc logout", err)
		return
	}

	endSession := s.app.Auth.OIDCEndSessionURL()
	if endSession == "" {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}

	// Build the provider logout URL with post_logout_redirect_uri pointing home.
	u, err := url.Parse(endSession)
	if err != nil {
		http.Redirect(w, r, "/", http.StatusFound)
		return
	}
	redirect := s.app.Config.OIDC.PostLogoutRedirectURI
	if redirect == "" {
		redirect = s.app.Config.BaseURL + "/"
	}
	q := u.Query()
	q.Set("client_id", s.app.Config.OIDC.ClientID)
	q.Set("post_logout_redirect_uri", redirect)
	u.RawQuery = q.Encode()
	http.Redirect(w, r, u.String(), http.StatusFound)
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *Server) sessionPayload(u *store.User) sessionResponse {
	return sessionResponse{
		Authenticated:       u != nil,
		User:                toUserResponse(u),
		AccessMode:          string(s.app.Config.AccessMode),
		OIDCEnabled:         s.app.Auth.OIDCEnabled(),
		OIDCDisplayName:     s.app.Auth.OIDCDisplayName(),
		OIDCOnly:            s.app.Auth.OIDCOnly(),
		RegistrationEnabled: s.app.Auth.RegistrationEnabled(),
		MailEnabled:         s.app.Auth.MailEnabled(),
		AccentColor:         s.app.Config.AccentColor,
		BackgroundAnimation: s.app.Config.BackgroundAnimation,
	}
}

func toUserResponse(u *store.User) *userResponse {
	if u == nil {
		return nil
	}
	var email *string
	if u.Email.Valid {
		e := u.Email.String
		email = &e
	}
	return &userResponse{
		ID:        u.ID,
		Email:     email,
		Kind:      u.Kind,
		IsAdmin:   u.IsAdmin,
		AvatarURL: u.AvatarURL,
	}
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(v)
}

func writeError(w http.ResponseWriter, status int, msg string) {
	writeJSON(w, status, map[string]string{"error": msg})
}

func decodeJSON(w http.ResponseWriter, r *http.Request, dst any) bool {
	if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 1<<20)).Decode(dst); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return false
	}
	return true
}

func (s *Server) serverError(w http.ResponseWriter, op string, err error) {
	s.app.Log.Error("request failed", "op", op, "error", err)
	writeError(w, http.StatusInternalServerError, "internal server error")
}

func randomToken() (string, error) {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return hex.EncodeToString(b[:]), nil
}

func secureCookies(cfg *config.Config) bool {
	return strings.HasPrefix(cfg.BaseURL, "https://")
}

type resetRequestBody struct {
	Email string `json:"email"`
}

func (s *Server) handleResetRequest(w http.ResponseWriter, r *http.Request) {
	var req resetRequestBody
	if !decodeJSON(w, r, &req) {
		return
	}
	if err := s.app.Auth.RequestPasswordReset(r.Context(), req.Email); err != nil {
		s.app.Log.Warn("password reset request failed", "error", err)
	}
	// Always 204 so the response doesn't reveal whether the email exists.
	w.WriteHeader(http.StatusNoContent)
}

type resetConfirmBody struct {
	Token    string `json:"token"`
	Password string `json:"password"`
}

func (s *Server) handleResetConfirm(w http.ResponseWriter, r *http.Request) {
	var req resetConfirmBody
	if !decodeJSON(w, r, &req) {
		return
	}
	err := s.app.Auth.ConfirmPasswordReset(r.Context(), req.Token, req.Password)
	switch {
	case errors.Is(err, auth.ErrInvalidResetToken):
		writeError(w, http.StatusBadRequest, "invalid or expired reset link")
	case errors.Is(err, auth.ErrInvalidCredentials):
		writeError(w, http.StatusBadRequest, "a new password is required")
	case err != nil:
		s.serverError(w, "reset confirm", err)
	default:
		w.WriteHeader(http.StatusNoContent)
	}
}

// handleICS serves a transformed feed at /ics/{secret}. It is secured by the
// secret-id in the path and, optionally, HTTP Basic Auth — never by session
// cookies, since calendar clients cannot do OIDC.
func (s *Server) handleICS(w http.ResponseWriter, r *http.Request) {
	f, err := s.app.Store.FeedBySecret(r.Context(), r.PathValue("secret"))
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, "feed lookup", err)
		return
	}

	if f.BasicAuthHash != "" && !validBasicAuth(r, f) {
		w.Header().Set("WWW-Authenticate", `Basic realm="TidyDAV"`)
		writeError(w, http.StatusUnauthorized, "authentication required")
		return
	}

	body, err := s.app.Feed.Render(r.Context(), f)
	if err != nil {
		s.app.Log.Error("feed render failed", "feed", f.ID, "error", err)
		writeError(w, http.StatusBadGateway, "feed could not be rendered")
		return
	}
	w.Header().Set("Content-Type", "text/calendar; charset=utf-8")
	w.Header().Set("Cache-Control", "no-cache")
	_, _ = w.Write(body)
}

func validBasicAuth(r *http.Request, f *store.Feed) bool {
	user, pass, ok := r.BasicAuth()
	if !ok {
		return false
	}
	if subtle.ConstantTimeCompare([]byte(user), []byte(f.BasicAuthUser)) != 1 {
		return false
	}
	return bcrypt.CompareHashAndPassword([]byte(f.BasicAuthHash), []byte(pass)) == nil
}
