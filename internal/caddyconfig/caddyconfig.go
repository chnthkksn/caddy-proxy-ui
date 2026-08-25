// Package caddyconfig builds the minimal Caddy JSON config we actually emit.
// It intentionally does not attempt to model Caddy's full config schema.
package caddyconfig

import (
	"encoding/json"
	"strings"

	"caddy-proxy-ui/internal/store"
)

type Config struct {
	Admin   Admin   `json:"admin"`
	Logging Logging `json:"logging,omitempty"`
	Apps    Apps    `json:"apps"`
}

// Logging routes Caddy's own access logs to a file caddy-ui also reads (see
// internal/trafficlog), under a dedicated named logger — never "default",
// which would also swallow Caddy's own startup/system logs into the same
// file. Shape confirmed via a real /load against the admin API, not guessed.
type Logging struct {
	Logs map[string]LogConfig `json:"logs,omitempty"`
}

type LogConfig struct {
	Writer  LogWriter  `json:"writer"`
	Encoder LogEncoder `json:"encoder"`
	Include []string   `json:"include,omitempty"`
}

type LogWriter struct {
	Output   string `json:"output"`
	Filename string `json:"filename"`
}

type LogEncoder struct {
	Format string `json:"format"`
}

const accessLoggerName = "access"

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
	Listen []string    `json:"listen"`
	Routes []Route     `json:"routes"`
	Logs   *ServerLogs `json:"logs,omitempty"`
}

// ServerLogs maps each host to the logger it should write access log
// entries through. An empty-string logger name means "default"; we always
// point every host at the dedicated "access" logger instead (see Logging).
type ServerLogs struct {
	LoggerNames map[string][]string `json:"logger_names,omitempty"`
}

type Route struct {
	Match  []Match   `json:"match,omitempty"`
	Handle []Handler `json:"handle"`
}

// Match covers host matching plus the two access-control matchers
// (remote_ip, and its "not" negation for allow-lists) — confirmed against
// real `caddy adapt` output, not guessed.
type Match struct {
	Host     []string       `json:"host,omitempty"`
	RemoteIP *RemoteIPMatch `json:"remote_ip,omitempty"`
	Not      []Match        `json:"not,omitempty"`
}

type RemoteIPMatch struct {
	Ranges []string `json:"ranges"`
}

