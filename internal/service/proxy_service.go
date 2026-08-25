// Package service holds ProxyService, the only place that touches both
// store.Store and caddyclient.Client. SQLite is the source of truth; every
// mutation writes to it first, then rebuilds and pushes the full config to
// Caddy. A failed push never fails the mutation — Caddy is allowed to be
// temporarily offline, and Sync exists to reconcile once it's back.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"time"

	"golang.org/x/crypto/bcrypt"

	"caddy-proxy-ui/internal/caddyclient"
	"caddy-proxy-ui/internal/caddyconfig"
	"caddy-proxy-ui/internal/caddyfile"
	"caddy-proxy-ui/internal/certinfo"
	"caddy-proxy-ui/internal/store"
	"caddy-proxy-ui/internal/trafficlog"
)

var ErrInvalidCIDR = errors.New("not a valid IP address or CIDR range")

type SyncResult struct {
	Synced bool
	Error  string
}

type ProxyService struct {
	store           *store.Store
	caddy           *caddyclient.Client
	adminListen     string
	accessLogPath   string
	traffic         *trafficlog.Tailer
	certStoragePath string
}

// accessLogPath is where Caddy writes structured access logs for every
// enabled host (see caddyconfig.Build); the same path is tailed by
// trafficlog to answer the Logs page and Overview dashboard. Pass "" to
// disable access logging entirely. certStoragePath is Caddy's own certmagic
// storage root (see internal/certinfo); pass "" if it isn't reachable from
// this process.
func New(s *store.Store, c *caddyclient.Client, adminListen, accessLogPath, certStoragePath string) *ProxyService {
	var traffic *trafficlog.Tailer
	if accessLogPath != "" {
		traffic = trafficlog.New(accessLogPath)
	}
	return &ProxyService{
		store:           s,
		caddy:           c,
		adminListen:     adminListen,
		accessLogPath:   accessLogPath,
		traffic:         traffic,
		certStoragePath: certStoragePath,
	}
}

func (p *ProxyService) sync(ctx context.Context) SyncResult {
	hosts, err := p.store.ListHosts()
	if err != nil {
		return SyncResult{Error: err.Error()}
	}

	ids := make([]int64, len(hosts))
	for i, h := range hosts {
		ids[i] = h.ID
	}
	rules, err := p.store.ListAccessRulesForHosts(ids)
	if err != nil {
		return SyncResult{Error: err.Error()}
	}

	cfg := caddyconfig.Build(hosts, p.adminListen, rules, p.accessLogPath)
	if err := p.caddy.Load(ctx, cfg); err != nil {
		return SyncResult{Error: err.Error()}
	}
	return SyncResult{Synced: true}
}

func (p *ProxyService) Sync(ctx context.Context) SyncResult {
	return p.sync(ctx)
}

func (p *ProxyService) ListHosts() ([]store.Host, error) {
	return p.store.ListHosts()
}

func (p *ProxyService) GetHost(id int64) (store.Host, error) {
	return p.store.GetHost(id)
}

func (p *ProxyService) CreateHost(ctx context.Context, domain, upstream, requestHeaders, groupLabel string, enabled bool) (store.Host, SyncResult, error) {
	h, err := p.store.CreateHost(domain, upstream, requestHeaders, groupLabel, enabled)
	if err != nil {
		return store.Host{}, SyncResult{}, err
	}
	return h, p.sync(ctx), nil
}

func (p *ProxyService) UpdateHost(ctx context.Context, id int64, domain, upstream, requestHeaders, groupLabel string, enabled bool) (store.Host, SyncResult, error) {
	h, err := p.store.UpdateHost(id, domain, upstream, requestHeaders, groupLabel, enabled)
	if err != nil {
		return store.Host{}, SyncResult{}, err
	}
	return h, p.sync(ctx), nil
}

func (p *ProxyService) DeleteHost(ctx context.Context, id int64) (SyncResult, error) {
	if err := p.store.DeleteHost(id); err != nil {
		return SyncResult{}, err
	}
	return p.sync(ctx), nil
}

func (p *ProxyService) ToggleHost(ctx context.Context, id int64, enabled bool) (store.Host, SyncResult, error) {
	h, err := p.store.SetHostEnabled(id, enabled)
	if err != nil {
		return store.Host{}, SyncResult{}, err
	}
	return h, p.sync(ctx), nil
}

// Paths reports where this instance is pointed. Read-only: every one of
// these is set by environment variable at startup (see cmd/caddy-ui/serve.go),
// so the UI shows them for troubleshooting rather than pretending they're
// editable at runtime.
type Paths struct {
	CaddyAdminURL   string
	AccessLogPath   string
	CertStoragePath string
}

func (p *ProxyService) Paths() Paths {
	return Paths{
		CaddyAdminURL:   p.caddy.BaseURL(),
		AccessLogPath:   p.accessLogPath,
		CertStoragePath: p.certStoragePath,
	}
}

