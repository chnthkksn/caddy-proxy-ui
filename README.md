# Caddy Proxy UI

![Release](https://img.shields.io/github/v/release/chnthkksn/caddy-proxy-ui?include_prereleases)
![Build](https://github.com/chnthkksn/caddy-proxy-ui/actions/workflows/release.yml/badge.svg)
![License](https://img.shields.io/github/license/chnthkksn/caddy-proxy-ui)

**A lightweight, native web UI for managing [Caddy](https://caddyserver.com) reverse
proxies on small servers.**

**The goal**: the convenience of tools like Nginx Proxy Manager, without the heavy
management stack (Node backend, Python/Certbot, external DB) that usually comes with it
— small enough to run alongside something else on a 1GB VPS. Not a smaller Caddy; Caddy
is already good at this. Just no extra weight around it.

Add a domain and an upstream in the UI; Caddy handles the reverse proxy, automatic
HTTPS, and certificate renewal — live, with no config file writes and no restarts.
`caddy-ui` is one small Go binary (REST API + SQLite + embedded Svelte frontend, static,
no CGO) that drives Caddy over its Admin API. Caddy itself stays completely standard —
this doesn't wrap or replace it, and you can always drop out to a hand-written Caddyfile.

**Non-goals:** multi-user/RBAC, a metrics/observability platform, replacing the Caddyfile
as a format. This is a UI for the 90% case (`domain → upstream, please`), not a Caddy
config IDE or a full DevOps platform.

Measured idle footprint (native, both processes, dev hardware — not a guarantee for
yours): `caddy-ui` ~19 MB RAM, ~60 MB combined with Caddy, ~0% CPU.

## Features

- Proxy hosts: add/edit/delete/enable/disable, with custom request headers
- Automatic HTTPS (Caddy's default), live reload via the Admin API — no restarts
- Caddyfile import (conservative — anything it can't safely translate is reported as
  *skipped*, never silently dropped) and clean Caddyfile export
- Single-admin login, no default credentials; Caddy connectivity indicator
- Native Linux install (systemd) — Docker is optional, not required
- 🚧 Planned, not yet built: certificates dashboard, access control, log viewer, stats

## Quick start

```bash
curl -fsSL https://raw.githubusercontent.com/chnthkksn/caddy-proxy-ui/main/contrib/install.sh | sudo bash -s -- install
```

Checks ports 80/443/8080 are free before touching anything, installs Caddy if it's
missing, downloads the latest `caddy-ui` binary, sets it up as a systemd service, and
prints the URL to open. Then create the administrator account — there's no default
login. The same script also handles `update`, `reset-password`, `uninstall [--purge]`,
and `status`; run `install.sh help` for the full list, or read it before running it —
it's a plain shell script.

Prefer Docker? That's supported too:

```bash
git clone https://github.com/chnthkksn/caddy-proxy-ui.git && cd caddy-proxy-ui
docker compose up -d
```

### Configuration

| Variable | Native default | Docker Compose value |
|---|---|---|
| `CADDY_ADMIN_URL` | `http://localhost:2019` | `http://caddy:2019` |
| `CADDY_ADMIN_LISTEN` | `127.0.0.1:2019` | `0.0.0.0:2019` |
| `DB_PATH` | `/var/lib/caddy-ui/caddy-ui.db` | `/data/caddy-ui.db` |
| `LISTEN_ADDR` | `:8080` | `:8080` |
| `COOKIE_SECURE` | `false` | `false` |

## How it works

SQLite is the source of truth. Every mutation writes to SQLite first, then rebuilds the
full Caddy config and pushes it via `POST /load`. If Caddy is briefly unreachable, the
write still succeeds — the UI shows a "Caddy offline" banner and reconciles once it's
back, instead of failing the request. Caddy always runs as its own separate process (or
container); if `caddy-ui` disappears, Caddy just keeps running with the last config
pushed to it.

## Contributing

Issues and PRs welcome. A few ground rules:

- **Keep it tiny** — stdlib `net/http`, no router/framework. Check the standard library
  covers something before adding a dependency.
- **SQLite stays the source of truth** — host mutations go through
  `internal/service.ProxyService`, the only place allowed to touch both the DB and
  Caddy's Admin API.
- **Import never silently drops config** — anything the Caddyfile importer can't
  confidently translate should come back as a reported "skipped" item.
- **Caddy stays a separate process** — embedding it as a library was considered and
  rejected (see "How it works").

Local dev: `go run ./cmd/caddy-ui`, and in `web/`, `npm install && npm run dev`.
`npm run check` and `go vet ./...` should both stay clean.

## License

[MIT](LICENSE)
