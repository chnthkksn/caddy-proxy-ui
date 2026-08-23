# Caddy Proxy UI

![Release](https://img.shields.io/github/v/release/chnthkksn/caddy-proxy-ui?include_prereleases)
![Build](https://github.com/chnthkksn/caddy-proxy-ui/actions/workflows/release.yml/badge.svg)
![License](https://img.shields.io/github/license/chnthkksn/caddy-proxy-ui)

**A lightweight, native web UI for managing [Caddy](https://caddyserver.com) reverse
proxies on small servers.**

Point it at Caddy's Admin API, add a domain and an upstream, and Caddy handles the
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

Reverse-proxying with TLS shouldn't require hand-editing config files. Tools that solve
that problem well already exist, but they typically bring a whole management stack along
for the ride — a separate backend runtime, a database server, a certificate-management
subprocess. On a 1GB VPS that's already running something else, that stack can be the
difference between "fits" and "doesn't."

The goal here isn't a smaller Caddy — Caddy already does automatic HTTPS and reverse
proxying well, out of the box, with zero extra services. The goal is to **not add a heavy
management stack around it**. `caddy-ui` is one small Go binary: a REST API, SQLite, and
an embedded Svelte frontend, all statically compiled — no Node runtime in production, no
Redis, no Postgres/MySQL, no Python/Certbot subprocess. Caddy remains completely
standard; this drives it over its Admin API rather than wrapping or replacing it, and you
can always drop out to a hand-written Caddyfile — nothing here is a lock-in.

It deliberately does **not** try to become a full DevOps platform: no built-in metrics
dashboard, no RBAC, no Prometheus/Grafana. If you need that, put something else in front
of Caddy — this just makes the common case (`domain → upstream, please`) fast, on
hardware where that needs to actually matter.

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
| ✅ | Native Linux install (systemd) — Docker is optional, not required |
| 🚧 | Certificates dashboard, access control, live log viewer, stats — planned, not yet built |

**Non-goals:** multi-user/RBAC, a metrics/observability platform, or replacing the
Caddyfile as a format. This is a UI for the 90% case, not a Caddy config IDE.

### Resource footprint

Engineering targets, not marketing claims — the "measured" column is real, not
aspirational:

| | Target | Measured¹ |
|---|---:|---:|
| `caddy-ui` idle RAM | < 30 MB | **19 MB** |
| `caddy-ui` idle CPU | ~0% | **0%** |
| `caddy-ui` + Caddy combined | < 100 MB | **~60 MB** |
| External database | none | none |
| Runtime dependencies | none | none (static binary, `CGO_ENABLED=0`) |

¹ Measured on a macOS/arm64 dev machine, both processes idle with no hosts configured,
running natively (not in Docker) — not the actual 1GB Linux VPS this is meant for.
Real-world numbers on Linux will differ somewhat; treat this as a rough sanity check on
the target column, not a guarantee for your hardware.

## Quick start: native install (recommended for small VPSs)

This is the primary way to run it — two plain OS processes, no Docker required.

**1. Install Caddy** via its [official instructions](https://caddyserver.com/docs/install)
(the apt/dnf repo installs a working `caddy.service` with the right capabilities for
ports 80/443 already set up). Caddy's admin API already defaults to `localhost:2019` —
no Caddyfile edits needed.

**2. Install `caddy-ui`** from the [latest release](https://github.com/chnthkksn/caddy-proxy-ui/releases):

```bash
curl -LO https://github.com/chnthkksn/caddy-proxy-ui/releases/latest/download/caddy-ui_linux_amd64.tar.gz
tar xzf caddy-ui_linux_amd64.tar.gz
sudo mv caddy-ui /usr/local/bin/caddy-ui
```

(Swap `linux_amd64` for `linux_arm64` or `linux_armv7` if that matches your server. This
always fetches whatever's currently latest — release filenames are deliberately
unversioned so this URL never needs updating.)

Verify the download against `checksums.txt` from the same release:

```bash
curl -LO https://github.com/chnthkksn/caddy-proxy-ui/releases/latest/download/checksums.txt
sha256sum -c checksums.txt --ignore-missing
```

**3. Install the systemd unit** and start it:

```bash
sudo cp contrib/systemd/caddy-ui.service /etc/systemd/system/
sudo systemctl daemon-reload
sudo systemctl enable --now caddy-ui
```

Then open `http://<your-server>:8080`, create the administrator account (there's no
default login), and add your first proxy host.

`caddy-ui`'s defaults already assume this setup: `CADDY_ADMIN_URL=http://localhost:2019`
and `CADDY_ADMIN_LISTEN=127.0.0.1:2019` — since both processes share the host instead of
separate Docker network namespaces, the admin API never needs to bind anything but
loopback, and it's never reachable off the box.

## Quick start: Docker Compose

If you already run Docker and prefer that, it's fully supported — two containers, Caddy
stays completely standard:

```bash
git clone https://github.com/chnthkksn/caddy-proxy-ui.git
cd caddy-proxy-ui
docker compose up -d
```

Then open `http://<your-server>:8080` and set up the administrator account, same as
above. The compose file overrides the admin API address to `0.0.0.0:2019` (needed so the
`caddy-ui` container can reach the `caddy` container) but never publishes port `2019` to
the host.

### Configuration

Environment variables `caddy-ui` reads (set on the systemd unit, or on the `caddy-ui`
service in `docker-compose.yml`):

| Variable | Native default | Docker Compose value | Purpose |
|---|---|---|---|
| `CADDY_ADMIN_URL` | `http://localhost:2019` | `http://caddy:2019` | Where caddy-ui reaches Caddy's Admin API |
| `CADDY_ADMIN_LISTEN` | `127.0.0.1:2019` | `0.0.0.0:2019` | Re-asserted on every pushed config — must match what Caddy itself binds admin to |
| `DB_PATH` | `/var/lib/caddy-ui/caddy-ui.db` | `/data/caddy-ui.db` | SQLite database path |
| `LISTEN_ADDR` | `:8080` | `:8080` | Where the UI/API itself listens |
| `COOKIE_SECURE` | `false` | `false` | Set `true` once caddy-ui is served over TLS (e.g. behind another proxy) |

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

Caddy always runs as its own separate, completely standard process (or container) —
`caddy-ui` only ever talks to it over the Admin API. This is a deliberate choice: Caddy
stays independently upgradable and debuggable, and if `caddy-ui` is ever removed, Caddy
just keeps running with whatever config was last pushed.

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
- **Caddy stays a separate process.** Don't embed Caddy as a library into `caddy-ui` —
  it's been considered and deliberately rejected, see "How it works" above.

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
cmd/caddy-ui/          entrypoint, wiring
internal/api/            HTTP handlers, session middleware
internal/service/         ProxyService + AuthService — the only DB+Caddy touchpoints
internal/store/             SQLite access
internal/caddyconfig/        builds the minimal Caddy JSON config we emit
internal/caddyclient/         talks to Caddy's Admin API (/load, /adapt)
internal/caddyfile/            Caddyfile export + conservative import parser
internal/webui/                 serves the embedded Svelte build
web/                            Svelte + TypeScript frontend (Vite)
contrib/systemd/                  example systemd unit for native installs
```

## License

[MIT](LICENSE)