// Handler covers every handler type we emit: reverse_proxy (with optional
// request headers), subroute (nested ordered routes — how a host's blocking
// access rules run before its proxy handler), authentication (basic auth),
// and static_response (used for access-denied responses).
type Handler struct {
	Handler    string         `json:"handler"`
	Upstreams  []Upstream     `json:"upstreams,omitempty"`
	Headers    *Headers       `json:"headers,omitempty"`
	Routes     []Route        `json:"routes,omitempty"`      // subroute
	Providers  *AuthProviders `json:"providers,omitempty"`   // authentication
	StatusCode int            `json:"status_code,omitempty"` // static_response
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

type AuthProviders struct {
	HTTPBasic *HTTPBasicProvider `json:"http_basic"`
}

type HTTPBasicProvider struct {
	Accounts []BasicAuthAccount `json:"accounts"`
	Hash     HashConfig         `json:"hash"`
}

type BasicAuthAccount struct {
	Username string `json:"username"`
	Password string `json:"password"` // a bcrypt hash, same format bcrypt.GenerateFromPassword produces
}

type HashConfig struct {
	Algorithm string `json:"algorithm"`
}

const serverName = "main"

// Build renders the full Caddy config for every enabled host. Disabled hosts
// are omitted entirely, matching "SQLite is the source of truth" — Caddy only
// ever sees what should currently be live. adminListen is repeated on every
// build (see Admin) so Caddy's admin API stays reachable from the caddy-ui
// container across config reloads. accessRules is keyed by host ID (see
// store.ListAccessRulesForHosts) — pass nil/empty if none apply. accessLogPath
// is where Caddy writes structured access logs for every enabled host; pass
// "" to omit logging entirely.
func Build(hosts []store.Host, adminListen string, accessRules map[int64][]store.AccessRule, accessLogPath string) Config {
	server := Server{
		Listen: []string{":443", ":80"},
	}

	loggerNames := make(map[string][]string)
	for _, h := range hosts {
		if !h.Enabled {
			continue
		}
		server.Routes = append(server.Routes, buildRoute(h, accessRules[h.ID]))
		if accessLogPath != "" {
			loggerNames[h.Domain] = []string{accessLoggerName}
		}
	}
	if len(loggerNames) > 0 {
		server.Logs = &ServerLogs{LoggerNames: loggerNames}
	}

	cfg := Config{
		Admin: Admin{Listen: adminListen},
		Apps: Apps{
			HTTP: HTTPApp{
				Servers: map[string]Server{serverName: server},
			},
		},
	}
	if accessLogPath != "" {
		cfg.Logging = Logging{
			Logs: map[string]LogConfig{
				accessLoggerName: {
					Writer:  LogWriter{Output: "file", Filename: accessLogPath},
					Encoder: LogEncoder{Format: "json"},
					Include: []string{"http.log.access"},
				},
			},
		}
	}
	return cfg
}

func buildRoute(h store.Host, rules []store.AccessRule) Route {
	proxyHandler := Handler{
		Handler: "reverse_proxy",
		Upstreams: []Upstream{
			{Dial: NormalizeDial(h.Upstream)},
		},
	}
	if headers := parseRequestHeaders(h.RequestHeaders); len(headers) > 0 {
		proxyHandler.Headers = &Headers{Request: &HeaderOps{Set: headers}}
	}

	if len(rules) == 0 {
		// No access rules: keep the plain flat shape (no subroute wrapper)
		// — simplest possible config for the common case.
		return Route{
			Match:  []Match{{Host: []string{h.Domain}}},
			Handle: []Handler{proxyHandler},
		}
	}

	var denyRanges, allowRanges []string
	var accounts []BasicAuthAccount
	for _, r := range rules {
		switch r.Kind {
		case store.AccessRuleIPDeny:
			denyRanges = append(denyRanges, r.Value)
		case store.AccessRuleIPAllow:
			allowRanges = append(allowRanges, r.Value)
		case store.AccessRuleBasicAuth:
			if account, ok := parseBasicAuthValue(r.Value); ok {
				accounts = append(accounts, account)
			}
		}
	}

	// Blocking rules run as their own routes, before the serving route, in
	// the same subroute — a route that matches wins and never falls through
	// to the ones after it (confirmed via `caddy adapt`).
	var subroutes []Route
	if len(denyRanges) > 0 {
		subroutes = append(subroutes, Route{
			Match:  []Match{{RemoteIP: &RemoteIPMatch{Ranges: denyRanges}}},
			Handle: []Handler{{Handler: "static_response", StatusCode: 403}},
		})
	}
	if len(allowRanges) > 0 {
		subroutes = append(subroutes, Route{
			Match:  []Match{{Not: []Match{{RemoteIP: &RemoteIPMatch{Ranges: allowRanges}}}}},
			Handle: []Handler{{Handler: "static_response", StatusCode: 403}},
		})
	}

	serveHandlers := make([]Handler, 0, 2)
	if len(accounts) > 0 {
		serveHandlers = append(serveHandlers, Handler{
			Handler: "authentication",
			Providers: &AuthProviders{
				HTTPBasic: &HTTPBasicProvider{
					Accounts: accounts,
					Hash:     HashConfig{Algorithm: "bcrypt"},
				},
			},
		})
	}
	serveHandlers = append(serveHandlers, proxyHandler)
	subroutes = append(subroutes, Route{Handle: serveHandlers})

	return Route{
		Match:  []Match{{Host: []string{h.Domain}}},
		Handle: []Handler{{Handler: "subroute", Routes: subroutes}},
	}
}

// parseBasicAuthValue decodes a host_access_rules.value JSON blob
// ({"username","bcrypt_hash"}) into the shape Caddy's authentication
// handler expects. Malformed rows are skipped rather than erroring the
// whole config build — a bad row shouldn't take down every other host.
func parseBasicAuthValue(raw string) (BasicAuthAccount, bool) {
	var v struct {
		Username   string `json:"username"`
		BcryptHash string `json:"bcrypt_hash"`
	}
	if err := json.Unmarshal([]byte(raw), &v); err != nil || v.Username == "" || v.BcryptHash == "" {
		return BasicAuthAccount{}, false
	}
	return BasicAuthAccount{Username: v.Username, Password: v.BcryptHash}, true
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
