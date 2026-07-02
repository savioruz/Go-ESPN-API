# Load test — Go-ESPN-API

A [k6](https://k6.io/) script (`events_news.js`) that drives the two hottest read
endpoints the sheka backend polls:

- `GET /api/v1/events/?league=<league>&ordering=date`
- `GET /api/v1/news/?league=<league>`

It is **not** run as part of CI — it needs a live deployment with data. Run it
against a staging deploy of the Go service (and, for comparison, the Django
service) under the same profile.

## Prerequisites

- `k6` installed (`brew install k6` / see k6.io/docs).
- A running Go-ESPN-API with data ingested and a valid API key.

## Run

```bash
BASE_URL=http://localhost:8080 \
API_KEY=your-key \
LEAGUE=nba \
VUS=50 DURATION=1m \
k6 run scripts/loadtest/events_news.js
```

Ramps 0→`VUS` over 15s, holds for `DURATION`, ramps down. Thresholds:
`http_req_failed < 1%` and `http_req_duration p95 < 300ms` (tune to your infra).

## What to record

Capture these for both the Go and Django deploys to demonstrate the performance
goal:

| Metric | Where |
|---|---|
| Latency p50 / p95 / p99 | k6 summary (`http_req_duration`) |
| Error rate | k6 summary (`http_req_failed`) |
| Throughput (req/s) | k6 summary (`http_reqs`) |
| App RSS + CPU | `docker stats espn-app` during the run |
| Peak Postgres connections | `SELECT count(*) FROM pg_stat_activity WHERE datname = current_database();` |

The Postgres connection count is the key parity signal: the Go service holds a
bounded pool (≤ 10 open) versus Django's per-request/Celery connection churn that
was spiking into "too many clients".

## Alternative: vegeta

If you prefer [vegeta](https://github.com/tsenart/vegeta), a targets file:

```
GET http://localhost:8080/api/v1/events/?league=nba&ordering=date
X-API-Key: your-key

GET http://localhost:8080/api/v1/news/?league=nba
X-API-Key: your-key
```

```bash
vegeta attack -targets=targets.txt -rate=100 -duration=60s | vegeta report
```
