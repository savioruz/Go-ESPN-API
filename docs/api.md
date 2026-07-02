# Go-ESPN-API — REST API Reference

> Per-endpoint reference for the Go service's own `/api/v1` REST API, with real
> example responses captured from a running instance. Long arrays are truncated
> to one or two representative items with a `… (truncated)` note.

---

## Overview

Go-ESPN-API is a **drop-in Go rewrite** of the Django `espn_service` +
`nhl_service`. It reproduces the Django REST Framework (DRF) JSON shape
byte-for-byte for the read endpoints, replacing Celery-beat + worker with an
in-process cron scheduler.

- **ESPN** read API — served under `/api/v1/*`.
- **NHL** read API — served under `/api/v1/nhl/*` (a deviation from Django, which
  served NHL at `/api/v1/`; see [Gating](#gating--service-enablement)).
- **Ingest** refresh hooks — `POST /api/v1/ingest/*`.

| | |
|---|---|
| Base URL (default) | `http://localhost:8080` |
| Versioning | Single version, path-prefixed `/api/v1`. No `Accept`-header versioning. |
| Content type | `application/json` |
| Trailing slashes | Normalized — `/api/v1/sports` and `/api/v1/sports/` resolve to the same handler. |

All examples below use `http://localhost:8080` as the base URL and a placeholder
API key of `your-api-key`.

---

## Authentication

Every `/api/v1/*` route (ESPN read, NHL read, and ingest) requires an
`X-API-Key` request header whose value matches the server's `APP_API_KEY`.
`/healthz`, `/health`, and `/docs` are public.

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/sports/
```

A missing or invalid key returns **403 Forbidden**:

```bash
curl http://localhost:8080/api/v1/sports/      # no X-API-Key header
```

```json
{ "error": "auth.forbidden" }
```

---

## Pagination

List endpoints use DRF `PageNumberPagination`:

- `?page=N` — 1-indexed. A missing, non-numeric, or `< 1` value falls back to page 1.
- **Page size is fixed at 25** (matches Django `PAGE_SIZE=25`). It is not client-configurable.

Every list response is wrapped in the DRF envelope:

```json
{
  "count": 50,
  "next": "http://localhost:8080/api/v1/news/?page=2",
  "previous": null,
  "results": [ ... ]
}
```

- `count` — total number of matching rows across all pages.
- `next` / `previous` — absolute URLs, or `null` at the last / first page.
- For **page 1 the `page` query parameter is omitted** from the `previous` URL
  (DRF behavior). Example from `GET /api/v1/news/?page=2`:

```json
{
  "count": 50,
  "next": null,
  "previous": "http://localhost:8080/api/v1/news/"
}
```

`results` is always a JSON array — empty collections marshal as `[]`, never `null`.

---

## Filtering, search & ordering

List endpoints accept a mix of the following query parameters (see each resource
for the exact set it supports):

| Parameter | Behavior |
|---|---|
| `search` | Case-insensitive substring match (`ILIKE '%term%'`) across a resource-specific set of columns. |
| `ordering` | Sort by a whitelisted field. Prefix with `-` for descending (e.g. `ordering=-date`). Unknown fields fall back to the resource default. |
| `sport`, `league` | Case-insensitive slug filters (e.g. `sport=basketball`, `league=nba`). |
| `date`, `date_from`, `date_to` | `YYYY-MM-DD` date filters (endpoint-dependent). |
| Other scalar filters | `status`, `team`, `is_active`, `abbreviation`, `season_year`, `season_type`, `season`, `game_type`, `athlete_espn_id`, etc. — documented per resource. |

Slug and text filters are matched case-insensitively, mirroring the Django
service.

Example — search + ordering on teams:

```bash
curl -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/teams/?search=lakers&ordering=display_name"
```

```json
{
  "count": 1,
  "next": null,
  "previous": null,
  "results": [
    { "id": 14, "abbreviation": "LAL", "display_name": "Los Angeles Lakers", "...": "..." }
  ]
}
```

---

## Errors

| Situation | Status | Body |
|---|---|---|
| Missing / invalid `X-API-Key` | 403 | `{ "error": "auth.forbidden" }` |
| Resource not found (by id / espn_id / slug) | 404 | `{ "error": "Not found." }` |
| Invalid request (bad body, missing required field, bad param) | 400 | `{ "error": "validation.failed" }` |
| Field-level validation errors | 422 / 400 | `{ "errors": [ { "field": "...", "message": "..." } ] }` |
| Rate limit exceeded (when enabled) | 429 | `{ "message": "..." }` |
| Upstream ESPN/NHL fetch failed (ingest) | 502 | `{ "error": "external.service" }` |

The general/404 form uses a single `error` key; validation failures with
per-field detail use an `errors` array of `{field, message}` objects.

404 example (missing team by id):

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/teams/999999/
```

```json
{ "error": "Not found." }
```

> **Cutover note:** for a by-id 404, this service returns `{"error":"Not found."}`
> whereas the legacy Django service returned `{"detail":"..."}`. The list
> endpoints (what the ingestor consumes) are byte-identical.

---

## Gating & service enablement

| Route group | Env gate | When disabled |
|---|---|---|
| `/api/v1/*` ESPN read + `/api/v1/ingest/*` | `SERVICES_ESPN_ENABLED` (default `true`) | Routes not mounted → 404. |
| `/api/v1/nhl/*` NHL read | `SERVICES_NHL_ENABLED` (default `false`) | Routes not mounted → 404. |

> **NHL path deviation:** the Django `nhl_service` served NHL at `/api/v1/`; this
> service serves NHL under `/api/v1/nhl/` so it can coexist with the ESPN group
> (which owns `/api/v1/teams`). NHL has no current consumer, so this is
> intentional and safe.

---

## Public endpoints

| Method | Path | Auth | Description |
|---|---|---|---|
| GET | `/healthz` | none | Health check (Django-compatible). Pings the read DB. |
| GET | `/health` | none | Alias of `/healthz`, kept for backward compatibility. |
| GET | `/docs` | none | Scalar API reference UI. Served **only when `SERVER_ENV=development`**. |

`GET /healthz` returns `200` when the read database is reachable, `503`
otherwise:

```bash
curl http://localhost:8080/healthz
```

```json
{ "message": "ok" }
```

The raw OpenAPI spec is committed at [`docs/openapi.json`](openapi.json) and
[`docs/swagger.json`](swagger.json); the `/docs` UI renders `docs/openapi.json`.

---

## ESPN resources — `/api/v1/*`

Gated by `SERVICES_ESPN_ENABLED`.

### Sports

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/sports/` | List sports (paginated). |
| GET | `/api/v1/sports/{slug}/` | Retrieve one sport by slug. |

**Query parameters (list):** `page`.
**Path parameters (detail):** `slug` (string) — e.g. `basketball`.

```bash
curl -H "X-API-Key: your-api-key" http://localhost:8080/api/v1/sports/
```

```json
{
  "count": 1,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 1,
      "slug": "basketball",
      "name": "Basketball",
      "created_at": "2026-07-02T08:37:19.677452Z",
      "updated_at": "2026-07-02T08:37:19.677452Z"
    }
  ]
}
```

`GET /api/v1/sports/basketball/` returns the bare object (no envelope):

```json
{
  "id": 1,
  "slug": "basketball",
  "name": "Basketball",
  "created_at": "2026-07-02T08:37:19.677452Z",
  "updated_at": "2026-07-02T08:37:19.677452Z"
}
```

---

### Leagues

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/leagues/` | List leagues (paginated). |
| GET | `/api/v1/leagues/{id}/` | Retrieve one league by numeric id. |

