package caddyconfig

import (
	"encoding/json"
	"strings"
	"testing"
)

func TestBuildDefaultPorts(t *testing.T) {
	cfg := Build(nil, "127.0.0.1:2019", nil, AccessLog{}, Ports{})
	srv := cfg.Apps.HTTP.Servers[serverName]
	if got := strings.Join(srv.Listen, ","); got != ":443,:80" {
		t.Errorf("listen = %s, want :443,:80", got)
	}
	b, _ := json.Marshal(cfg)
	for _, unwanted := range []string{"https_port", "http_port", "tls-alpn"} {
		if strings.Contains(string(b), unwanted) {
			t.Errorf("default config should not mention %s: %s", unwanted, b)
		}
	}
}

func TestBuildCustomHTTPSPort(t *testing.T) {
	cfg := Build(nil, "127.0.0.1:2019", nil, AccessLog{}, Ports{HTTP: 80, HTTPS: 8443})
	srv := cfg.Apps.HTTP.Servers[serverName]
	if got := strings.Join(srv.Listen, ","); got != ":8443,:80" {
		t.Errorf("listen = %s, want :8443,:80", got)
	}
	if cfg.Apps.HTTP.HTTPSPort != 8443 {
		t.Errorf("https_port = %d, want 8443 so redirects target it", cfg.Apps.HTTP.HTTPSPort)
	}
	if cfg.Apps.TLS == nil {
		t.Fatal("TLS-ALPN must be disabled when 443 isn't ours")
	}
	b, _ := json.Marshal(cfg.Apps.TLS)
	if !strings.Contains(string(b), `"tls-alpn":{"disabled":true}`) {
		t.Errorf("tls policy = %s", b)
	}
}
