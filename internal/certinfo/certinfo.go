// Package certinfo reads real certificate expiry/issuer information straight
// off Caddy's on-disk certificate storage. It never fabricates a value: if
// Caddy's storage root can't be read (e.g. a custom non-file storage backend
// like Consul or S3), Discover reports that explicitly rather than returning
// an empty list that would look identical to "no certificates issued yet".
package certinfo

import (
	"crypto/x509"
	"encoding/pem"
	"os"
	"path/filepath"
	"time"
)

type Cert struct {
	Domain    string // matched against store.Host.Domain by exact string equality
	IssuerKey string // the on-disk issuer directory name, e.g. "acme-v02.api.letsencrypt.org-directory" or "local"
	Issuer    string // the certificate's real issuer CN, from the parsed x509 cert
	NotBefore time.Time
	NotAfter  time.Time
}

// Discover walks storageRoot/certificates/<issuer-key>/<domain>/<domain>.crt
// — a bounded two-level walk (there are only ever a handful of issuer
// directories, and one domain directory per issued cert), not an unbounded
// recursive walk. Confirmed against a real Caddy-issued certificate's
// on-disk layout, not guessed.
//
// available is false when storageRoot itself can't be read at all (wrong
// path, custom storage backend, permission denied) — the caller should show
// "certificate info unavailable", never a silent empty list. available is
// true with an empty Cert slice when the root is readable but no
// certificates have been issued yet, which is a normal state.
//
// Wildcard domains (certmagic transforms "*.example.com" on disk) won't
// match a plain host domain and are silently skipped — a known v1 gap, not
// something this package should paper over by guessing the transform.
func Discover(storageRoot string) (certs []Cert, available bool) {
	certsRoot := filepath.Join(storageRoot, "certificates")
	issuerEntries, err := os.ReadDir(certsRoot)
	if err != nil {
		if os.IsNotExist(err) {
			// storageRoot is readable, it just has no certificates directory
			// yet — nothing has been issued. Confirm storageRoot itself is
			// actually reachable before calling this "available".
			if _, statErr := os.Stat(storageRoot); statErr != nil {
				return nil, false
			}
			return nil, true
		}
		return nil, false
	}

	for _, issuerEntry := range issuerEntries {
		if !issuerEntry.IsDir() {
			continue
		}
		issuerKey := issuerEntry.Name()
		issuerDir := filepath.Join(certsRoot, issuerKey)

		domainEntries, err := os.ReadDir(issuerDir)
		if err != nil {
			continue
		}
		for _, domainEntry := range domainEntries {
			if !domainEntry.IsDir() {
				continue
			}
			domain := domainEntry.Name()
			crtPath := filepath.Join(issuerDir, domain, domain+".crt")
			cert, ok := parseCertFile(crtPath)
			if !ok {
				continue
			}
			certs = append(certs, Cert{
				Domain:    domain,
				IssuerKey: issuerKey,
				Issuer:    cert.Issuer.CommonName,
				NotBefore: cert.NotBefore,
				NotAfter:  cert.NotAfter,
			})
		}
	}
	return certs, true
}

func parseCertFile(path string) (*x509.Certificate, bool) {
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, false
	}
	block, _ := pem.Decode(data)
	if block == nil {
		return nil, false
	}
	cert, err := x509.ParseCertificate(block.Bytes)
	if err != nil {
		return nil, false
	}
	return cert, true
}