**Query parameters (list):** `sport` (slug filter), `page`.
**Path parameters (detail):** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/leagues/?sport=basketball"
```

```json
{
  "count": 1,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 1,
      "slug": "nba",
      "name": "National Basketball Association",
      "abbreviation": "NBA",
      "sport": {
        "id": 1,
        "slug": "basketball",
        "name": "Basketball",
        "created_at": "2026-07-02T08:37:19.677452Z",
        "updated_at": "2026-07-02T08:37:19.677452Z"
      },
      "created_at": "2026-07-02T08:37:19.677452Z",
      "updated_at": "2026-07-02T08:37:19.677452Z"
    }
  ]
}
```

`GET /api/v1/leagues/1/` returns the same object shape without the envelope.

---

### Teams

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/teams/` | List teams (paginated, lightweight serializer). |
| GET | `/api/v1/teams/{id}/` | Retrieve one team by numeric id (full serializer). |
| GET | `/api/v1/teams/espn/{espn_id}/` | Retrieve one team by ESPN id (full serializer). |

**Query parameters (list):** `sport`, `league`, `is_active` (boolean),
`abbreviation`, `search`, `ordering`, `page`.
**`search` matches:** `display_name`, `abbreviation`, `location`, `name`.
**`ordering` fields:** `display_name` (default), `abbreviation`, `created_at`.
**Path parameters:** `id` (integer) / `espn_id` (string).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/teams/?league=nba"
```

```json
{
  "count": 30,
  "next": "http://localhost:8080/api/v1/teams/?page=2",
  "previous": null,
  "results": [
    {
      "id": 1,
      "espn_id": "1",
      "abbreviation": "ATL",
      "display_name": "Atlanta Hawks",
      "short_display_name": "Hawks",
      "location": "Atlanta",
      "color": "c8102e",
      "primary_logo": "https://a.espncdn.com/i/teamlogos/nba/500/atl.png",
      "league_slug": "nba",
      "sport_slug": "basketball",
      "is_active": true
    }
    // … (24 more per page, truncated)
  ]
}
```

`GET /api/v1/teams/1/` (and the byte-identical `GET /api/v1/teams/espn/1/`)
return the full team serializer:

```json
{
  "id": 1,
  "espn_id": "1",
  "uid": "s:40~l:46~t:1",
  "slug": "atlanta-hawks",
  "abbreviation": "ATL",
  "display_name": "Atlanta Hawks",
  "short_display_name": "Hawks",
  "name": "Hawks",
  "nickname": "Atlanta",
  "location": "Atlanta",
  "color": "c8102e",
  "alternate_color": "fdb927",
  "is_active": true,
  "is_all_star": false,
  "logos": [
    {
      "alt": "",
      "rel": ["full", "default"],
      "href": "https://a.espncdn.com/i/teamlogos/nba/500/atl.png",
      "width": 500,
      "height": 500
    }
    // … (15 more logo variants, truncated)
  ],
  "primary_logo": "https://a.espncdn.com/i/teamlogos/nba/500/atl.png",
  "league": {
    "id": 1,
    "slug": "nba",
    "name": "National Basketball Association",
    "abbreviation": "NBA",
    "sport_slug": "basketball"
  },
  "created_at": "2026-07-02T08:37:19.677452Z",
  "updated_at": "2026-07-02T08:37:19.677452Z"
}
```

---

### Events

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/events/` | List events (paginated, list serializer). |
| GET | `/api/v1/events/{id}/` | Retrieve one event by numeric id (full serializer). |
| GET | `/api/v1/events/espn/{espn_id}/` | Retrieve one event by ESPN id (full serializer). |

