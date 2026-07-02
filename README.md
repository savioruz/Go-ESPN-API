# Go-ESPN-API

A Go rewrite of the Django `espn_service` + `nhl_service`. It is a **drop-in
replacement** for the ESPN service deployed at `https://espn-0a381a153f.sheka.xyz`
and consumed by the sheka backend (`backend/src/domains/prediction/ingestor.ts`),
which is tightly coupled to the DRF JSON shape. The Go service reproduces that
shape byte-for-byte for the read endpoints while replacing Celery-beat + worker
with an in-process cron scheduler.

- **ESPN** read API is served under `/api/v1/*`.
- **NHL** read API is served under `/api/v1/nhl/*` (deviation from Django, which
  served NHL at `/api/v1/` — but it has no consumer and ESPN owns `/api/v1/teams`).
- **Ingest** refresh hooks are `POST /api/v1/ingest/*`.
- Both service groups are gated by `SERVICES_ESPN_ENABLED` / `SERVICES_NHL_ENABLED`.

## Tech stack

| Category | Technology |
|---|---|
| Language | Go 1.26 |
| HTTP router | chi v5 |
| Database | PostgreSQL via sqlx (read/write split) |
| Cache | Redis 7 |
| DI | google/wire (compile-time) |
| Logging | zerolog |
| Tracing | OpenTelemetry (OTLP gRPC) |
| Scheduler | robfig/cron v3 |
| Migrations | golang-migrate |
| API docs | swaggo/swag → Scalar (OpenAPI 3.1) |

Request path spans: `handler → service → client/repo` (OTel scopes `handler`,
`service`, `espn_client` / `nhl_client`, `repository`). The HTTP app reports as
service name `${APP_NAME}` (default `espn`); the worker overrides it to
`espn_worker` in-process so traces are distinguishable.

## Running

### Local (Go toolchain)

```bash
cp .env.example .env          # set APP_API_KEY etc.
make migrate.up               # apply migrations
make dev                      # HTTP API with hot reload (air)
# or: make run                # HTTP API without reload
make worker                   # ingestion scheduler
```

### Docker Compose (full stack)

```bash
cp .env.example .env          # set APP_API_KEY
docker compose up -d postgres redis migrate app worker
# optional traces UI:
docker compose up -d jaeger   # then set EXTERNAL_OTEL_ENDPOINT=jaeger:4317
```

The `migrate` service runs `/migrate up` once and exits; `app` and `worker`
`depends_on` it completing, so the schema is always in place before they boot.

### Make targets

| Target | Purpose |
|---|---|
| `make dev` | run the HTTP API with hot reload |
| `make run` | run the HTTP API once |
| `make worker` | run the ingestion scheduler |
| `make build` | build the `engine` binary |
| `make migrate.up` / `make migrate.down` | apply / revert migrations |
| `make migrate.create name=<n>` | scaffold a new migration |
| `make test` | run tests with coverage |
| `make lint` | run golangci-lint |
| `make generate` | regenerate swagger + wire + openapi |

## Environment variables

Full reference is in [`.env.example`](.env.example). Key knobs:

### App / auth
| Var | Default | Notes |
|---|---|---|
| `APP_API_KEY` | — | **Required.** Value clients must send as `X-API-Key`. |
| `APP_NAME` | `espn` | OTel service name / cache-key prefix. |
| `APP_CORS_ENABLE` | `true` | Toggle CORS. |
| `APP_RATE_LIMITER_ENABLE` | `true` | Toggle rate limiting. |
| `SERVER_PORT` | `8080` | HTTP listen port. |
| `SERVER_ENV` | — | `development` exposes `/docs`; else docs are off. |

### Service gates
| Var | Default | Notes |
|---|---|---|
| `SERVICES_ESPN_ENABLED` | `true` | Mounts `/api/v1/*` ESPN routes + ESPN scheduler jobs. |
| `SERVICES_NHL_ENABLED` | `false` | Mounts `/api/v1/nhl/*` routes + NHL scheduler jobs. When off, NHL paths return 404. |

### Ingest / scheduler
| Var | Default | Notes |
|---|---|---|
| `SERVICES_ESPN_INGEST_LEAGUES` | `soccer:fifa.world,...,basketball:nba` | Comma-separated `sport:league` pairs the periodic jobs fan out over. Trimmed from the Django ~45-league set to avoid Postgres "too many clients". |
| `SERVICES_ESPN_INGEST_CONCURRENCY` | `4` | Max concurrent ingest units per job. Keep ≤ Postgres pool max-open. |
| `SERVICES_NHL_INGEST_CONCURRENCY` | `4` | Max concurrent per-team roster ingests. |
| `SERVICES_ESPN_TIMEOUT` / `SERVICES_NHL_TIMEOUT` | `30` | Upstream HTTP timeout (s). |
| `SERVICES_ESPN_MAXRETRIES` / `SERVICES_NHL_MAXRETRIES` | `3` | Upstream retry count. |
| `SERVICES_ESPN_RELAYS` | — | Optional comma-separated relay pool URLs. |

