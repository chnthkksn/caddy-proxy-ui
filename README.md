# Caddy Proxy UI

![Release](https://img.shields.io/github/v/release/chnthkksn/caddy-proxy-ui?include_prereleases)
![Build](https://github.com/chnthkksn/caddy-proxy-ui/actions/workflows/release.yml/badge.svg)
![License](https://img.shields.io/github/license/chnthkksn/caddy-proxy-ui)

**A point-and-click UX for [Caddy](https://caddyserver.com), at a fraction of the usual footprint.**

A small self-hosted UI for managing reverse proxy hosts backed by Caddy. Point it at
your Caddy container's Admin API, add a domain and an upstream, and Caddy handles the
reverse proxy, automatic HTTPS, and certificate renewal — live, with no config file
writes and no restarts.

```
Browser ──▶ Caddy Proxy UI (Go binary: REST API + SQLite + embedded Svelte SPA)
                    │
                    │ Admin API (POST /load)
                    ▼
                  Caddy  ──▶ :80 / :443 ──▶ your upstreams
```

## Why this exists

Reverse-proxying with TLS shouldn't require hand-editing config files, but most tools
that solve that drag along a heavy stack (a separate backend, a database server, a
Node runtime) to do it. Caddy already does automatic HTTPS and reverse proxying well,
out of the box, with zero extra services — it just doesn't ship a UI.

This project is **one job**: a beautiful, minimal UI for managing Caddy as a reverse
proxy. Caddy remains completely standard — this doesn't wrap or replace it, it drives it
over the Admin API. You can always drop out to a hand-written Caddyfile; nothing here is
a lock-in.

It deliberately does **not** try to become a full DevOps platform: no built-in metrics
dashboard, no RBAC, no Prometheus/Grafana. If you need that, put something else in front
of Caddy — this just makes the common case (`domain → upstream, please`) fast.

## Features

| | |
|---|---|
| ✅ | Proxy hosts: add/edit/delete/enable/disable |
| ✅ | Automatic HTTPS (Caddy's default — no config needed) |
| ✅ | Live reload via Caddy's Admin API — no restarts |
| ✅ | Custom request headers per host |
| ✅ | Caddyfile import (conservative — anything it can't safely translate is reported as *skipped*, never silently dropped) |
| ✅ | Caddyfile export — a clean, hand-editable file, not a JSON dump |
| ✅ | Single-admin login, no default credentials |
| ✅ | Caddy connectivity indicator — mutations still save if Caddy is briefly offline |
| 🚧 | Certificates dashboard, access control, live log viewer — planned, not yet built |

**Non-goals:** multi-user/RBAC, a metrics/observability platform, or replacing the
Caddyfile as a format. This is a UI for the 90% case, not a Caddy config IDE.

## Quick start (Docker Compose)

This is the intended way to run it: two containers, Caddy stays completely standard.

```bash
git clone https://github.com/chnthkksn/caddy-proxy-ui.git
cd caddy-proxy-ui
docker compose up -d
```

Then open `http://<your-server>:8080`, create the administrator account (there's no
default login), and add your first proxy host. Caddy listens on `:80`/`:443` as usual —
point your domains' DNS at this server and Caddy will provision certificates
automatically the first time each domain is actually requested.

The compose file already handles the one non-obvious wiring detail: Caddy's Admin API
(port `2019`) is reachable from the `caddy-ui` container over the internal Docker
network, but is **never** published to the host.

### Configuration

Set these as environment variables on the `caddy-ui` service in `docker-compose.yml`:

| Variable | Default | Purpose |
|---|---|---|
| `CADDY_ADMIN_URL` | `http://caddy:2019` | Where caddy-ui reaches Caddy's Admin API |
| `CADDY_ADMIN_LISTEN` | `0.0.0.0:2019` | Re-asserted on every pushed config — must match what Caddy itself binds admin to |
| `DB_PATH` | `/data/caddy-ui.db` | SQLite database path |
| `LISTEN_ADDR` | `:8080` | Where the UI/API itself listens |
| `COOKIE_SECURE` | `false` | Set `true` once caddy-ui is served over TLS (e.g. behind another proxy) |

## Running without Docker

Each [release](https://github.com/chnthkksn/caddy-proxy-ui/releases) publishes a static
`caddy-ui` binary for `linux/amd64`, `linux/arm64`, and `linux/armv7` — no runtime
dependencies, no CGO.

```bash
# 1. Run Caddy yourself, with its admin API reachable (see Caddyfile.bootstrap):
caddy run --config Caddyfile.bootstrap

# 2. Download and run caddy-ui:
curl -LO https://github.com/chnthkksn/caddy-proxy-ui/releases/latest/download/caddy-ui_<version>_linux_amd64.tar.gz
tar xzf caddy-ui_<version>_linux_amd64.tar.gz
CADDY_ADMIN_URL=http://localhost:2019 ./caddy-ui
```

Verify a binary with `checksums.txt` from the same release:

```bash
sha256sum -c checksums.txt --ignore-missing
```

## How it works

SQLite is the source of truth; Caddy's running config is just a disposable rebuild of
whatever is currently in the database. Every mutation (create/edit/delete/toggle host)
writes to SQLite first, then rebuilds the full Caddy config and pushes it via
`POST /load`. If Caddy is briefly unreachable, the database write still succeeds — the
UI shows a "Caddy offline, will sync automatically" banner instead of failing the
request outright, and reconciles on the next successful health check or a manual sync.

```
SQLite ──▶ Build() minimal JSON config ──▶ POST /load ──▶ Caddy applies it, zero-downtime
```

## Contributing

Issues and PRs are welcome. A few things that'll make a PR easy to merge:

- **Keep it tiny.** The whole point of this project is staying small and fast. The
  backend is stdlib `net/http` on purpose — no router, no framework. Before adding a
  dependency (Go or npm), ask whether the standard library already covers it.
- **SQLite stays the source of truth.** Any change that touches proxy hosts should go
  through `internal/service.ProxyService` — it's the one place allowed to talk to both
  the database and Caddy's Admin API.
- **Import must never silently drop config.** If you extend the Caddyfile importer,
  anything it can't confidently translate should come back as a reported "skipped" item,
  not a best-effort guess.

### Local development

```bash
# Backend
go run ./cmd/caddy-ui

# Frontend (separate terminal — proxies /api to :8080 via vite.config.js)
cd web
npm install
npm run dev
```

`npm run check` runs `svelte-check` (TypeScript + Svelte diagnostics) and `go vet ./...`
covers the backend. Both run clean on `main`.

### Project layout

```
cmd/caddy-ui/       entrypoint, wiring
internal/api/        HTTP handlers, session middleware
internal/service/     ProxyService + AuthService — the only DB+Caddy touchpoints
internal/store/        SQLite access
internal/caddyconfig/   builds the minimal Caddy JSON config we emit
internal/caddyclient/   talks to Caddy's Admin API (/load, /adapt)
internal/caddyfile/     Caddyfile export + conservative import parser
internal/webui/         serves the embedded Svelte build
web/                    Svelte + TypeScript frontend (Vite)
```

## License

[MIT](LICENSE)
