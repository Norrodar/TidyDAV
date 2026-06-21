package server

import (
	"errors"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Norrodar/TidyDAV/internal/dav"
	"github.com/Norrodar/TidyDAV/internal/store"
)

type syncJobRequest struct {
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Direction       string `json:"direction"`
	Conflict        string `json:"conflict"`
	AURL            string `json:"aUrl"`
	AUsername       string `json:"aUsername"`
	APassword       string `json:"aPassword"` // write-only
	BURL            string `json:"bUrl"`
	BUsername       string `json:"bUsername"`
	BPassword       string `json:"bPassword"` // write-only
	IntervalSeconds int    `json:"intervalSeconds"`
	Enabled         bool   `json:"enabled"`
	WindowStart     string `json:"windowStart"`
	WindowEnd       string `json:"windowEnd"`
}

type syncJobResponse struct {
	ID              string `json:"id"`
	Name            string `json:"name"`
	Kind            string `json:"kind"`
	Direction       string `json:"direction"`
	Conflict        string `json:"conflict"`
	AURL            string `json:"aUrl"`
	AUsername       string `json:"aUsername"`
	APasswordSet    bool   `json:"aPasswordSet"`
	BURL            string `json:"bUrl"`
	BUsername       string `json:"bUsername"`
	BPasswordSet    bool   `json:"bPasswordSet"`
	IntervalSeconds int    `json:"intervalSeconds"`
	Enabled         bool   `json:"enabled"`
	WindowStart     string `json:"windowStart"`
	WindowEnd       string `json:"windowEnd"`
	LastRunAt       string `json:"lastRunAt"`
	LastStatus      string `json:"lastStatus"`
	CreatedAt       string `json:"createdAt"`
	UpdatedAt       string `json:"updatedAt"`
}

func (s *Server) handleListSyncJobs(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	jobs, err := s.app.Store.SyncJobsByUser(r.Context(), u.ID)
	if err != nil {
		s.serverError(w, "list sync jobs", err)
		return
	}
	out := make([]syncJobResponse, 0, len(jobs))
	for _, j := range jobs {
		out = append(out, toSyncJobResponse(j))
	}
	writeJSON(w, http.StatusOK, out)
}

func (s *Server) handleCreateSyncJob(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req syncJobRequest
	if !decodeJSON(w, r, &req) || !validateSyncJobRequest(w, &req) {
		return
	}
	id, err := randomToken()
	if err != nil {
		s.serverError(w, "generate id", err)
		return
	}
	job := jobFromRequest(id, u.ID, &req, nil)
	if err := s.app.Store.CreateSyncJob(r.Context(), job); err != nil {
		s.serverError(w, "create sync job", err)
		return
	}
	s.app.Audit.Record(r.Context(), u, "sync.create", job.ID, job.Name)
	writeJSON(w, http.StatusCreated, toSyncJobResponse(job))
}

func (s *Server) handleGetSyncJob(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	job, ok := s.ownedSyncJob(w, r, u)
	if !ok {
		return
	}
	writeJSON(w, http.StatusOK, toSyncJobResponse(job))
}

func (s *Server) handleUpdateSyncJob(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	existing, ok := s.ownedSyncJob(w, r, u)
	if !ok {
		return
	}
	var req syncJobRequest
	if !decodeJSON(w, r, &req) || !validateSyncJobRequest(w, &req) {
		return
	}
	job := jobFromRequest(existing.ID, u.ID, &req, existing)
	if err := s.app.Store.UpdateSyncJob(r.Context(), job); err != nil {
		s.serverError(w, "update sync job", err)
		return
	}
	s.app.Audit.Record(r.Context(), u, "sync.update", job.ID, job.Name)
	// Reload to return persisted run metadata.
	if reloaded, err := s.app.Store.SyncJobByID(r.Context(), job.ID); err == nil {
		job = reloaded
	}
	writeJSON(w, http.StatusOK, toSyncJobResponse(job))
}

func (s *Server) handleDeleteSyncJob(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	err := s.app.Store.DeleteSyncJob(r.Context(), r.PathValue("id"), u.ID)
	if errors.Is(err, store.ErrNotFound) {
		http.NotFound(w, r)
		return
	}
	if err != nil {
		s.serverError(w, "delete sync job", err)
		return
	}
	s.app.Audit.Record(r.Context(), u, "sync.delete", r.PathValue("id"), "")
	w.WriteHeader(http.StatusNoContent)
}

