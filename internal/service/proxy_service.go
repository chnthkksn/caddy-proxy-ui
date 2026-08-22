// Package service holds ProxyService, the only place that touches both
// store.Store and caddyclient.Client. SQLite is the source of truth; every
// mutation writes to it first, then rebuilds and pushes the full config to
// Caddy. A failed push never fails the mutation — Caddy is allowed to be
// temporarily offline, and Sync exists to reconcile once it's back.
package service

import (
	"context"
	"errors"
	"fmt"

	"caddy-proxy-ui/internal/caddyclient"
	"caddy-proxy-ui/internal/caddyconfig"
	"caddy-proxy-ui/internal/caddyfile"
	"caddy-proxy-ui/internal/store"
)

type SyncResult struct {
	Synced bool
	Error  string
}

type ProxyService struct {
	store       *store.Store
	caddy       *caddyclient.Client
	adminListen string
}

func New(s *store.Store, c *caddyclient.Client, adminListen string) *ProxyService {
	return &ProxyService{store: s, caddy: c, adminListen: adminListen}
}

func (p *ProxyService) sync(ctx context.Context) SyncResult {
	hosts, err := p.store.ListHosts()
	if err != nil {
		return SyncResult{Error: err.Error()}
	}
	cfg := caddyconfig.Build(hosts, p.adminListen)
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

func (p *ProxyService) CreateHost(ctx context.Context, domain, upstream, requestHeaders string, enabled bool) (store.Host, SyncResult, error) {
	h, err := p.store.CreateHost(domain, upstream, requestHeaders, enabled)
	if err != nil {
		return store.Host{}, SyncResult{}, err
	}
	return h, p.sync(ctx), nil
}

func (p *ProxyService) UpdateHost(ctx context.Context, id int64, domain, upstream, requestHeaders string, enabled bool) (store.Host, SyncResult, error) {
	h, err := p.store.UpdateHost(id, domain, upstream, requestHeaders, enabled)
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
		if _, err := p.store.CreateHost(h.Domain, h.Upstream, "{}", true); err != nil {
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