### Database
| Var | Default | Notes |
|---|---|---|
| `DB_POSTGRES_WRITE_*` / `DB_POSTGRES_READ_*` | see `.env.example` | Read/write host, port, user, password, name, timezone, ssl_mode. |
| `DB_POSTGRES_MAX_RETRY` | `3` | Connection retry attempts on startup. |
| `DB_POSTGRES_RETRY_WAIT_TIME` | `5` | Seconds between connection retries. |
| `DB_POSTGRES_AUTO_MIGRATE` | `false` | If `true`, the app runs migrations on start. |
| `DB_POSTGRES_MIGRATION_TABLE` | `espn_migrations` | Migration bookkeeping table. |

### Cache / tracing
| Var | Default | Notes |
|---|---|---|
| `CACHE_REDIS_PRIMARY_*` | see `.env.example` | Redis host/port/password/db/tls. |
| `CACHE_TTL` | `120` | Cache TTL (s). |
| `EXTERNAL_OTEL_ENDPOINT` | — | OTLP gRPC endpoint. Empty disables trace export. |

## Endpoint catalog

Full API reference with examples: [`docs/api.md`](docs/api.md).

All `/api/v1` routes require the `X-API-Key` header (403 without). Trailing
slashes are normalized, so `/api/v1/sports` and `/api/v1/sports/` both resolve.

### Public
| Method | Path | Notes |
|---|---|---|
| GET | `/healthz` | Health check (Django-compatible, no auth). |
| GET | `/health` | Alias kept for backward compat. |
| GET | `/docs` | Scalar API reference (only when `SERVER_ENV=development`). |

### ESPN — `/api/v1/*` (gated by `SERVICES_ESPN_ENABLED`)
| Method | Path |
|---|---|
| GET | `/api/v1/sports/`, `/api/v1/sports/{slug}/` |
| GET | `/api/v1/leagues/`, `/api/v1/leagues/{id}/` |
| GET | `/api/v1/teams/`, `/api/v1/teams/espn/{espn_id}/`, `/api/v1/teams/{id}/` |
| GET | `/api/v1/events/`, `/api/v1/events/espn/{espn_id}/`, `/api/v1/events/{id}/` |
| GET | `/api/v1/news/`, `/api/v1/news/{id}/` |
| GET | `/api/v1/injuries/`, `/api/v1/injuries/{id}/` |
| GET | `/api/v1/transactions/`, `/api/v1/transactions/{id}/` |
| GET | `/api/v1/athlete-stats/`, `/api/v1/athlete-stats/{id}/` |

### NHL — `/api/v1/nhl/*` (gated by `SERVICES_NHL_ENABLED`)
| Method | Path |
|---|---|
| GET | `/api/v1/nhl/teams/`, `/api/v1/nhl/teams/{id}/` |
| GET | `/api/v1/nhl/players/`, `/api/v1/nhl/players/{id}/` |
| GET | `/api/v1/nhl/games/`, `/api/v1/nhl/games/{id}/` |
| GET | `/api/v1/nhl/standings/`, `/api/v1/nhl/standings/{id}/` |
| GET | `/api/v1/nhl/skater-stats/`, `/api/v1/nhl/skater-stats/{id}/` |
| GET | `/api/v1/nhl/goalie-stats/`, `/api/v1/nhl/goalie-stats/{id}/` |

### Ingest — `POST /api/v1/ingest/*` (gated by `SERVICES_ESPN_ENABLED`)
| Method | Path |
|---|---|
| POST | `/api/v1/ingest/scoreboard` |
| POST | `/api/v1/ingest/teams` |
| POST | `/api/v1/ingest/news` |
| POST | `/api/v1/ingest/injuries` |
| POST | `/api/v1/ingest/transactions` |

## Scheduler jobs

The `worker` process registers cron jobs on boot (ported 1:1 from the Django
`CELERY_BEAT_SCHEDULE`). Jobs are **not** run immediately on start; cron fires
them on the first interval boundary. Each job is panic- and error-isolated: a
failure in one league or a panic in one tick never aborts the rest.

### ESPN jobs (when `SERVICES_ESPN_ENABLED=true`)
| Job | Cadence | What it does |
|---|---|---|
| `scoreboards` | 5 min | Re-ingest every configured league for today + yesterday (UTC). |
| `unstick` | 10 min | Re-ingest scoreboards for games stuck past kickoff. |
| `news` | 30 min | Refresh news per league (limit 50). |
| `injuries` | 4 h | Refresh injuries per league. |
| `transactions` | 6 h | Refresh transactions per league. |
| `teams` | 7 d | Refresh team rosters per league. |

### NHL jobs (when `SERVICES_NHL_ENABLED=true`)
| Job | Cadence | What it does |
|---|---|---|
| `nhl_teams` | 7 d | Sync the (near-static) team list. |
| `nhl_rosters` | 24 h | Sync rosters, fanning out over active teams. |
| `nhl_standings` | 1 h | Sync standings after game nights. |

If neither service is enabled, the worker idles cleanly (stays up, no DB/ESPN
contact) until it receives a signal.

## Env-gating behavior summary

- `SERVICES_ESPN_ENABLED=false` → `/api/v1/*` ESPN routes and `/api/v1/ingest/*`
  are not mounted (404); ESPN scheduler jobs are not registered.
- `SERVICES_NHL_ENABLED=false` → `/api/v1/nhl/*` returns 404; NHL jobs not registered.
- Missing/invalid `X-API-Key` on any mounted `/api/v1` route → 403.
- `/healthz` is always public.