func (s *Server) handleRunSyncJob(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	job, ok := s.ownedSyncJob(w, r, u)
	if !ok {
		return
	}
	s.app.Sync.RunOne(r.Context(), job)
	if reloaded, err := s.app.Store.SyncJobByID(r.Context(), job.ID); err == nil {
		job = reloaded
	}
	writeJSON(w, http.StatusOK, toSyncJobResponse(job))
}

// ── Merge preview ────────────────────────────────────────────────────────────

type syncPreviewRequest struct {
	ID          string `json:"id"` // preview a saved job: reuse stored passwords
	Kind        string `json:"kind"`
	Direction   string `json:"direction"`
	AURL        string `json:"aUrl"`
	AUsername   string `json:"aUsername"`
	APassword   string `json:"aPassword"`
	BURL        string `json:"bUrl"`
	BUsername   string `json:"bUsername"`
	BPassword   string `json:"bPassword"`
	WindowStart string `json:"windowStart"`
	WindowEnd   string `json:"windowEnd"`
	WeekStart   string `json:"weekStart"` // ISO date; restricts the preview to that week
}

type syncPreviewResponse struct {
	A      []dav.PreviewEntry `json:"a"`
	B      []dav.PreviewEntry `json:"b"`
	Merged []dav.PreviewEntry `json:"merged"`
}

// handleSyncPreview fetches a date-windowed snapshot of both servers and returns
// each side plus the simulated merge for the chosen direction. It makes live DAV
// calls and never writes to either server.
func (s *Server) handleSyncPreview(w http.ResponseWriter, r *http.Request) {
	u, ok := s.requireUser(w, r)
	if !ok {
		return
	}
	var req syncPreviewRequest
	if !decodeJSON(w, r, &req) {
		return
	}

	if req.Kind != "caldav" && req.Kind != "carddav" {
		writeError(w, http.StatusBadRequest, "kind must be caldav or carddav")
		return
	}
	var opts dav.Options
	switch dav.Direction(req.Direction) {
	case dav.AToB, dav.BToA, dav.Bidirectional:
		opts.Direction = dav.Direction(req.Direction)
	default:
		writeError(w, http.StatusBadRequest, "invalid direction")
		return
	}
	for _, raw := range []string{req.AURL, req.BURL} {
		parsed, err := url.Parse(raw)
		if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
			writeError(w, http.StatusBadRequest, "both endpoints need a valid http(s) URL")
			return
		}
	}

	opts.UID = dav.CalendarUID
	if req.Kind == "carddav" {
		opts.UID = dav.ContactUID
	}

	// Date window (CalDAV only): an explicit preview week wins, else the saved range.
	if req.Kind == "caldav" {
		if ws := strings.TrimSpace(req.WeekStart); ws != "" {
			start, _, err := dav.ParseWindow(ws, "")
			if err != nil {
				writeError(w, http.StatusBadRequest, "invalid week: "+err.Error())
				return
			}
			opts.WindowStart, opts.WindowEnd = start, start.Add(7*24*time.Hour)
		} else if ws, we, err := dav.ParseWindow(strings.TrimSpace(req.WindowStart), strings.TrimSpace(req.WindowEnd)); err == nil {
			opts.WindowStart, opts.WindowEnd = ws, we
		} else {
			writeError(w, http.StatusBadRequest, "invalid date range: "+err.Error())
			return
		}
	}

	// Reuse stored passwords when previewing a saved job.
	aPass, bPass := req.APassword, req.BPassword
	if req.ID != "" {
		if existing, err := s.app.Store.SyncJobByID(r.Context(), req.ID); err == nil && existing.UserID == u.ID {
			if aPass == "" {
				aPass = existing.APassword
			}
			if bPass == "" {
				bPass = existing.BPassword
			}
		}
	}

	a, b, err := buildPreviewCollections(req.Kind, req.AURL, req.AUsername, aPass, req.BURL, req.BUsername, bPass)
	if err != nil {
		writeError(w, http.StatusBadRequest, err.Error())
		return
	}

	aOut, bOut, merged, err := dav.PreviewMerge(r.Context(), a, b, opts, req.Kind)
	if err != nil {
		s.app.Log.Warn("sync preview failed", "error", err)
		writeError(w, http.StatusBadGateway, "preview failed: "+err.Error())
		return
	}
	writeJSON(w, http.StatusOK, syncPreviewResponse{A: aOut, B: bOut, Merged: merged})
}

