package caddyfile

import (
	"encoding/json"
	"fmt"
)

// ImportedHost is a proxy host successfully recognized during import.
type ImportedHost struct {
	Domain   string
	Upstream string
}

// SkippedHost records a route the importer deliberately did not translate,
// with a human-readable reason — import must never silently drop config.
type SkippedHost struct {
	Domain string
	Reason string
}

type adaptedTop struct {
	Apps struct {
		HTTP struct {
			Servers map[string]struct {
				Routes []adaptedRoute `json:"routes"`
			} `json:"servers"`
		} `json:"http"`
	} `json:"apps"`
}

type adaptedRoute struct {
	Match  []map[string]json.RawMessage `json:"match"`
	Handle []map[string]json.RawMessage `json:"handle"`
}

// ParseAdapted walks Caddy's adapted JSON (the output of the Admin API's
// /adapt endpoint) and extracts only the simple "one host, one reverse_proxy,
// one upstream" shape. Anything else — path/header matchers, multiple
// upstreams, extra directives, nested handle blocks — is reported as
// skipped rather than guessed at.
func ParseAdapted(raw json.RawMessage) (imported []ImportedHost, skipped []SkippedHost, err error) {
	var top adaptedTop
	if err := json.Unmarshal(raw, &top); err != nil {
		return nil, nil, fmt.Errorf("decode adapted config: %w", err)
	}

	for _, server := range top.Apps.HTTP.Servers {
		for _, route := range server.Routes {
			domain, ok := extractDomain(route.Match)
			if !ok {
				skipped = append(skipped, SkippedHost{Domain: "(unknown)", Reason: "route does not match exactly one host"})
				continue
			}
			upstream, reason := extractUpstream(route.Handle)
			if reason != "" {
				skipped = append(skipped, SkippedHost{Domain: domain, Reason: reason})
				continue
			}
			imported = append(imported, ImportedHost{Domain: domain, Upstream: upstream})
		}
	}
	return imported, skipped, nil
}

func extractDomain(match []map[string]json.RawMessage) (string, bool) {
	if len(match) != 1 {
		return "", false
	}
	m := match[0]
	if len(m) != 1 {
		return "", false
	}
	hostRaw, ok := m["host"]
	if !ok {
		return "", false
	}
	var hosts []string
	if err := json.Unmarshal(hostRaw, &hosts); err != nil || len(hosts) != 1 {
		return "", false
	}
	return hosts[0], true
}

func extractUpstream(handle []map[string]json.RawMessage) (upstream string, skipReason string) {
	if len(handle) != 1 {
		return "", "multiple top-level handlers"
	}

	handlerType, ok := decodeHandlerType(handle[0])
	if !ok {
		return "", "malformed handler"
	}
	if handlerType != "subroute" {
		return "", fmt.Sprintf("unsupported handler %q", handlerType)
	}

	routesRaw, ok := handle[0]["routes"]
	if !ok {
		return "", "empty site block"
	}
	var nested []adaptedRoute
	if err := json.Unmarshal(routesRaw, &nested); err != nil {
		return "", "unrecognized site block structure"
	}
	if len(nested) != 1 {
		return "", "multiple routes (path-based handle blocks)"
	}
	if len(nested[0].Match) != 0 {
		return "", "path or header matched routes"
	}
	if len(nested[0].Handle) != 1 {
		return "", "multiple directives"
	}

	h := nested[0].Handle[0]
	handlerType, ok = decodeHandlerType(h)
	if !ok {
		return "", "malformed directive"
	}
	if handlerType != "reverse_proxy" {
		return "", fmt.Sprintf("unsupported directive %q", handlerType)
	}
	for k := range h {
		if k != "handler" && k != "upstreams" {
			return "", "reverse_proxy has advanced options not supported by import"
		}
	}

	upstreamsRaw, ok := h["upstreams"]
	if !ok {
		return "", "reverse_proxy has no upstreams"
	}
	var upstreams []struct {
		Dial string `json:"dial"`
	}
	if err := json.Unmarshal(upstreamsRaw, &upstreams); err != nil || len(upstreams) != 1 {
		return "", "multiple upstreams not supported by import"
	}
	return upstreams[0].Dial, ""
}

func decodeHandlerType(m map[string]json.RawMessage) (string, bool) {
	raw, ok := m["handler"]
	if !ok {
		return "", false
	}
	var s string
	if err := json.Unmarshal(raw, &s); err != nil {
		return "", false
	}
	return s, true
}
