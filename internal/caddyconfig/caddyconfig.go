// Package caddyconfig builds the minimal Caddy JSON config we actually emit.
// It intentionally does not attempt to model Caddy's full config schema.
package caddyconfig

import (
	"encoding/json"
	"strings"

	"caddy-proxy-ui/internal/store"
)

type Config struct {
	Admin Admin `json:"admin"`
	Apps  Apps  `json:"apps"`
}

// Admin must be repeated on every pushed config: Caddy resets it to its
// built-in default ("localhost:2019") on any /load payload that omits it,
// which would strand caddy-ui — the admin API would no longer be reachable
// from the other container after the very first push.
type Admin struct {
	Listen string `json:"listen"`
}

type Apps struct {
	HTTP HTTPApp `json:"http"`
}

type HTTPApp struct {
	Servers map[string]Server `json:"servers"`
}

type Server struct {
	Listen []string `json:"listen"`
	Routes []Route  `json:"routes"`
}

type Route struct {
	Match  []Match   `json:"match"`
	Handle []Handler `json:"handle"`
}

type Match struct {
	Host []string `json:"host"`
}

// Handler covers the one handler type we emit: reverse_proxy, optionally
// with request headers to set on upstream requests.
type Handler struct {
	Handler   string     `json:"handler"`
	Upstreams []Upstream `json:"upstreams,omitempty"`
	Headers   *Headers   `json:"headers,omitempty"`
}

type Upstream struct {
	Dial string `json:"dial"`
}

type Headers struct {
	Request *HeaderOps `json:"request,omitempty"`
}

type HeaderOps struct {
	Set map[string][]string `json:"set,omitempty"`
}

const serverName = "main"

// Build renders the full Caddy config for every enabled host. Disabled hosts
// are omitted entirely, matching "SQLite is the source of truth" — Caddy only
// ever sees what should currently be live. adminListen is repeated on every
// build (see Admin) so Caddy's admin API stays reachable from the caddy-ui
// container across config reloads.
func Build(hosts []store.Host, adminListen string) Config {
	server := Server{
		Listen: []string{":443", ":80"},
	}

	for _, h := range hosts {
		if !h.Enabled {
			continue
		}
		server.Routes = append(server.Routes, buildRoute(h))
	}

	return Config{
		Admin: Admin{Listen: adminListen},
		Apps: Apps{
			HTTP: HTTPApp{
				Servers: map[string]Server{serverName: server},
			},
		},
	}
}

func buildRoute(h store.Host) Route {
	handler := Handler{
		Handler: "reverse_proxy",
		Upstreams: []Upstream{
			{Dial: NormalizeDial(h.Upstream)},
		},
	}

	if headers := parseRequestHeaders(h.RequestHeaders); len(headers) > 0 {
		handler.Headers = &Headers{Request: &HeaderOps{Set: headers}}
	}

	return Route{
		Match:  []Match{{Host: []string{h.Domain}}},
		Handle: []Handler{handler},
	}
}

// NormalizeDial strips a URL scheme, since the DB stores upstreams as the
// user typed them (e.g. "http://192.168.1.20:3000") but Caddy's "dial"
// field (and a clean Caddyfile export) want a bare "host:port".
func NormalizeDial(upstream string) string {
	upstream = strings.TrimPrefix(upstream, "https://")
	upstream = strings.TrimPrefix(upstream, "http://")
	return strings.TrimSuffix(upstream, "/")
}

// parseRequestHeaders turns the stored {"X-Foo":"bar"} JSON object into the
// []string-valued map Caddy's header.set expects. Invalid/empty JSON yields
// no headers rather than an error — header config is optional.
func parseRequestHeaders(raw string) map[string][]string {
	if raw == "" {
		return nil
	}
	var m map[string]string
	if err := json.Unmarshal([]byte(raw), &m); err != nil {
		return nil
	}
	if len(m) == 0 {
		return nil
	}
	out := make(map[string][]string, len(m))
	for k, v := range m {
		out[k] = []string{v}
	}
	return out
}
