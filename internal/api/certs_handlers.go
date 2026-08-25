package api

import (
	"net/http"
	"sort"
)

type certResponse struct {
	Domain    string `json:"domain"`
	Issuer    string `json:"issuer"`
	NotBefore string `json:"not_before"` // RFC3339
	NotAfter  string `json:"not_after"`  // RFC3339
}

type certificatesResponse struct {
	Available    bool           `json:"available"`
	Certificates []certResponse `json:"certificates"`
}

func (s *Server) handleCertificates(w http.ResponseWriter, r *http.Request) {
	certs, ok := s.proxy.Certificates()
	if !ok {
		writeJSON(w, http.StatusOK, certificatesResponse{Available: false})
		return
	}

	sort.Slice(certs, func(i, j int) bool { return certs[i].NotAfter.Before(certs[j].NotAfter) })

	resp := make([]certResponse, len(certs))
	for i, c := range certs {
		resp[i] = certResponse{
			Domain:    c.Domain,
			Issuer:    c.Issuer,
			NotBefore: c.NotBefore.Format("2006-01-02T15:04:05Z07:00"),
			NotAfter:  c.NotAfter.Format("2006-01-02T15:04:05Z07:00"),
		}
	}
	writeJSON(w, http.StatusOK, certificatesResponse{Available: true, Certificates: resp})
}
