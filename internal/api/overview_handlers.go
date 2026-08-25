package api

import (
	"net/http"
	"time"

	"caddy-proxy-ui/internal/selfstats"
)

type overviewResponse struct {
	HostCount     int `json:"host_count"`
	LiveHostCount int `json:"live_host_count"`

	CertsAvailable         bool   `json:"certs_available"`
	CertCount              int    `json:"cert_count"`
	EarliestExpiringDomain string `json:"earliest_expiring_domain,omitempty"`
	EarliestExpiringAt     string `json:"earliest_expiring_at,omitempty"` // RFC3339

	TrafficAvailable bool `json:"traffic_available"`
	Requests24h      int  `json:"requests_24h"`
	Errors24h        int  `json:"errors_24h"`

	FootprintBytes int64 `json:"footprint_bytes"`
}

func (s *Server) handleOverview(w http.ResponseWriter, r *http.Request) {
	resp := overviewResponse{FootprintBytes: selfstats.RSSBytes()}

	hosts, err := s.proxy.ListHosts()
	if err != nil {
		writeError(w, http.StatusInternalServerError, "failed to load hosts")
		return
	}
	resp.HostCount = len(hosts)
	for _, h := range hosts {
		if h.Enabled {
			resp.LiveHostCount++
		}
	}

	if certs, ok := s.proxy.Certificates(); ok {
		resp.CertsAvailable = true
		resp.CertCount = len(certs)
		var earliest time.Time
		for _, c := range certs {
			if resp.EarliestExpiringDomain == "" || c.NotAfter.Before(earliest) {
				earliest = c.NotAfter
				resp.EarliestExpiringDomain = c.Domain
				resp.EarliestExpiringAt = c.NotAfter.Format("2006-01-02T15:04:05Z07:00")
			}
		}
	}

	if snap, ok := s.proxy.TrafficSnapshot(); ok {
		resp.TrafficAvailable = true
		resp.Requests24h = snap.Total24h
		resp.Errors24h = snap.Errors24h
	}

	writeJSON(w, http.StatusOK, resp)
}
