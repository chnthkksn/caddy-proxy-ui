package api

import (
	"encoding/json"
	"io"
	"net/http"
)

type importRequest struct {
	Caddyfile string `json:"caddyfile"`
}

type skippedHostResponse struct {
	Domain string `json:"domain"`
	Reason string `json:"reason"`
}

func (s *Server) handleImport(w http.ResponseWriter, r *http.Request) {
	var req importRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeError(w, http.StatusBadRequest, "invalid request body")
		return
	}
	if req.Caddyfile == "" {
		writeError(w, http.StatusBadRequest, "caddyfile text is required")
		return
	}

	result, sync, err := s.proxy.Import(r.Context(), req.Caddyfile)
	if err != nil {
		writeError(w, http.StatusBadRequest, "failed to import: "+err.Error())
		return
	}

	skipped := make([]skippedHostResponse, len(result.Skipped))
	for i, sk := range result.Skipped {
		skipped[i] = skippedHostResponse{Domain: sk.Domain, Reason: sk.Reason}
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"imported": result.Imported,
		"skipped":  skipped,
		"sync":     toSyncResponse(sync),
	})
}

func (s *Server) handleExport(w http.ResponseWriter, r *http.Request) {
	text, err := s.proxy.Export()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to export")
		return
	}
	w.Header().Set("Content-Type", "text/plain; charset=utf-8")
	w.Header().Set("Content-Disposition", `attachment; filename="Caddyfile"`)
	_, _ = io.WriteString(w, text)
}
