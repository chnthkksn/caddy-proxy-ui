package api

import "net/http"

// settingsResponse is read-only on purpose: everything here is set by
// environment variable at startup, so the UI surfaces it for troubleshooting
// instead of offering fields that couldn't actually take effect.
type settingsResponse struct {
	CaddyAdminURL   string `json:"caddy_admin_url"`
	AccessLogPath   string `json:"access_log_path"`
	CertStoragePath string `json:"cert_storage_path"`
	Version         string `json:"version"`
}

func (s *Server) handleSettings(w http.ResponseWriter, r *http.Request) {
	paths := s.proxy.Paths()
	writeJSON(w, http.StatusOK, settingsResponse{
		CaddyAdminURL:   paths.CaddyAdminURL,
		AccessLogPath:   paths.AccessLogPath,
		CertStoragePath: paths.CertStoragePath,
		Version:         s.version,
	})
}
