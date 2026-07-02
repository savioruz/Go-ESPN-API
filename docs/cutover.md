# Cutover & rollback runbook — Go-ESPN-API

This runbook covers replacing the Django `espn_service` (deployed at
`https://espn-0a381a153f.sheka.xyz`, consumed by the sheka backend
`backend/src/domains/prediction/ingestor.ts`) with the Go rewrite, and rolling
back if needed.

The Go service is a **drop-in** for the read API. Two known, low-risk deltas:

1. **NHL path deviation.** Django `nhl_service` served NHL at `/api/v1/`; the Go
   service serves NHL under `/api/v1/nhl/`. NHL has no current consumer and ESPN
   owns `/api/v1/teams`, so this is intentional and safe.
2. **By-id 404 body.** For a missing resource fetched by id, the Go service
   returns `{"error":"Not found."}` whereas Django returns `{"detail":"..."}`.
   The list endpoints (what the ingestor uses) are byte-identical; this only
   affects direct by-id 404s.

---

## 1. Provision Postgres + Redis

- Postgres 16 (fresh database, e.g. `espn`) and Redis 7 reachable from the app
  and worker.
- Set `DB_POSTGRES_WRITE_*` / `DB_POSTGRES_READ_*` (point both at the same
  instance unless you have a replica) and `CACHE_REDIS_PRIMARY_*`.
- Keep `SERVICES_ESPN_INGEST_CONCURRENCY` (default 4) ≤ the Postgres pool
  max-open connections to avoid "too many clients".

## 2. Run migrations

Pick one:

```bash
# one-shot container (compose service exits 0 when done)
docker compose run --rm migrate

# or the binary / make target
make migrate.up            # local toolchain
/migrate up                # inside the image
```

Or let the app self-migrate on boot by setting `DB_POSTGRES_AUTO_MIGRATE=true`
(fine for a single-replica deploy; prefer the one-shot migrator for multi-replica
so replicas don't race).

## 3. Deploy app + worker

- Deploy the **app** (`/app`) and the **worker** (`/worker`) from the same image.
- Required env:
  - `APP_API_KEY` — must match what the sheka backend sends as `X-API-Key`.
  - `SERVICES_ESPN_ENABLED=true`.
  - `SERVICES_NHL_ENABLED=false` (no consumer yet; leave off unless needed).
  - `SERVICES_ESPN_INGEST_LEAGUES` — the trimmed league set (default covers
    soccer incl. World Cup, NFL, NBA).
  - `SERVICES_ESPN_INGEST_CONCURRENCY=4`.
  - DB + Redis connection vars.
- Confirm the worker logs `registered ingest jobs` with the expected `espn_jobs`
  count (6). With all services disabled it instead logs `worker idling`.

## 4. Back-fill and sanity-check row counts

The worker's cron does **not** fire immediately on boot (it waits for the first
interval). To populate data now, either wait for the intervals or trigger the
ingest hooks directly:

```bash
KEY=your-key; BASE=http://localhost:8080
for lg in eng.1 esp.1; do
  curl -sS -XPOST "$BASE/api/v1/ingest/scoreboard" -H "X-API-Key: $KEY" \
    -H 'Content-Type: application/json' -d "{\"sport\":\"soccer\",\"league\":\"$lg\"}"
  curl -sS -XPOST "$BASE/api/v1/ingest/news" -H "X-API-Key: $KEY" \
    -H 'Content-Type: application/json' -d "{\"sport\":\"soccer\",\"league\":\"$lg\"}"
done
curl -sS -XPOST "$BASE/api/v1/ingest/teams" -H "X-API-Key: $KEY" \
  -H 'Content-Type: application/json' -d '{"sport":"basketball","league":"nba"}'
```

Then check row counts:

```sql
SELECT 'sports' t, count(*) FROM sports
UNION ALL SELECT 'leagues', count(*) FROM leagues
UNION ALL SELECT 'teams',   count(*) FROM teams
UNION ALL SELECT 'events',  count(*) FROM events
UNION ALL SELECT 'news',    count(*) FROM news;
```

Expect non-zero sports/leagues/teams/events for the ingested leagues.

## 5. Parity check against Django

With both services reachable, run the parity harness (see below). Then point the
sheka backend ingestor at the Go host (or a staging host) and confirm a full
ingest cycle succeeds with no shape errors.

### Parity harness — `scripts/parity_check.sh`

Hits the same read endpoints on Django and Go and diffs the JSON with sorted
keys (`jq -S`), reporting per-endpoint PASS/FAIL. Requires `curl` + `jq`; it does
not need the services reachable when you're just reading the script.

```bash
DJANGO_BASE=https://espn-0a381a153f.sheka.xyz \
GO_BASE=http://localhost:8080 \
API_KEY=your-key \
scripts/parity_check.sh
```

Endpoints compared: `/api/v1/sports/`,
`/api/v1/events/?league=nba&ordering=date`, `/api/v1/news/?league=nba`,
`/api/v1/injuries/?league=nfl`, and one `/api/v1/events/{id}/` (auto-discovered
from the Go list, or set `EVENT_ID`). Exit code is non-zero if any endpoint
diffs. Set `KEEP_TMP=1` to keep the per-endpoint response bodies.

Expect PASS on the list endpoints. A by-id 404 may differ per the known delta
above — that's acceptable.

### Load test

Optional but recommended before cutover — see `scripts/loadtest/README.md`.
Record p50/p95, app RSS/CPU, and peak Postgres connections for Go vs Django.

## 6. DNS / route cutover (keep Django warm)

- Repoint `espn-0a381a153f.sheka.xyz` (or your ingress route) at the Go app.
- **Keep the Django deployment running and warm** for the rollback window; do not
  tear it down until the Go service has run clean through several scheduler
  cycles (scoreboards every 5 min, news every 30 min) and the sheka ingestor has
  completed multiple cycles without errors.
- Watch: HTTP 5xx rate, worker job logs (`scheduler job completed` with sane
  created/updated counts, low errors), and Postgres connection count.

## 7. Rollback

If the Go service misbehaves:

1. Repoint DNS / the ingress route back to the still-warm Django deployment.
   This is the fast path — no data migration needed, since both write the same
   schema/shape.
2. Stop the Go **worker** first (halts ingestion writes), then the Go app.
3. If the Go worker wrote bad data, Django's own Celery jobs re-ingest and
   overwrite on their next cycle (ingest is idempotent upsert). If needed, the
   scoreboard/news/teams ingest hooks can be re-driven manually (step 4).
4. Capture logs + the failing parity diff before redeploying so the issue can be
   fixed forward.

Because the two services share the same Postgres schema and produce the same
JSON, cutover and rollback are both just a route flip — no schema or data
conversion is involved.
