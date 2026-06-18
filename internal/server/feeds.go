package server

import (
	"encoding/json"
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/feed"
	"github.com/Norrodar/TidyDAV/internal/pipeline"
	"github.com/Norrodar/TidyDAV/internal/store"
	"golang.org/x/crypto/bcrypt"
)

// ── DTOs (kept in sync with web/src/lib/api.ts) ─────────────────────────────

type feedSourceDTO struct {
	URL         string `json:"url"`
	Username    string `json:"username,omitempty"`
	Password    string `json:"password,omitempty"`    // write-only
	HasPassword bool   `json:"hasPassword,omitempty"` // read-only
}

type feedRequest struct {
	Name              string          `json:"name"`
	Sources           []feedSourceDTO `json:"sources"`
	Rules             json.RawMessage `json:"rules"`
	TTLSeconds        int             `json:"ttlSeconds"`
	BasicAuthUser     string          `json:"basicAuthUser"`
	BasicAuthPassword string          `json:"basicAuthPassword"` // write-only
}

type feedResponse struct {
	ID               string          `json:"id"`
	Name             string          `json:"name"`
	Secret           string          `json:"secret"`
	ICSURL           string          `json:"icsUrl"`
	Sources          []feedSourceDTO `json:"sources"`
	Rules            json.RawMessage `json:"rules"`
	TTLSeconds       int             `json:"ttlSeconds"`
	BasicAuthUser    string          `json:"basicAuthUser"`
	BasicAuthEnabled bool            `json:"basicAuthEnabled"`
	CreatedAt        string          `json:"createdAt"`
	UpdatedAt        string          `json:"updatedAt"`
}

type previewResponse struct {
	Original    []feed.EventSummary `json:"original"`
	Transformed []feed.EventSummary `json:"transformed"`
}

// ── Handlers ─────────────────────────────────────────────────────────────────

func (s *Server) handleListFeeds(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	feeds, err := s.app.Store.FeedsByUser(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, "list feeds", err)
		return
	}
	out := make([]feedResponse, 0, len(feeds))
	for _, f := range feeds {
		out = append(out, s.toFeedResponse(f))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req feedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validateFeedRequest(w, &req) {
		return
	}

	user, hash, err := resolveBasicAuth(req.BasicAuthUser, req.BasicAuthPassword, nil)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	id, err := randomToken()
	if err != nil {
		s.serverError(w, "generate id", err)
		return
	}
	secret, err := randomToken()
	if err != nil {
		s.serverError(w, "generate secret", err)
		return
	}

	f := &store.Feed{
		ID:            id,
		UserID:        u.ID,
		Name:          strings.TrimSpace(req.Name),
		Secret:        secret,
		Sources:       toStoreSources(req.Sources),
		Rules:         rulesOrDefault(req.Rules),
		TTLSeconds:    normalizeTTL(req.TTLSeconds),
		BasicAuthUser: user,
		BasicAuthHash: hash,
	}
	if err := s.app.Store.CreateFeed(r.Context(), f); err != nil {
		s.serverError(w, "create feed", err)
		return
	}
	writeJSON(w, http.StatusCreated, s.toFeedResponse(f))
}

func (s *Server) handleGetFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	f, ok := s.ownedFeed(w, r, u)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, s.toFeedResponse(f))
}

func (s *Server) handleUpdateFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	existing, ok := s.ownedFeed(w, r, u)
	if !ok {
		return
	}
	var req feedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validateFeedRequest(w, &req) {
		return
	}

	user, hash, err := resolveBasicAuth(req.BasicAuthUser, req.BasicAuthPassword, existing)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}
	sources := toStoreSources(req.Sources)
	preserveSourceSecrets(sources, existing)

	existing.Name = strings.TrimSpace(req.Name)
	existing.Sources = sources
	existing.Rules = rulesOrDefault(req.Rules)
	existing.TTLSeconds = normalizeTTL(req.TTLSeconds)
	existing.BasicAuthUser = user
	existing.BasicAuthHash = hash

	if err := s.app.Store.UpdateFeed(r.Context(), existing); err != nil {
		s.serverError(w, "update feed", err)
		return
	}
	writeJSON(w, http.StatusOK, s.toFeedResponse(existing))
}

