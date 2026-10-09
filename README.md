# PulseWatch

HTTP monitor + status pages. Dark ops UI (rose `#E11D48`). Go + Next.js + Postgres.

![Login](docs/login.png)
![Monitors](docs/monitors.png)
![Monitor detail](docs/monitor-detail.png)
![Public status](docs/status.png)
![Incidents](docs/incidents.png)

## What it does

- Auth with orgs: register/login return `orgs[]`; the frontend persists `orgs[0].id` and sends `X-Org-ID` (+ `X-User`) on tenant calls. JWT access 15 min, rotating refresh in an httpOnly cookie.
- Monitor CRUD: URL, method, interval (10–3600s), timeout (1–30s), expected status, body-contains assert, optional webhook, pause/resume.
- Worker pool: 5s scheduler tick, 8 check workers, per-check timeout, 1 MB body cap, same-host redirects (max 3, re-validated). Opens an incident on failure, resolves on recovery; fires log + webhook alerts. Graceful shutdown cancels workers before the HTTP server stops.
- Check history (latency, status code, error), 24h uptime %, per-monitor incidents, alert log.
- Public status page per org slug (`/status/demo`, no auth).
- **Hard anti-SSRF**: only `http/https`, no userinfo, hostname resolved via DNS and every IP checked — loopback, private (10/8, 172.16/12, 192.168/16), link-local/cloud metadata `169.254.169.254`, `::1`, `fc00::/7`, `fe80::/10`, `0.0.0.0/8`, `100.64/10` all refused, including decimal/octal/hex literal bypasses. Redirects re-validated. Concurrency (8), per-monitor rate limit (manual check 1/10s), timeouts and body caps enforced.

## Quickstart

```bash
cp .env.example .env
docker compose up -d --build
# API :8086, web :3006, db :5439. Seed runs on first boot (SEED=1).
```

Demo login: `admin@pulsewatch.demo` / `demo1234` (org `Demo Ops`, slug `demo`).

Local dev without Docker:

```bash
# db
docker compose up -d db
# api
export DATABASE_URL=postgres://pulsewatch:pulsewatch@localhost:5439/pulsewatch?sslmode=disable
export JWT_SECRET=dev-secret-at-least-16-chars PORT=8086 SEED=1
go run ./cmd/api
# web
cd web && npm install && npm run dev
```

## API

Canonical prefix `/api/v1` (same handlers also mounted unprefixed for the frontend). Envelopes: success `{data:...}`, errors `{error:{code,message}}`. Full spec: `openapi.yaml`, served at `GET /openapi.yaml`. Key routes:

| Method | Path | Notes |
|---|---|---|
| POST | `/auth/register`, `/auth/login`, `/auth/refresh`, `/auth/logout` | login returns `{user, orgs, access_token}` |
| GET/POST | `/monitors` | list enriched with `uptime_24h`, `last_code`, `last_latency_ms` |
| GET/PUT/DELETE | `/monitors/{id}` | org-scoped, SSRF-validated |
| GET | `/monitors/{id}/checks?limit=50` | newest first |
| GET | `/monitors/{id}/uptime?hours=24` | `{uptime_pct,total,ok,hours}` |
| GET | `/monitors/{id}/incidents` | per-monitor incidents |
| POST | `/monitors/{id}/check` | manual check, 1 per 10s |
| GET/POST | `/incidents`, `/incidents/{id}/resolve` | `?status=open` filter |
| GET | `/alerts` | last 100 alert logs |
| GET | `/public/{slug}/status` | **no auth** — org, monitors+uptime, open incidents |
| GET | `/healthz`, `/openapi.yaml` | health reports `{status, db}` |

## Configuration

| Var | Default | Meaning |
|---|---|---|
| `DATABASE_URL` | — (required) | pgx pool URL |
| `JWT_SECRET` | — (required, 16+ chars) | access-token HMAC |
| `PORT` | `8086` | API listen port |
| `SEED` | `0` | `1` = idempotent demo seed on boot |
| `NEXT_PUBLIC_API_URL` | `http://localhost:8086` | baked into web at build (Docker `ARG`) |
| `DB_PORT`/`API_PORT`/`WEB_PORT` | `5439`/`8086`/`3006` | compose host mappings |

## Tests

```bash
go test ./...          # ssrf (169.254.169.254, 127.0.0.1, localhost, privates refused), worker transitions, checker redirects, config
cd web && npm run build # tsc --noEmit + next build
```

E2E: Playwright against the local stack — login persists `orgs[0].id`, monitors CRUD, detail charts, public status (see `docs/*.png`, captured from a live run).

## Security notes

- bcrypt passwords, short-lived JWT + hashed refresh tokens, `X-Org-ID` membership enforced per query.
- SSRF is the headline threat: DNS-resolution check (not string match), redirect re-validation, no metadata access. See `internal/ssrf` + tests.
- CORS allowlist includes `Authorization`, `X-Org-ID`, `X-User`; error envelope never leaks internals.

## Layout

```
cmd/api cmd/seed  internal/{config,db,auth,http,ssrf,checker,worker,alerts,seed}
migrations/ openapi.yaml  web/ (Next.js 15)  docs/  docker-compose.yml
Dockerfile.backend  web/Dockerfile  .github/workflows/ci.yml
```

More in [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). PT-BR: [README.pt-BR.md](README.pt-BR.md). License: MIT.
