package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"caddy-proxy-ui/internal/api"
	"caddy-proxy-ui/internal/caddyclient"
	"caddy-proxy-ui/internal/service"
	"caddy-proxy-ui/internal/store"
	"caddy-proxy-ui/internal/webui"
)

// version is set at build time via -ldflags "-X main.version=...";
// see .github/workflows/release.yml.
var version = "dev"

func main() {
	dbPath := envOr("DB_PATH", "./data/caddy-ui.db")
	caddyAdminURL := envOr("CADDY_ADMIN_URL", "http://caddy:2019")
	// Must match what Caddy's own bootstrap config binds admin to (see
	// Caddyfile.bootstrap) and gets re-asserted on every pushed config —
	// see the comment on caddyconfig.Admin for why.
	caddyAdminListen := envOr("CADDY_ADMIN_LISTEN", "0.0.0.0:2019")
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	cookieSecure := envOr("COOKIE_SECURE", "false") == "true"

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	caddy := caddyclient.New(caddyAdminURL)
	proxy := service.New(st, caddy, caddyAdminListen)
	auth := service.NewAuthService(st)

	// Best-effort initial push so a caddy-ui restart re-applies existing
	// hosts to Caddy even if Caddy started with an empty bootstrap config.
	if result := proxy.Sync(context.Background()); !result.Synced {
		log.Printf("initial sync to caddy failed (will retry via /api/sync or the next mutation): %s", result.Error)
	}

	apiServer := api.NewServer(proxy, auth, cookieSecure, version)
	spaHandler, err := webui.Handler()
	if err != nil {
		log.Fatalf("build web ui handler: %v", err)
	}

	mux := http.NewServeMux()
	mux.Handle("/api/", apiServer.Routes())
	mux.Handle("/", spaHandler)

	srv := &http.Server{
		Addr:              listenAddr,
		Handler:           mux,
		ReadHeaderTimeout: 5 * time.Second,
	}

	go func() {
		log.Printf("caddy-ui %s listening on %s (caddy admin: %s)", version, listenAddr, caddyAdminURL)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("serve: %v", err)
		}
	}()

	stop := make(chan os.Signal, 1)
	signal.Notify(stop, syscall.SIGINT, syscall.SIGTERM)
	<-stop

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		log.Printf("graceful shutdown failed: %v", err)
	}
}

func envOr(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}