**Query parameters (list):** `sport`, `league`, `date` (`YYYY-MM-DD`),
`date_from`, `date_to`, `status`, `season_year` (int), `season_type` (int),
`team`, `search`, `ordering`, `page`.
**`search` matches:** `name`, `short_name`.
**`ordering` fields:** `date` (default), `created_at`.
**Path parameters:** `id` (integer) / `espn_id` (string).

```bash
curl -H "X-API-Key: your-api-key" \
  "http://localhost:8080/api/v1/events/?league=nba&ordering=-date"
```

```json
{
  "count": 1,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 1,
      "espn_id": "401859967",
      "date": "2026-06-14T00:30:00Z",
      "name": "New York Knicks at San Antonio Spurs",
      "short_name": "NY @ SA",
      "status": "final",
      "status_detail": "Final",
      "league_slug": "nba",
      "sport_slug": "basketball",
      "venue_name": "Frost Bank Center",
      "competitors": [
        {
          "id": 1,
          "team": {
            "id": 27,
            "espn_id": "24",
            "abbreviation": "SA",
            "display_name": "San Antonio Spurs",
            "short_display_name": "Spurs",
            "location": "San Antonio",
            "color": "000000",
            "primary_logo": "https://a.espncdn.com/i/teamlogos/nba/500/sa.png"
          },
          "home_away": "home",
          "score": "90",
          "score_int": 90,
          "winner": false,
          "line_scores": [ { "value": 23, "period": 1, "displayValue": "23" } ],
          "records": [ { "name": "overall", "type": "total", "summary": "62-20", "abbreviation": "Total" } ],
          "statistics": [ { "name": "points", "abbreviation": "PTS", "displayValue": "90" } ],
          "leaders": [ { "name": "points", "displayName": "Points", "leaders": [ { "value": 25, "displayValue": "25", "athlete": { "id": "5037871", "fullName": "Dylan Harper" } } ] } ],
          "order": 0
        }
        // … (away competitor, truncated)
      ]
    }
  ]
}
```