// Certificates returns every certificate found in Caddy's on-disk storage.
// ok is false when certStoragePath is unset or unreadable (e.g. a custom
// non-file storage backend) — never a fake empty list in that case.
func (p *ProxyService) Certificates() ([]certinfo.Cert, bool) {
	if p.certStoragePath == "" {
		return nil, false
	}
	return certinfo.Discover(p.certStoragePath)
}

// TrafficSnapshot returns the current rolling 24h traffic window, re-scanning
// whatever is new in the access log since the last call. ok is false when
// access logging isn't configured (empty accessLogPath) — never a fake zero
// snapshot in that case.
func (p *ProxyService) TrafficSnapshot() (trafficlog.Snapshot, bool) {
	if p.traffic == nil {
		return trafficlog.Snapshot{}, false
	}
	return p.traffic.Snapshot(time.Now()), true
}

func (p *ProxyService) ListAccessRules(hostID int64) ([]store.AccessRule, error) {
	return p.store.ListAccessRules(hostID)
}

// AddBasicAuthRule hashes the password with bcrypt before storing it —
// never store plaintext. Reuses the same bcrypt package already imported
// for admin login (internal/service/auth_service.go).
func (p *ProxyService) AddBasicAuthRule(ctx context.Context, hostID int64, username, password string) (store.AccessRule, SyncResult, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return store.AccessRule{}, SyncResult{}, fmt.Errorf("hash password: %w", err)
	}
	value, err := json.Marshal(struct {
		Username   string `json:"username"`
		BcryptHash string `json:"bcrypt_hash"`
	}{Username: username, BcryptHash: string(hash)})
	if err != nil {
		return store.AccessRule{}, SyncResult{}, fmt.Errorf("encode access rule: %w", err)
	}

	rule, err := p.store.CreateAccessRule(hostID, store.AccessRuleBasicAuth, string(value))
	if err != nil {
		return store.AccessRule{}, SyncResult{}, err
	}
	return rule, p.sync(ctx), nil
}

// AddIPRule validates value is a real IP or CIDR before storing it — an
// invalid range would otherwise fail silently at Caddy's end.
func (p *ProxyService) AddIPRule(ctx context.Context, hostID int64, kind store.AccessRuleKind, value string) (store.AccessRule, SyncResult, error) {
	if !isValidIPOrCIDR(value) {
		return store.AccessRule{}, SyncResult{}, ErrInvalidCIDR
	}
	rule, err := p.store.CreateAccessRule(hostID, kind, value)
	if err != nil {
		return store.AccessRule{}, SyncResult{}, err
	}
	return rule, p.sync(ctx), nil
}

func (p *ProxyService) DeleteAccessRule(ctx context.Context, id int64) (SyncResult, error) {
	if err := p.store.DeleteAccessRule(id); err != nil {
		return SyncResult{}, err
	}
	return p.sync(ctx), nil
}

func isValidIPOrCIDR(s string) bool {
	if _, _, err := net.ParseCIDR(s); err == nil {
		return true
	}
	return net.ParseIP(s) != nil
}

type ImportResult struct {
	Imported int
	Skipped  []caddyfile.SkippedHost
}

// Import adapts the given Caddyfile text via Caddy itself, then conservatively
// translates only the simple "one host, one reverse_proxy" routes into hosts;
// anything else — and any domain that already exists — is reported as skipped,
// never silently dropped.
func (p *ProxyService) Import(ctx context.Context, caddyfileText string) (ImportResult, SyncResult, error) {
	adapted, err := p.caddy.Adapt(ctx, caddyfileText)
	if err != nil {
		return ImportResult{}, SyncResult{}, fmt.Errorf("adapt caddyfile: %w", err)
	}
	hosts, skipped, err := caddyfile.ParseAdapted(adapted)
	if err != nil {
		return ImportResult{}, SyncResult{}, err
	}

	imported := 0
	for _, h := range hosts {
		if _, err := p.store.CreateHost(h.Domain, h.Upstream, "{}", "", true); err != nil {
			skipped = append(skipped, caddyfile.SkippedHost{Domain: h.Domain, Reason: importSkipReason(err)})
			continue
		}
		imported++
	}

	return ImportResult{Imported: imported, Skipped: skipped}, p.sync(ctx), nil
}

func importSkipReason(err error) string {
	if errors.Is(err, store.ErrDuplicateDomain) {
		return "a host for this domain already exists"
	}
	return err.Error()
}

func (p *ProxyService) Export() (string, error) {
	hosts, err := p.store.ListHosts()
	if err != nil {
		return "", err
	}
	return caddyfile.Export(hosts), nil
}

type Status struct {
	CaddyConnected bool
	HostCount      int
}

func (p *ProxyService) Status(ctx context.Context) (Status, error) {
	hosts, err := p.store.ListHosts()
	if err != nil {
		return Status{}, err
	}
	return Status{
		CaddyConnected: p.caddy.Ping(ctx),
		HostCount:      len(hosts),
	}, nil
}
