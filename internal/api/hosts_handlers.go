package api

import (
	"encoding/json"
	"errors"
	"net/http"
	"strconv"
	"strings"

	"caddy-proxy-ui/internal/service"
	"caddy-proxy-ui/internal/store"
)

type hostRequest struct {
	Domain         string            `json:"domain"`
	Upstream       string            `json:"upstream"`
	RequestHeaders map[string]string `json:"request_headers"`
	Enabled        *bool             `json:"enabled"`
}

type hostResponse struct {
	ID             int64             `json:"id"`
	Domain         string            `json:"domain"`
	Upstream       string            `json:"upstream"`
	RequestHeaders map[string]string `json:"request_headers"`
	Enabled        bool              `json:"enabled"`
	CreatedAt      string            `json:"created_at"`
	UpdatedAt      string            `json:"updated_at"`
}

type syncStatusResponse struct {
	Synced bool   `json:"synced"`
	Error  string `json:"error,omitempty"`
}

func toHostResponse(h store.Host) hostResponse {
	headers := map[string]string{}
	if h.RequestHeaders != "" {
		_ = json.Unmarshal([]byte(h.RequestHeaders), &headers)
	}
	return hostResponse{
		ID:             h.ID,
		Domain:         h.Domain,
		Upstream:       h.Upstream,
		RequestHeaders: headers,
		Enabled:        h.Enabled,
		CreatedAt:      h.CreatedAt,
		UpdatedAt:      h.UpdatedAt,
	}
}

func toSyncResponse(r service.SyncResult) syncStatusResponse {
	return syncStatusResponse{Synced: r.Synced, Error: r.Error}
}

func writeHostError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "host not found")
	case errors.Is(err, store.ErrDuplicateDomain):
		writeError(w, http.StatusConflict, "a host for this domain already exists")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}

func parseHostID(w http.ResponseWriter, r *http.Request) (int64, bool) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid host id")
		return 0, false
	}
	return id, true
}

func (s *Server) handleListHosts(w http.ResponseWriter, r *http.Request) {
	hosts, err := s.proxy.ListHosts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load hosts")
		return
	}
	resp := make([]hostResponse, len(hosts))
	for i, h := range hosts {
		resp[i] = toHostResponse(h)
	}
	writeJSON(w, http.StatusOK, resp)
}

func (s *Server) handleGetHost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHostID(w, r)
	if !ok {
		return
	}
	h, err := s.proxy.GetHost(id)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, toHostResponse(h))
}

func (s *Server) handleCreateHost(w http.ResponseWriter, r *http.Request) {
	var req hostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	domain := strings.TrimSpace(req.Domain)
	upstream := strings.TrimSpace(req.Upstream)
	if domain == "" || upstream == "" {
		writeError(w, http.StatusBadRequest, "domain and upstream are required")
		return
	}
	headersJSON, err := json.Marshal(req.RequestHeaders)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request headers")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	h, sync, err := s.proxy.CreateHost(r.Context(), domain, upstream, string(headersJSON), enabled)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusCreated, map[string]any{
		"host": toHostResponse(h),
		"sync": toSyncResponse(sync),
	})
}

func (s *Server) handleUpdateHost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHostID(w, r)
	if !ok {
		return
	}
	var req hostRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	domain := strings.TrimSpace(req.Domain)
	upstream := strings.TrimSpace(req.Upstream)
	if domain == "" || upstream == "" {
		writeError(w, http.StatusBadRequest, "domain and upstream are required")
		return
	}
	headersJSON, err := json.Marshal(req.RequestHeaders)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid request headers")
		return
	}
	enabled := true
	if req.Enabled != nil {
		enabled = *req.Enabled
	}

	h, sync, err := s.proxy.UpdateHost(r.Context(), id, domain, upstream, string(headersJSON), enabled)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host": toHostResponse(h),
		"sync": toSyncResponse(sync),
	})
}

func (s *Server) handleDeleteHost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHostID(w, r)
	if !ok {
		return
	}
	sync, err := s.proxy.DeleteHost(r.Context(), id)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync": toSyncResponse(sync)})
}

func (s *Server) handleToggleHost(w http.ResponseWriter, r *http.Request) {
	id, ok := parseHostID(w, r)
	if !ok {
		return
	}
	current, err := s.proxy.GetHost(id)
	if err != nil {
		writeHostError(w, err)
		return
	}
	h, sync, err := s.proxy.ToggleHost(r.Context(), id, !current.Enabled)
	if err != nil {
		writeHostError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{
		"host": toHostResponse(h),
		"sync": toSyncResponse(sync),
	})
}

func (s *Server) handleSync(w http.ResponseWriter, r *http.Request) {
	writeJSON(w, http.StatusOK, toSyncResponse(s.proxy.Sync(r.Context())))
}