`GET /api/v1/events/1/` (or `GET /api/v1/events/espn/401859967/`) returns the
full event serializer, which additionally includes `uid`, `season_year`,
`season_type`, `season_slug`, `week`, `clock`, `period`, `attendance`,
`broadcasts`, `links`, a full `venue` object, and `created_at` / `updated_at`:

```json
{
  "id": 1,
  "espn_id": "401859967",
  "uid": "s:40~l:46~e:401859967",
  "date": "2026-06-14T00:30:00Z",
  "name": "New York Knicks at San Antonio Spurs",
  "short_name": "NY @ SA",
  "season_year": 2026,
  "season_type": 3,
  "season_slug": "post-season",
  "week": null,
  "status": "final",
  "status_detail": "Final",
  "clock": "0.0",
  "period": 4,
  "attendance": 18984,
  "broadcasts": [ { "names": ["ABC"], "market": "national" } ],
  "links": [ { "rel": ["summary", "desktop", "event"], "href": "https://www.espn.com/nba/game/_/gameId/401859967/knicks-spurs", "text": "Gamecast", "shortText": "Gamecast", "isExternal": false, "isPremium": false, "language": "en-US" } ],
  "league": { "id": 1, "slug": "nba", "name": "National Basketball Association", "abbreviation": "NBA", "sport_slug": "basketball" },
  "venue": {
    "id": 1,
    "espn_id": "780",
    "name": "Frost Bank Center",
    "city": "San Antonio",
    "state": "TX",
    "country": "USA",
    "is_indoor": true,
    "capacity": null,
    "created_at": "2026-07-02T08:40:17.176960Z",
    "updated_at": "2026-07-02T08:40:17.176960Z"
  },
  "competitors": [ /* same shape as the list example above */ ],
  "created_at": "2026-07-02T08:40:17.176960Z",
  "updated_at": "2026-07-02T08:40:17.176960Z"
}
```

---

### News

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/news/` | List news articles (paginated, list serializer). |
| GET | `/api/v1/news/{id}/` | Retrieve one article by numeric id (full serializer). |

**Query parameters (list):** `sport`, `league`, `date_from` (`YYYY-MM-DD`),
`search`, `ordering`, `page`.
**`search` matches:** `headline`, `description`.
**`ordering` fields:** `published` (default), `created_at`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/news/?league=nba"
```

```json
{
  "count": 50,
  "next": "http://localhost:8080/api/v1/news/?page=2",
  "previous": null,
  "results": [
    {
      "id": 9,
      "espn_id": "2ccc9050da584",
      "headline": "Sources: Celtics sending Brown to 76ers for George, picks",
      "description": "Jaylen Brown has a new home, with the Celtics agreeing to send their longtime star to the 76ers for Paul George, two first-round picks (2028, '31) and two second-round selections, sources told ESPN's Shams Charania on Wednesday.",
      "published": "2026-07-02T04:09:27Z",
      "type": "HeadlineNews",
      "thumbnail": "https://a.espncdn.com/photo/2026/0701/r1683009_600x600_1-1.jpg",
      "league_slug": "nba",
      "sport_slug": "basketball"
    }
    // … (24 more per page, truncated)
  ]
}
```