func (s *Server) handleDeleteFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	err := s.app.Store.DeleteFeed(r.Context(), r.PathValue("id"), u.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, "delete feed", err)
		return
	}
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handlePreviewFeed(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req feedRequest
	if !decodeJSON(w, r, &req) {
		return
	}
	if !validateRules(w, req.Rules) {
		return
	}

	f := &store.Feed{
		ID:         "preview",
		UserID:     u.ID,
		Sources:    toStoreSources(req.Sources),
		Rules:      rulesOrDefault(req.Rules),
		TTLSeconds: normalizeTTL(req.TTLSeconds),
	}
	original, transformed, err := s.app.Feed.Preview(r.Context(), f)
	if err != nil {
		s.app.Log.Warn("feed preview failed", "error", err)
		writeError(w, http.StatusBadGateway, "preview failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, previewResponse{Original: original, Transformed: transformed})
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *Server) requireUser(w http.ResponseWriter, r *http.Request) (*store.User, bool) {
	u, err := s.app.Auth.UserFromRequest(r.Context(), r)
	if err != nil || u == nil {
		writeError(w, http.StatusUnauthorized, "authentication required")
		return nil, false
	}
	return u, true
}

// ownedFeed loads the feed in the path and ensures it belongs to u.
func (s *Server) ownedFeed(w http.ResponseWriter, r *http.Request, u *store.User) (*store.Feed, bool) {
	f, err := s.app.Store.FeedByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) || (f != nil && f.UserID != u.ID) {
		http.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		s.serverError(w, "load feed", err)
		return nil, false
	}
	return f, true
}

func (s *Server) toFeedResponse(f *store.Feed) feedResponse {
	sources := make([]feedSourceDTO, 0, len(f.Sources))
	for _, src := range f.Sources {
		sources = append(sources, feedSourceDTO{
			URL:         src.URL,
			Username:    src.Username,
			HasPassword: src.Password != "",
		})
	}
	return feedResponse{
		ID:               f.ID,
		Name:             f.Name,
		Secret:           f.Secret,
		ICSURL:           s.app.Config.BaseURL + "/ics/" + f.Secret,
		Sources:          sources,
		Rules:            rulesOrDefault(f.Rules),
		TTLSeconds:       f.TTLSeconds,
		BasicAuthUser:    f.BasicAuthUser,
		BasicAuthEnabled: f.BasicAuthHash != "",
		CreatedAt:        f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:        f.UpdatedAt.Format(time.RFC3339),
	}
}

func validateFeedRequest(w http.ResponseWriter, req *feedRequest) bool {
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return false
	}
	for _, src := range req.Sources {
		u, err := url.Parse(src.URL)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			writeError(w, http.StatusBadRequest, "each source needs a valid http(s) URL")
			return false
		}
	}
	return validateRules(w, req.Rules)
}

func validateRules(w http.ResponseWriter, raw json.RawMessage) bool {
	if len(raw) == 0 {
		return true
	}
	var configs []pipeline.RuleConfig
	if err := json.Unmarshal(raw, &configs); err != nil {
		writeError(w, http.StatusBadRequest, "rules must be a JSON array")
		return false
	}
	if _, err := pipeline.BuildPipeline(configs); err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return false
	}
	return true
}

func resolveBasicAuth(user, password string, existing *store.Feed) (resolvedUser, hash string, err error) {
	user = strings.TrimSpace(user)
	if user == "" {
		return "", "", nil // basic auth disabled
	}
	if password != "" {
		b, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
		if err != nil {
			return "", "", err
		}
		return user, string(b), nil
	}
	if existing != nil && existing.BasicAuthHash != "" {
		return user, existing.BasicAuthHash, nil // keep existing password
	}
	return "", "", errors.New("basic auth password is required")
}

func preserveSourceSecrets(sources []store.FeedSource, existing *store.Feed) {
	if existing == nil {
		return
	}
	old := make(map[string]string, len(existing.Sources))
	for _, src := range existing.Sources {
		if src.Password != "" {
			old[src.URL] = src.Password
		}
	}
	for i := range sources {
		if sources[i].Password == "" {
			if pw, ok := old[sources[i].URL]; ok {
				sources[i].Password = pw
			}
		}
	}
}

func toStoreSources(dtos []feedSourceDTO) []store.FeedSource {
	out := make([]store.FeedSource, 0, len(dtos))
	for _, d := range dtos {
		out = append(out, store.FeedSource{
			URL:      strings.TrimSpace(d.URL),
			Username: d.Username,
			Password: d.Password,
		})
	}
	return out
}

func rulesOrDefault(raw json.RawMessage) json.RawMessage {
	if len(raw) == 0 {
		return json.RawMessage("[]")
	}
	return raw
}

func normalizeTTL(ttl int) int {
	if ttl < 0 {
		return 0
	}
	return ttl
}
