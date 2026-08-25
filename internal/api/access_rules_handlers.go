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

type accessRuleResponse struct {
	ID        int64  `json:"id"`
	HostID    int64  `json:"host_id"`
	Kind      string `json:"kind"`
	Value     string `json:"value"` // for basic_auth: just the username, never the hash
	CreatedAt string `json:"created_at"`
}

func toAccessRuleResponse(r store.AccessRule) accessRuleResponse {
	value := r.Value
	if r.Kind == store.AccessRuleBasicAuth {
		var v struct {
			Username string `json:"username"`
		}
		if err := json.Unmarshal([]byte(r.Value), &v); err == nil {
			value = v.Username
		}
	}
	return accessRuleResponse{
		ID:        r.ID,
		HostID:    r.HostID,
		Kind:      string(r.Kind),
		Value:     value,
		CreatedAt: r.CreatedAt,
	}
}

func (s *Server) handleListAccessRules(w http.ResponseWriter, r *http.Request) {
	hostID, ok := parseHostID(w, r)
	if !ok {
		return
	}
	rules, err := s.proxy.ListAccessRules(hostID)
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load access rules")
		return
	}
	resp := make([]accessRuleResponse, len(rules))
	for i, rule := range rules {
		resp[i] = toAccessRuleResponse(rule)
	}
	writeJSON(w, http.StatusOK, resp)
}

type createAccessRuleRequest struct {
	Kind     string `json:"kind"`
	Username string `json:"username"` // basic_auth only
	Password string `json:"password"` // basic_auth only
	Value    string `json:"value"`    // ip_allow / ip_deny only: a CIDR or bare IP
}

func (s *Server) handleCreateAccessRule(w http.ResponseWriter, r *http.Request) {
	hostID, ok := parseHostID(w, r)
	if !ok {
		return
	}
	var req createAccessRuleRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}

	switch store.AccessRuleKind(req.Kind) {
	case store.AccessRuleBasicAuth:
		username := strings.TrimSpace(req.Username)
		if username == "" || req.Password == "" {
			writeError(w, http.StatusBadRequest, "username and password are required")
			return
		}
		if len(req.Password) < 8 {
			writeError(w, http.StatusBadRequest, "password must be at least 8 characters")
			return
		}
		rule, sync, err := s.proxy.AddBasicAuthRule(r.Context(), hostID, username, req.Password)
		if err != nil {
			writeAccessRuleError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"rule": toAccessRuleResponse(rule),
			"sync": toSyncResponse(sync),
		})

	case store.AccessRuleIPAllow, store.AccessRuleIPDeny:
		value := strings.TrimSpace(req.Value)
		if value == "" {
			writeError(w, http.StatusBadRequest, "value (IP or CIDR) is required")
			return
		}
		rule, sync, err := s.proxy.AddIPRule(r.Context(), hostID, store.AccessRuleKind(req.Kind), value)
		if err != nil {
			writeAccessRuleError(w, err)
			return
		}
		writeJSON(w, http.StatusCreated, map[string]any{
			"rule": toAccessRuleResponse(rule),
			"sync": toSyncResponse(sync),
		})

	default:
		writeError(w, http.StatusBadRequest, "kind must be basic_auth, ip_allow, or ip_deny")
	}
}

func (s *Server) handleDeleteAccessRule(w http.ResponseWriter, r *http.Request) {
	id, err := strconv.ParseInt(r.PathValue("id"), 10, 64)
	if err != nil {
		writeError(w, http.StatusBadRequest, "invalid access rule id")
		return
	}
	sync, err := s.proxy.DeleteAccessRule(r.Context(), id)
	if err != nil {
		writeAccessRuleError(w, err)
		return
	}
	writeJSON(w, http.StatusOK, map[string]any{"sync": toSyncResponse(sync)})
}

func writeAccessRuleError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, store.ErrNotFound):
		writeError(w, http.StatusNotFound, "access rule not found")
	case errors.Is(err, service.ErrInvalidCIDR):
		writeError(w, http.StatusBadRequest, "not a valid IP address or CIDR range")
	default:
		writeError(w, http.StatusInternalServerError, "internal error")
	}
}
