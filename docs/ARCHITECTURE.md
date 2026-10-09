# Architecture — PulseWatch

Monolithic Go API + Next.js frontend + Postgres 16. One repo, `docker compose up` runs everything.

## Runtime

```
browser ──► web (Next 15, :3006) ──► api (Chi, :8086) ──► postgres 16 (:5439)
                     │                        │
                     │                        ├── worker pool (5s tick, 8 checkers)
                     │                        └── targets (example.com, httpbin…) — SSRF-gated
                     └── /status/[slug] is server-rendered, calls /public/{slug}/status (no auth)
```

## Backend (`cmd/api`, `internal/`)

- `config`: env load + validation (`DATABASE_URL`, `JWT_SECRET` 16+, `PORT`, `SEED`).
- `db`: pgxpool connect, file-based migrator (`migrations/*.up.sql`, tracked in `schema_migrations`), health probe.
- `auth`: bcrypt (cost 11), access JWT 15 min (`user_id`, `org_ids`), refresh opaque SHA-256 in `refresh_tokens`, httpOnly cookie `pw_refresh`, rotation on refresh.
- `http`: Chi router. `requireAuth` (Bearer) + `requireOrg` (`X-Org-ID` membership via `org_members`). Canonical `/api/v1`, same handlers mounted at `/` for the frontend. Envelopes `{data}` / `{error:{code,message}}`. `GET /monitors` enriches rows with 24h uptime + latest check in one `DISTINCT ON` query (defaults 100% when no checks). `GET /monitors/{id}/uptime` uses `make_interval(hours => $2)` (a `$2 || ' hours'` variant fails: int param has no `||` operator — found live, fixed).
- `ssrf.ValidateURL`: scheme http/https only, no userinfo, length cap; host classified by `netip` (literal, decimal/octal/hex forms) or `net.LookupIP` (DNS names); blocked: localhost variants, `.localhost`/`.internal`/`.local`, loopback, RFC1918, 169.254/16 (metadata), `::1`, `fc00::/7`, `fe80::/10`, `0.0.0.0/8`, `100.64/10`, any-empty resolution. Pure func, table-tested.
- `checker.Check`: per-monitor timeout ctx, global semaphore (8), 1 MB `LimitReader`, redirect policy ≤3 same-host with re-`ValidateURL`, asserts (status, body substring), returns `{OK, StatusCode, LatencyMs, ErrMsg}`.
- `worker`: `Run(ctx)` tick 5s → due monitors (`active`, `last_checked_at + interval < now()`) → 8-goroutine pool → `ApplyResult` (insert check, update monitor, open/resolve incident, alert logs). Webhook POST 5s timeout, never fails the check. `WaitGroup` drains on ctx cancel; `main` cancels workers then `srv.Shutdown(10s)`.
- `alerts`: log + webhook dispatch helpers. `seed`: idempotent demo org/user/monitors/checks/incident.

## Schema (`migrations/`)

`users`, `orgs`, `org_members(org_id,user_id,role)` pk composite, `refresh_tokens`, `monitors` (+`webhook_url`, `consecutive_failures`, `last_status`, `last_checked_at`), `checks(monitor_id, checked_at)` idx `(monitor_id, checked_at DESC)`, `incidents(monitor_id, status)` idx `(monitor_id, status)`, `alert_logs`. App-level tenancy (`org_id` on every row); no RLS.

## Frontend (`web/`)

Next 15 app router, TS, Tailwind, standalone output. `lib/api.ts`: `API_BASE`, `handle()` unwraps the `{data}` envelope once, sends `Bearer` + `X-Org-ID` + `X-User`; `login()` persists `pw_token/pw_user/pw_orgs/pw_org_id=orgs[0].id`. Pages: `/login`, `/monitors`, `/monitors/[id]` (SVG sparkline, checks, incidents), `/incidents`, `/alerts`, `/status/[slug]` (public, unwraps envelope + `org.name`). Dark zinc-950 + rose `#E11D48`.

## Ops

`docker-compose.yml` (ports via `DB_PORT/API_PORT/WEB_PORT`), `Dockerfile.backend` (multi-stage, `HEALTHCHECK /healthz`), `web/Dockerfile` (`ARG NEXT_PUBLIC_API_URL`, standalone), `.github/workflows/ci.yml` (go vet+test, tsc+build). Structured `slog` logs; `/healthz` reports `{status, db}` without requiring auth.

## Failure modes handled

- Target down → incident opens, consecutive_failures++ ; recovery → resolved + alert rows.
- SSRF probe (metadata/loopback/private) → `400 validation_error`, nothing fetched (tested + proven live).
- Manual check spam → `429` (1/10s). Slow target → ctx timeout recorded as failed check.
- DB down → `/healthz` degraded, API 500s with envelope, workers idle until pool recovers.