`GET /api/v1/news/9/` adds `last_modified`, `categories`, `images`, `links`, and
`created_at` / `updated_at`:

```json
{
  "id": 9,
  "espn_id": "2ccc9050da584",
  "headline": "Sources: Celtics sending Brown to 76ers for George, picks",
  "description": "Jaylen Brown has a new home, ...",
  "published": "2026-07-02T04:09:27Z",
  "last_modified": "2026-07-02T04:09:27Z",
  "type": "HeadlineNews",
  "categories": [
    { "id": 9577, "type": "league", "description": "NBA", "leagueId": 46, "sportId": 46 }
    // … (team / athlete / guid categories, truncated)
  ],
  "images": [
    { "id": 49241930, "type": "header", "name": "Jaylen Brown Paul George [600x600]", "url": "https://a.espncdn.com/photo/2026/0701/r1683009_600x600_1-1.jpg", "width": 600, "height": 600, "credit": "AP Photo/Robert F. Bukaty" }
    // … (truncated)
  ],
  "links": {
    "web": { "href": "https://www.espn.com/nba/story/_/id/49241843/sources-celtics-sending-brown-76ers-george-picks" },
    "api": { "self": { "href": "https://content.core.api.espn.com/v1/sports/news/49241843" } },
    "mobile": { "href": "http://m.espn.go.com/nba/story?storyId=49241843" }
  },
  "thumbnail": "https://a.espncdn.com/photo/2026/0701/r1683009_600x600_1-1.jpg",
  "league_slug": "nba",
  "sport_slug": "basketball",
  "created_at": "2026-07-02T08:37:21.465122Z",
  "updated_at": "2026-07-02T08:37:21.465122Z"
}
```

---

### Injuries

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/injuries/` | List injuries (paginated). |
| GET | `/api/v1/injuries/{id}/` | Retrieve one injury by numeric id. |

**Query parameters (list):** `sport`, `league`, `status`, `team`, `search`,
`ordering`, `page`.
**`search` matches:** `athlete_name`, `injury_type`, `description`.
**`ordering` fields:** `updated_at` (default), `athlete_name`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/injuries/?league=nba"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty in offseason.** At capture time (early July, NBA offseason) ESPN
> returned no active injuries for the ingested leagues, so this collection is
> empty. Each item, when present, has the shape:
>
> ```json
> {
>   "id": 12,
>   "athlete_espn_id": "3136776",
>   "athlete_name": "Stephen Curry",
>   "position": "PG",
>   "status": "doubtful",
>   "status_display": "Doubtful",
>   "description": "Left knee soreness",
>   "injury_type": "Knee",
>   "injury_date": "2026-03-10",
>   "return_date": null,
>   "league_slug": "nba",
>   "sport_slug": "basketball",
>   "team_abbreviation": "GSW",
>   "created_at": "...",
>   "updated_at": "..."
> }
> ```

---

### Transactions

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/transactions/` | List transactions (paginated). |
| GET | `/api/v1/transactions/{id}/` | Retrieve one transaction by numeric id. |

**Query parameters (list):** `sport`, `league`, `date_from` (`YYYY-MM-DD`),
`search`, `ordering`, `page`.
**`search` matches:** `description`, `athlete_name`, `type`.
**`ordering` fields:** `date` (default), `created_at`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/transactions/?league=nba&ordering=-date"
```

```json
{
  "count": 25,
  "next": null,
  "previous": null,
  "results": [
    {
      "id": 1,
      "espn_id": "",
      "date": "2026-07-01",
      "description": "Re-signed F Jamir Watkins to a two-way contract.",
      "type": "",
      "athlete_name": "",
      "athlete_espn_id": "",
      "league_slug": "nba",
      "sport_slug": "basketball",
      "team_abbreviation": "WSH",
      "created_at": "2026-07-02T08:37:22.930239Z",
      "updated_at": "2026-07-02T08:37:22.930239Z"
    }
    // … (24 more, truncated)
  ]
}
```

`GET /api/v1/transactions/1/` returns the same object shape without the envelope.

---

### Athlete season stats

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/athlete-stats/` | List athlete season stat lines (paginated). |
| GET | `/api/v1/athlete-stats/{id}/` | Retrieve one stat line by numeric id. |

