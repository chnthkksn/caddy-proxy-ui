package api

import (
	"net/http"

	"caddy-proxy-ui/internal/trafficlog"
)

type hourBucketResponse struct {
	Hour     string `json:"hour"` // RFC3339, start of the hour, UTC
	Requests int    `json:"requests"`
	Errors   int    `json:"errors"`
}

type logEntryResponse struct {
	Time     string  `json:"time"` // RFC3339
	Domain   string  `json:"domain"`
	Status   int     `json:"status"`
	Duration float64 `json:"duration"` // seconds
}

type trafficResponse struct {
	Available bool                 `json:"available"`
	Hourly    []hourBucketResponse `json:"hourly"`
	Recent    []logEntryResponse   `json:"recent"`
	Total24h  int                  `json:"total_24h"`
	Errors24h int                  `json:"errors_24h"`
	ByDomain  map[string]int       `json:"by_domain"`
}

func (s *Server) handleTraffic(w http.ResponseWriter, r *http.Request) {
	snap, ok := s.proxy.TrafficSnapshot()
	if !ok {
		writeJSON(w, http.StatusOK, trafficResponse{Available: false})
		return
	}
	writeJSON(w, http.StatusOK, toTrafficResponse(snap))
}

func toTrafficResponse(snap trafficlog.Snapshot) trafficResponse {
	hourly := make([]hourBucketResponse, len(snap.Hourly))
	for i, b := range snap.Hourly {
		hourly[i] = hourBucketResponse{
			Hour:     b.HourStart.Format("2006-01-02T15:04:05Z07:00"),
			Requests: b.Requests,
			Errors:   b.Errors,
		}
	}
	recent := make([]logEntryResponse, len(snap.Recent))
	for i, e := range snap.Recent {
		recent[i] = logEntryResponse{
			Time:     e.Time.Format("2006-01-02T15:04:05.000Z07:00"),
			Domain:   e.Domain,
			Status:   e.Status,
			Duration: e.Duration,
		}
	}
	byDomain := snap.ByDomain
	if byDomain == nil {
		byDomain = map[string]int{}
	}
	return trafficResponse{
		Available: true,
		Hourly:    hourly,
		Recent:    recent,
		Total24h:  snap.Total24h,
		Errors24h: snap.Errors24h,
		ByDomain:  byDomain,
	}
}
