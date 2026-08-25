package main

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"caddy-proxy-ui/internal/api"
	"caddy-proxy-ui/internal/caddyclient"
	"caddy-proxy-ui/internal/service"
	"caddy-proxy-ui/internal/store"
	"caddy-proxy-ui/internal/webui"
)

func runServe() {
	dbPath := envOr("DB_PATH", "./data/caddy-ui.db")
	// Defaults assume the primary deployment: caddy-ui and Caddy as two plain
	// processes on the same host (see contrib/systemd/caddy-ui.service), so
	// the admin API only ever needs loopback — never 0.0.0.0. Docker Compose
	// overrides both of these explicitly since it needs the cross-container
	// hop instead.
	caddyAdminURL := envOr("CADDY_ADMIN_URL", "http://localhost:2019")
	// Re-asserted on every pushed config — see the comment on
	// caddyconfig.Admin for why that's necessary regardless of deployment.
	caddyAdminListen := envOr("CADDY_ADMIN_LISTEN", "127.0.0.1:2019")
	listenAddr := envOr("LISTEN_ADDR", ":8080")
	cookieSecure := envOr("COOKIE_SECURE", "false") == "true"
	accessLogPath := envOr("ACCESS_LOG_PATH", "./data/access.log")
	// Native/systemd default (apt/dnf's Caddy package sets no XDG_DATA_HOME,
	// so certmagic falls back to this path); Docker Compose overrides it to
	// the read-only-mounted caddy_data volume's path instead.
	certStoragePath := envOr("CADDY_STORAGE_PATH", "/var/lib/caddy/.local/share/caddy")

	st, err := store.Open(dbPath)
	if err != nil {
		log.Fatalf("open store: %v", err)
	}
	defer st.Close()

	if accessLogPath != "" {
		if err := os.MkdirAll(filepath.Dir(accessLogPath), 0o755); err != nil {
			log.Fatalf("create access log dir: %v", err)
		}
	}

	caddy := caddyclient.New(caddyAdminURL)
	proxy := service.New(st, caddy, caddyAdminListen, accessLogPath, certStoragePath)
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