**Query parameters (list):** `sport`, `league`, `season`, `athlete_espn_id`,
`search`, `ordering`, `page`.
**`search` matches:** `athlete_name`.
**`ordering` fields:** `season_year` (default), `athlete_name`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/athlete-stats/?league=nba"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty at capture time.** The offseason NBA ingest produced no athlete season
> stats. Each item, when present, has the shape:
>
> ```json
> {
>   "id": 1,
>   "athlete_espn_id": "3136776",
>   "athlete_name": "Stephen Curry",
>   "season_year": 2026,
>   "season_type": 2,
>   "stats": { "avgPoints": 26.4, "avgRebounds": 4.5, "avgAssists": 6.1 },
>   "league_slug": "nba",
>   "sport_slug": "basketball",
>   "created_at": "...",
>   "updated_at": "..."
> }
> ```

---

## NHL resources — `/api/v1/nhl/*`

Gated by `SERVICES_NHL_ENABLED` (default off). NHL data is sourced from the NHL
API and populated only by the worker's NHL sync jobs (there are no NHL ingest
HTTP hooks).

### NHL teams

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/teams/` | List NHL teams (paginated). |
| GET | `/api/v1/nhl/teams/{id}/` | Retrieve one NHL team by numeric id. |

**Query parameters (list):** `search`, `ordering`, `page`.
**`search` matches:** `name`, `full_name`, `abbreviation`.
**`ordering` fields:** `name`, `abbreviation`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/teams/"
```

```json
{
  "count": 62,
  "next": "http://localhost:8080/api/v1/nhl/teams/?page=2",
  "previous": null,
  "results": [
    {
      "id": 7,
      "team_id": "36",
      "abbreviation": "SEN",
      "name": "(1917)",
      "full_name": "Ottawa Senators (1917)",
      "franchise_id": "3",
      "is_active": true,
      "raw_data": {
        "id": 36,
        "triCode": "SEN",
        "fullName": "Ottawa Senators (1917)",
        "leagueId": 133,
        "rawTricode": "SEN",
        "franchiseId": 3
      }
    }
    // … (24 more per page, truncated)
  ]
}
```

`GET /api/v1/nhl/teams/7/` returns the same object without the envelope.

---

### NHL players

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/players/` | List NHL players (paginated). |
| GET | `/api/v1/nhl/players/{id}/` | Retrieve one player by numeric id. |

**Query parameters (list):** `search`, `ordering`, `page`.
**`search` matches:** `first_name`, `last_name`, `full_name`.
**`ordering` fields:** `last_name`, `first_name`, `sweater_number`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/players/?search=aho"
```

```json
{
  "count": 560,
  "next": "http://localhost:8080/api/v1/nhl/players/?page=2",
  "previous": null,
  "results": [
    {
      "id": 95,
      "player_id": "8478427",
      "first_name": "Sebastian",
      "last_name": "Aho",
      "full_name": "Sebastian Aho",
      "sweater_number": "20",
      "position": "C",
      "current_team": {
        "id": 16,
        "team_id": "12",
        "abbreviation": "CAR",
        "name": "Hurricanes",
        "full_name": "Carolina Hurricanes",
        "franchise_id": "26",
        "is_active": true,
        "raw_data": { "id": 12, "triCode": "CAR", "fullName": "Carolina Hurricanes", "leagueId": 133, "rawTricode": "CAR", "franchiseId": 26 }
      },
      "is_active": true,
      "headshot_url": "https://assets.nhle.com/mugs/nhl/20252026/CAR/8478427.png"
    }
    // … (truncated)
  ]
}
```

