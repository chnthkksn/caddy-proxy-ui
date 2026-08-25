// Package api exposes the REST API over ProxyService and AuthService.
package api

import (
	"encoding/json"
	"log"
	"net/http"

	"caddy-proxy-ui/internal/service"
)

type Server struct {
	proxy        *service.ProxyService
	auth         *service.AuthService
	cookieSecure bool
	version      string
}

func NewServer(proxy *service.ProxyService, auth *service.AuthService, cookieSecure bool, version string) *Server {
	return &Server{proxy: proxy, auth: auth, cookieSecure: cookieSecure, version: version}
}

// Routes returns the API mux only (no static asset serving — that's wired
// up separately in cmd/caddy-ui alongside internal/webui).
func (s *Server) Routes() *http.ServeMux {
	mux := http.NewServeMux()

	mux.HandleFunc("POST /api/setup", s.handleSetup)
	mux.HandleFunc("POST /api/login", s.handleLogin)
	mux.HandleFunc("POST /api/logout", s.handleLogout)
	mux.HandleFunc("GET /api/status", s.handleStatus)
	mux.Handle("POST /api/change-password", s.requireAuth(http.HandlerFunc(s.handleChangePassword)))

	mux.Handle("GET /api/hosts", s.requireAuth(http.HandlerFunc(s.handleListHosts)))
	mux.Handle("POST /api/hosts", s.requireAuth(http.HandlerFunc(s.handleCreateHost)))
	mux.Handle("GET /api/hosts/{id}", s.requireAuth(http.HandlerFunc(s.handleGetHost)))
	mux.Handle("PUT /api/hosts/{id}", s.requireAuth(http.HandlerFunc(s.handleUpdateHost)))
	mux.Handle("DELETE /api/hosts/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteHost)))
	mux.Handle("POST /api/hosts/{id}/toggle", s.requireAuth(http.HandlerFunc(s.handleToggleHost)))

	mux.Handle("GET /api/hosts/{id}/access-rules", s.requireAuth(http.HandlerFunc(s.handleListAccessRules)))
	mux.Handle("POST /api/hosts/{id}/access-rules", s.requireAuth(http.HandlerFunc(s.handleCreateAccessRule)))
	mux.Handle("DELETE /api/access-rules/{id}", s.requireAuth(http.HandlerFunc(s.handleDeleteAccessRule)))

	mux.Handle("GET /api/traffic", s.requireAuth(http.HandlerFunc(s.handleTraffic)))
	mux.Handle("GET /api/certificates", s.requireAuth(http.HandlerFunc(s.handleCertificates)))
	mux.Handle("GET /api/overview", s.requireAuth(http.HandlerFunc(s.handleOverview)))
	mux.Handle("GET /api/settings", s.requireAuth(http.HandlerFunc(s.handleSettings)))

	mux.Handle("POST /api/sync", s.requireAuth(http.HandlerFunc(s.handleSync)))
	mux.Handle("POST /api/import", s.requireAuth(http.HandlerFunc(s.handleImport)))
	mux.Handle("GET /api/export", s.requireAuth(http.HandlerFunc(s.handleExport)))

	return mux
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	if v == nil {
		return
	}
	if err := json.NewEncoder(w).Encode(v); err != nil {
		log.Printf("encode response: %v", err)
	}
}

func writeError(w http.ResponseWriter, status int, message string) {
	writeJSON(w, status, map[string]string{"error": message})
}
