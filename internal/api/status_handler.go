package api

import "net/http"

func (s *Server) handleStatus(w http.ResponseWriter, r *http.Request) {
	setupRequired, err := s.auth.SetupRequired()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to check setup status")
		return
	}

	status, err := s.proxy.Status(r.Context())
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load status")
		return
	}

	writeJSON(w, http.StatusOK, map[string]any{
		"setup_required":  setupRequired,
		"caddy_connected": status.CaddyConnected,
		"host_count":      status.HostCount,
		"version":         s.version,
	})
}
