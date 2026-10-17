package main

import (
	"fmt"
	"os"
	"strconv"
)

// version is set at build time via -ldflags "-X main.version=...";
// see .github/workflows/release.yml.
var version = "dev"

func main() {
	cmd := "serve"
	if len(os.Args) > 1 {
		cmd = os.Args[1]
	}

	switch cmd {
	case "serve":
		runServe()
	case "reset-password":
		runResetPassword()
	case "version":
		fmt.Println(version)
	default:
		fmt.Fprintf(os.Stderr, "caddy-ui: unknown command %q\nusage: caddy-ui [serve|reset-password|version]\n", cmd)
		os.Exit(1)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// envIntOr reads a positive integer setting. A value that isn't one is
// reported and ignored rather than silently treated as zero, which for a
// size limit would mean something very different from what was intended.
func envIntOr(key string, fallback int) int {
	v := os.Getenv(key)
	if v == "" {
		return fallback
	}
	n, err := strconv.Atoi(v)
	if err != nil || n < 1 {
		fmt.Fprintf(os.Stderr, "caddy-ui: ignoring %s=%q (want a positive whole number), using %d\n", key, v, fallback)
		return fallback
	}
	return n
}