`GET /api/v1/nhl/players/95/` returns the same nested object without the envelope.

---

### NHL games

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/games/` | List NHL games (paginated). |
| GET | `/api/v1/nhl/games/{id}/` | Retrieve one game by numeric id. |

**Query parameters (list):** `season` (e.g. `20232024`), `game_type` (int:
`1`=pre, `2`=regular, `3`=playoff), `status`, `ordering`, `page`.
**`ordering` fields:** `date`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/games/"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty at capture time.** No NHL games were ingested (offseason; the games
> sync had no schedule to pull). Each item, when present, has the shape:
>
> ```json
> {
>   "id": 1,
>   "game_id": "2023020001",
>   "season": "20232024",
>   "game_type": 2,
>   "date": "2023-10-10",
>   "home_team": { "id": 1, "team_id": "10", "abbreviation": "TOR", "name": "Maple Leafs", "full_name": "Toronto Maple Leafs", "franchise_id": "5", "is_active": true, "raw_data": { } },
>   "away_team": { "...": "same TeamSerializer shape" },
>   "home_score": 4,
>   "away_score": 3,
>   "status": "final"
> }
> ```

---

### NHL standings

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/standings/` | List standing rows (paginated). |
| GET | `/api/v1/nhl/standings/{id}/` | Retrieve one standing row by numeric id. |

**Query parameters (list):** `date` (`YYYY-MM-DD`), `ordering`, `page`.
**`ordering` fields:** `date`, `points`, `point_pct`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/standings/"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty at capture time.** The NHL standings sync was rate-limited (HTTP 429)
> by the upstream NHL API during capture, and standings are off-season anyway. In
> normal operation each item has the shape:
>
> ```json
> {
>   "id": 1,
>   "team": { "id": 1, "team_id": "10", "abbreviation": "TOR", "name": "Maple Leafs", "full_name": "Toronto Maple Leafs", "franchise_id": "5", "is_active": true, "raw_data": { } },
>   "date": "2024-03-15",
>   "games_played": 70,
>   "wins": 42,
>   "losses": 20,
>   "ot_losses": 8,
>   "points": 92,
>   "point_pct": 0.657,
>   "division_name": "Atlantic",
>   "conference_name": "Eastern"
> }
> ```

---

