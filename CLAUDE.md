# CLAUDE.md

This file provides guidance to Claude Code (claude.ai/code) when working with code in this repository.

## Project overview

**AWG Hop** — admin panel for an AmneziaWG (AWG) VPN server. The server acts as an AWG ingress for clients and simultaneously as an AWG client of one or more remote upstream servers. Each client routes either directly to the internet or through a selected upstream tunnel.

Stack: **Go 1.22** backend, **Svelte 5 + Vite 6** frontend (compiled to `web/dist`, embedded in the binary), **SQLite** (modernc.org/sqlite — pure Go, no CGo), Docker for deployment.

## Commands

### Backend
```bash
go run ./cmd/awghop          # run dev server (HTTP :8080, data in ./data)
go build ./...               # build check
go vet ./...                 # static analysis
go test ./...                # run all tests
go test -race ./...          # run with race detector (matches CI)
go test ./internal/amnezia/... # run a single package's tests
```

### Frontend
```bash
cd web && npm ci             # install deps
cd web && npm run build      # build to web/dist (must be done before go build for embedded UI)
cd web && npm run dev        # dev server with proxy to API at http://127.0.0.1:8080
cd web && npm run check      # svelte-check type check
```

### Docker
```bash
docker compose up --build    # local build (HTTP on 127.0.0.1:8080, UDP 51820)
```

For dev with cross-origin frontend: set `AWGHOP_DEV=1` on the backend.

## Architecture

### Startup wiring (`cmd/awghop/main.go`)

`config.Load()` → `db.Open()` (runs migrations) → `store.New()` → `netctl.New()` → optional auto-apply → `api.NewRouter()` → `http.Server`.

### Layers

| Package | Role |
|---|---|
| `internal/config` | Loads all config from env vars into `Config` struct |
| `internal/db` | Opens SQLite, embeds and auto-runs `migrations/*.sql` |
| `internal/domain` | Pure domain types: `Client`, `UpstreamTunnel`, `IngressSettings` |
| `internal/store` | SQL data-access layer (clients, upstreams, auth, system settings) |
| `internal/api` | chi router, HTTP handlers, CSRF/rate-limit middleware |
| `internal/netctl` | `Controller` — syncs AWG interfaces + policy routing + NAT with the DB |
| `internal/amnezia` | Builds `.conf` files for `awg-quick`; generates AmneziaWG junk parameters |
| `internal/awgshow` | Parses `awg show <iface> dump` output (handshake/RX/TX) |
| `internal/wgquick` | Parses `awg-quick` / WireGuard `.conf` format |
| `internal/wgeasy` | Parses `wg-easy` JSON export for bootstrap import |
| `internal/ipalloc` | Allocates client IP addresses from the server's subnet |
| `internal/wgk` | WireGuard key-pair generation |
| `internal/ui` | `//go:embed dist` — serves the built Svelte app |

### netctl.Controller — the core apply pipeline

`Controller.Apply(ctx, store)` is the heart of the system. It runs under a mutex and:
1. Tears down previous iptables/ip-rule/ip-route entries from saved `net-policy-state.json`.
2. Downs WG interfaces listed in `wireguard-runtime-state.json`.
3. Regenerates `$AWGHOP_DATA/wireguard/<iface>.conf` (ingress) and `upstream-<id>-*.conf` (egress) then runs `awg-quick up`.
4. Adds `ip rule from <clientIP>/32 → table 10000+<upstream_id>` and `ip route … default dev <upstream_iface>` per via-upstream client.
5. Adds `iptables -t nat -A POSTROUTING … -j MASQUERADE` per client (on external iface for `direct`, on upstream iface for `via_upstream`).
6. Persists state to disk. Failed upstreams are aggregated into a warning rather than aborting the whole apply (best-effort upstreams).

All state lives in three JSON files inside `$AWGHOP_DATA`:
- `net-policy-state.json` — ip rule/route entries to undo
- `wireguard-runtime-state.json` — WG interface names to tear down
- `nat-state.json` — iptables MASQUERADE rules to undo

### Auto-apply after mutations

Every successful API mutation (create/update/delete client, upstream, or ingress settings) calls `Handlers.applyAfterMutation()`, which runs `Controller.Apply` in a goroutine. Errors are logged and surfaced via `GET /api/v1/system/status` (`last_error` field). Explicit `POST /api/v1/system/apply` is only needed for manual re-deploy.

### Database schema

Migrations are embedded Go files in `internal/db/migrations/` and run automatically at startup in version order:
- `001_init.sql` — clients, sessions, system settings
- `002_server_keys.sql` — ingress WG key storage
- `003_upstream_tunnels.sql` — upstream tunnel records
- `004_system_settings.sql` — tunnel_offline_policy and external_interface settings

### Frontend

Svelte 5 SPA in `web/src/`. Views: `Dashboard`, `Clients`, `Upstreams`, `Settings`, `Backup`, `Bootstrap`, `Login`. API client is in `web/src/lib/api.ts`. Types mirror backend domain in `web/src/lib/types.ts`.

The compiled output (`web/dist/`) is embedded into the binary via `internal/ui/embed.go`. **When changing the frontend, always run `npm run build` before testing the Go binary.**

### Security model

- Session cookie (`awghop_session`, httpOnly) + CSRF double-submit (`X-CSRF-Token` header vs `awghop_csrf` cookie).
- Rate-limit on `/auth/login` and `/setup/bootstrap` (5 req/min/IP).
- `AWGHOP_TLS=1` enables secure cookies (set when behind a TLS reverse proxy).
- `AWGHOP_DEV=1` opens CORS to any origin (dev only).

## Key env vars

| Variable | Default | Notes |
|---|---|---|
| `AWGHOP_LISTEN` | `:8080` | HTTP listen address |
| `AWGHOP_DATA` | `./data` | DB, keys, generated confs, state files |
| `AWGHOP_DEV` | `0` | Dev CORS (any origin) |
| `AWGHOP_AUTO_APPLY` | `1` | Apply on startup |
| `AWGHOP_WG_QUICK_BIN` | `wg-quick` | `awg-quick` in Docker |
| `AWGHOP_AWG_BIN` | `awg` | Used for `awg show` status parsing |
| `AWGHOP_EXTERNAL_IFACE` | autodetect | External iface for direct-client NAT |
| `AWGHOP_LOG_FORMAT` | `text` | `json` for log aggregators |

## Testing notes

Unit tests cover `internal/amnezia` (config building, defaults) and `internal/awgshow` + `internal/wgeasy` (parsers). The `netctl` controller requires Linux kernel interfaces (`awg-quick`, `ip`, `iptables`) so it is not unit-tested; integration is validated via Docker.

CI runs `go test -race ./...` — always use `-race` when adding new concurrent code.
