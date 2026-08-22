package caddyfile

import (
	"strings"

	"caddy-proxy-ui/internal/caddyconfig"
	"caddy-proxy-ui/internal/store"
)

// Export renders hosts as a clean, hand-editable Caddyfile — not a
// reflection of our internal JSON, just simple "domain { reverse_proxy X }"
// blocks in the same shape a human would write by hand.
func Export(hosts []store.Host) string {
	var b strings.Builder
	for i, h := range hosts {
		if i > 0 {
			b.WriteString("\n")
		}
		b.WriteString(h.Domain)
		b.WriteString(" {\n    reverse_proxy ")
		b.WriteString(caddyconfig.NormalizeDial(h.Upstream))
		b.WriteString("\n}\n")
	}
	return b.String()
}
