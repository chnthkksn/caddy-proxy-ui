package main

import (
	"fmt"
	"os"
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