func buildPreviewCollections(kind, aURL, aUser, aPass, bURL, bUser, bPass string) (dav.Collection, dav.Collection, error) {
	if kind == "carddav" {
		a, err := dav.NewCardDAVCollection(strings.TrimSpace(aURL), aUser, aPass)
		if err != nil {
			return nil, nil, err
		}
		b, err := dav.NewCardDAVCollection(strings.TrimSpace(bURL), bUser, bPass)
		if err != nil {
			return nil, nil, err
		}
		return a, b, nil
	}
	a, err := dav.NewCalDAVCollection(strings.TrimSpace(aURL), aUser, aPass)
	if err != nil {
		return nil, nil, err
	}
	b, err := dav.NewCalDAVCollection(strings.TrimSpace(bURL), bUser, bPass)
	if err != nil {
		return nil, nil, err
	}
	return a, b, nil
}

// ── Helpers ─────────────────────────────────────────────────────────────────

func (s *Server) ownedSyncJob(w http.ResponseWriter, r *http.Request, u *store.User) (*store.SyncJob, bool) {
	job, err := s.app.Store.SyncJobByID(r.Context(), r.PathValue("id"))
	if errors.Is(err, store.ErrNotFound) || (job != nil && job.UserID != u.ID) {
		http.NotFound(w, r)
		return nil, false
	}
	if err != nil {
		s.serverError(w, "load sync job", err)
		return nil, false
	}
	return job, true
}

func jobFromRequest(id, userID string, req *syncJobRequest, existing *store.SyncJob) *store.SyncJob {
	conflict := req.Conflict
	if conflict == "" {
		conflict = "newest-wins"
	}
	interval := req.IntervalSeconds
	if interval <= 0 {
		interval = 900
	}
	job := &store.SyncJob{
		ID: id, UserID: userID,
		Name:            strings.TrimSpace(req.Name),
		Kind:            req.Kind,
		Direction:       req.Direction,
		Conflict:        conflict,
		AURL:            strings.TrimSpace(req.AURL),
		AUsername:       req.AUsername,
		APassword:       req.APassword,
		BURL:            strings.TrimSpace(req.BURL),
		BUsername:       req.BUsername,
		BPassword:       req.BPassword,
		IntervalSeconds: interval,
		Enabled:         req.Enabled,
		WindowStart:     strings.TrimSpace(req.WindowStart),
		WindowEnd:       strings.TrimSpace(req.WindowEnd),
	}
	// Passwords are write-only: keep stored ones when the request omits them.
	if existing != nil {
		if job.APassword == "" {
			job.APassword = existing.APassword
		}
		if job.BPassword == "" {
			job.BPassword = existing.BPassword
		}
	}
	return job
}

func toSyncJobResponse(j *store.SyncJob) syncJobResponse {
	lastRunAt := ""
	if !j.LastRunAt.IsZero() {
		lastRunAt = j.LastRunAt.UTC().Format(time.RFC3339)
	}
	return syncJobResponse{
		ID:              j.ID,
		Name:            j.Name,
		Kind:            j.Kind,
		Direction:       j.Direction,
		Conflict:        j.Conflict,
		AURL:            j.AURL,
		AUsername:       j.AUsername,
		APasswordSet:    j.APassword != "",
		BURL:            j.BURL,
		BUsername:       j.BUsername,
		BPasswordSet:    j.BPassword != "",
		IntervalSeconds: j.IntervalSeconds,
		Enabled:         j.Enabled,
		WindowStart:     j.WindowStart,
		WindowEnd:       j.WindowEnd,
		LastRunAt:       lastRunAt,
		LastStatus:      j.LastStatus,
		CreatedAt:       j.CreatedAt.Format(time.RFC3339),
		UpdatedAt:       j.UpdatedAt.Format(time.RFC3339),
	}
}

func validateSyncJobRequest(w http.ResponseWriter, req *syncJobRequest) bool {
	if strings.TrimSpace(req.Name) == "" {
		writeError(w, http.StatusBadRequest, "name is required")
		return false
	}
	switch req.Kind {
	case "caldav", "carddav":
	default:
		writeError(w, http.StatusBadRequest, "kind must be caldav or carddav")
		return false
	}
	switch req.Direction {
	case "a-to-b", "b-to-a", "bidirectional":
	default:
		writeError(w, http.StatusBadRequest, "invalid direction")
		return false
	}
	switch req.Conflict {
	case "", "newest-wins", "source-wins":
	default:
		writeError(w, http.StatusBadRequest, "invalid conflict policy")
		return false
	}
	for _, raw := range []string{req.AURL, req.BURL} {
		u, err := url.Parse(raw)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" {
			writeError(w, http.StatusBadRequest, "both endpoints need a valid http(s) URL")
			return false
		}
	}
	if _, _, err := dav.ParseWindow(strings.TrimSpace(req.WindowStart), strings.TrimSpace(req.WindowEnd)); err != nil {
		writeError(w, http.StatusBadRequest, "invalid date range: "+err.Error())
		return false
	}
	return true
}