### NHL skater stats

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/skater-stats/` | List skater season stat lines (paginated). |
| GET | `/api/v1/nhl/skater-stats/{id}/` | Retrieve one stat line by numeric id. |

**Query parameters (list):** `season` (e.g. `20232024`), `ordering`, `page`.
**`ordering` fields:** `points`, `goals`, `assists`, `plus_minus`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/skater-stats/?ordering=-points"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty at capture time.** No skater season stats were ingested (offseason).
> Each item, when present, has the shape:
>
> ```json
> {
>   "id": 1,
>   "player": { "id": 95, "player_id": "8478427", "first_name": "Sebastian", "last_name": "Aho", "full_name": "Sebastian Aho", "sweater_number": "20", "position": "C", "current_team": { }, "is_active": true, "headshot_url": "..." },
>   "season": "20232024",
>   "games_played": 82,
>   "goals": 36,
>   "assists": 53,
>   "points": 89,
>   "plus_minus": 12,
>   "raw_data": { }
> }
> ```

---

### NHL goalie stats

| Method | Path | Description |
|---|---|---|
| GET | `/api/v1/nhl/goalie-stats/` | List goalie season stat lines (paginated). |
| GET | `/api/v1/nhl/goalie-stats/{id}/` | Retrieve one stat line by numeric id. |

**Query parameters (list):** `season` (e.g. `20232024`), `ordering`, `page`.
**`ordering` fields:** `wins`, `save_pct`, `gaa`, `shutouts`.
**Path parameters:** `id` (integer).

```bash
curl -H "X-API-Key: your-api-key" "http://localhost:8080/api/v1/nhl/goalie-stats/?ordering=-wins"
```

```json
{
  "count": 0,
  "next": null,
  "previous": null,
  "results": []
}
```

> **Empty at capture time.** No goalie season stats were ingested (offseason).
> Each item, when present, has the shape:
>
> ```json
> {
>   "id": 1,
>   "player": { "id": 120, "player_id": "8479394", "first_name": "...", "last_name": "...", "full_name": "...", "sweater_number": "31", "position": "G", "current_team": { }, "is_active": true, "headshot_url": "..." },
>   "season": "20232024",
>   "games_played": 60,
>   "wins": 38,
>   "losses": 18,
>   "save_pct": 0.921,
>   "gaa": 2.35,
>   "shutouts": 5,
>   "raw_data": { }
> }
> ```

---

## Ingest endpoints — `POST /api/v1/ingest/*`

Gated by `SERVICES_ESPN_ENABLED`. These synchronously fetch from ESPN and upsert
into the database, returning a counts summary. Use them to back-fill on demand
(the worker cron does the same on a schedule).

**Request body** (JSON):

| Endpoint | Body fields |
|---|---|
| `POST /api/v1/ingest/scoreboard` | `sport` (req), `league` (req), `date` (opt, `YYYYMMDD` — 8 digits) |
| `POST /api/v1/ingest/teams` | `sport` (req), `league` (req) |
| `POST /api/v1/ingest/news` | `sport` (req), `league` (req), `limit` (opt, int `1..200`, default `50`) |
| `POST /api/v1/ingest/injuries` | `sport` (req), `league` (req) |
| `POST /api/v1/ingest/transactions` | `sport` (req), `league` (req) |

`sport` and `league` are trimmed and lowercased; blank values are rejected with
`400 { "error": "validation.failed" }`. Note the scoreboard `date` uses the
compact `YYYYMMDD` form (e.g. `20241215`), **not** `YYYY-MM-DD`.

**Response body** (`200 OK`) — mirrors Django's `IngestionResultSerializer`:

| Field | Type | Meaning |
|---|---|---|
| `created` | int | Rows inserted. |
| `updated` | int | Rows updated (idempotent upsert). |
| `errors` | int | Rows that failed to process. |
| `total_processed` | int | `created + updated`. |
| `details` | string[] | Per-row error/detail messages (often empty). |

An upstream fetch failure returns `502 { "error": "external.service" }`.

### Examples

**Teams:**

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/teams" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"sport":"basketball","league":"nba"}'
```

```json
{ "created": 0, "updated": 30, "errors": 0, "total_processed": 30, "details": [] }
```

**Scoreboard** (optionally scoped to a date):

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/scoreboard" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"sport":"basketball","league":"nba","date":"20260614"}'
```

```json
{ "created": 1, "updated": 0, "errors": 0, "total_processed": 1, "details": [] }
```

**News** (with `limit`):

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/news" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"sport":"basketball","league":"nba","limit":25}'
```

```json
{ "created": 0, "updated": 25, "errors": 0, "total_processed": 25, "details": [] }
```

**Transactions:**

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/transactions" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"sport":"basketball","league":"nba"}'
```

```json
{ "created": 0, "updated": 25, "errors": 0, "total_processed": 25, "details": [] }
```

**Injuries** (offseason — ESPN returned no processable injuries, so `errors` is
non-zero and `total_processed` is 0):

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/injuries" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"sport":"basketball","league":"nba"}'
```

```json
{ "created": 0, "updated": 0, "errors": 27, "total_processed": 0, "details": [] }
```

**Validation error** (missing `sport`):

```bash
curl -XPOST "http://localhost:8080/api/v1/ingest/teams" \
  -H "X-API-Key: your-api-key" -H 'Content-Type: application/json' \
  -d '{"league":"nba"}'
```

```json
{ "error": "validation.failed" }
```

---

## See also

- **Interactive API reference (Scalar UI):** `GET /docs` — available when the
  server runs with `SERVER_ENV=development`. Renders [`docs/openapi.json`](openapi.json).
- **Cutover & rollback runbook:** [`docs/cutover.md`](cutover.md).
- **Endpoint catalog & env reference:** [`README.md`](../README.md).
