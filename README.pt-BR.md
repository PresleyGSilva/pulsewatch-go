# PulseWatch

Monitoramento HTTP + páginas de status. UI dark ops (rose `#E11D48`). Go + Next.js + Postgres.

![Login](docs/login.png)
![Monitores](docs/monitors.png)
![Detalhe](docs/monitor-detail.png)
![Status público](docs/status.png)
![Incidentes](docs/incidents.png)

## O que faz

- Auth com orgs: register/login retornam `orgs[]`; o frontend persiste `orgs[0].id` e envia `X-Org-ID` (+ `X-User`) nas chamadas do tenant. JWT de acesso 15 min, refresh rotativo em cookie httpOnly.
- CRUD de monitores: URL, método, intervalo (10–3600s), timeout (1–30s), status esperado, assert de corpo, webhook opcional, pausar/retomar.
- Worker pool: tick de 5s, 8 workers, timeout por check, teto de 1 MB de corpo, redirects só mesmo host (máx 3, revalidados). Abre incidente na falha, resolve na recuperação; dispara alertas de log + webhook. Shutdown gracioso cancela workers antes de parar o HTTP.
- Histórico de checks (latência, status, erro), uptime % 24h, incidentes por monitor, log de alertas.
- Página pública de status por slug da org (`/status/demo`, sem auth).
- **Anti-SSRF duro**: só `http/https`, sem userinfo, hostname resolvido via DNS e todo IP checado — loopback, privados (10/8, 172.16/12, 192.168/16), link-local/metadados cloud `169.254.169.254`, `::1`, `fc00::/7`, `fe80::/10`, `0.0.0.0/8`, `100.64/10` recusados, incluindo bypass decimal/octal/hex. Redirects revalidados. Concorrência (8), rate limit (check manual 1/10s), timeouts e teto de corpo aplicados.

## Início rápido

```bash
cp .env.example .env
docker compose up -d --build
# API :8086, web :3006, db :5439. Seed roda no primeiro boot (SEED=1).
```

Login demo: `admin@pulsewatch.demo` / `demo1234` (org `Demo Ops`, slug `demo`).

Dev local sem Docker:

```bash
docker compose up -d db
export DATABASE_URL=postgres://pulsewatch:pulsewatch@localhost:5439/pulsewatch?sslmode=disable
export JWT_SECRET=dev-secret-at-least-16-chars PORT=8086 SEED=1
go run ./cmd/api
cd web && npm install && npm run dev
```

## API

Prefixo canônico `/api/v1` (mesmos handlers sem prefixo para o frontend). Envelopes: sucesso `{data:...}`, erros `{error:{code,message}}`. Spec completa: `openapi.yaml`, servido em `GET /openapi.yaml`. Rotas principais: auth (login retorna `{user, orgs, access_token}`), `/monitors` (lista enriquecida com `uptime_24h`, `last_code`, `last_latency_ms`), `/monitors/{id}/checks`, `/uptime?hours=24`, `/incidents`, `POST /monitors/{id}/check` (1 por 10s), `/incidents/{id}/resolve`, `/alerts`, `/public/{slug}/status` (sem auth), `/healthz`.

## Configuração

`DATABASE_URL` (obrigatória), `JWT_SECRET` (16+ chars), `PORT` (8086), `SEED` (0/1), `NEXT_PUBLIC_API_URL` (bake no build web via `ARG`), `DB_PORT`/`API_PORT`/`WEB_PORT` (5439/8086/3006).

## Testes

```bash
go test ./...          # ssrf (169.254.169.254, 127.0.0.1, localhost, privados recusados), transições do worker, redirects, config
cd web && npm run build # tsc --noEmit + next build
```

E2E: Playwright contra o stack local — login persiste `orgs[0].id`, CRUD de monitores, gráficos, status público (ver `docs/*.png`, capturas reais).

## Segurança

- bcrypt, JWT curto + refresh hasheado, membership de `X-Org-ID` checada por query.
- SSRF é a ameaça principal: checagem por resolução DNS (não string match), revalidação de redirects, sem acesso a metadados. Ver `internal/ssrf` + testes.
- CORS permite `Authorization`, `X-Org-ID`, `X-User`; erros nunca vazam internals.

## Layout

```
cmd/api cmd/seed  internal/{config,db,auth,http,ssrf,checker,worker,alerts,seed}
migrations/ openapi.yaml  web/ (Next.js 15)  docs/  docker-compose.yml
Dockerfile.backend  web/Dockerfile  .github/workflows/ci.yml
```

Detalhes em [docs/ARCHITECTURE.md](docs/ARCHITECTURE.md). License: MIT.
