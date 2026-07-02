#!/usr/bin/env bash
#
# parity_check.sh — diff the Django ESPN service against the Go rewrite.
#
# Hits the SAME read endpoints on both services and compares the JSON bodies
# with keys sorted (`jq -S`) so ordering differences don't produce false fails.
# Reports per-endpoint PASS/FAIL and exits non-zero if any endpoint diffs.
#
# Usage:
#   DJANGO_BASE=https://espn-0a381a153f.sheka.xyz \
#   GO_BASE=http://localhost:8080 \
#   API_KEY=your-key \
#   scripts/parity_check.sh
#
# Optional:
#   EVENT_ID=<numeric id>   # override the by-id event probe (default: first id
#                           #   discovered from the Go /events list)
#   KEEP_TMP=1              # keep the per-endpoint response files for inspection
#
# Requires: bash, curl, jq. Does NOT need the services reachable at author time.

set -uo pipefail

: "${DJANGO_BASE:?set DJANGO_BASE (e.g. https://espn-0a381a153f.sheka.xyz)}"
: "${GO_BASE:?set GO_BASE (e.g. http://localhost:8080)}"
: "${API_KEY:?set API_KEY}"

command -v jq >/dev/null 2>&1 || { echo "jq is required" >&2; exit 2; }

TMP="$(mktemp -d)"
trap '[ -n "${KEEP_TMP:-}" ] && echo "responses kept in $TMP" || rm -rf "$TMP"' EXIT

pass=0
fail=0

# fetch <base> <path> <outfile>  -> writes normalized (sorted-key) JSON, or a
# sentinel object carrying the HTTP status when the body is not valid JSON.
fetch() {
  local base="$1" path="$2" out="$3" code
  code="$(curl -sS -o "$TMP/raw" -w '%{http_code}' \
    -H "X-API-Key: $API_KEY" -H "Accept: application/json" \
    "$base$path" 2>/dev/null || echo "000")"
  if jq -S -e . "$TMP/raw" >"$out" 2>/dev/null; then
    echo "$code"
  else
    # Non-JSON (error page, empty, timeout): record status so the diff is meaningful.
    printf '{"__non_json__":true,"__status__":%s}\n' "${code:-000}" | jq -S . >"$out"
    echo "$code"
  fi
}

compare() {
  local label="$1" path="$2"
  local dj="$TMP/dj.json" go="$TMP/go.json"
  local dcode gcode
  dcode="$(fetch "$DJANGO_BASE" "$path" "$dj")"
  gcode="$(fetch "$GO_BASE" "$path" "$go")"

  if diff -q "$dj" "$go" >/dev/null 2>&1; then
    printf 'PASS  %-42s django=%s go=%s\n' "$label" "$dcode" "$gcode"
    pass=$((pass + 1))
  else
    printf 'FAIL  %-42s django=%s go=%s\n' "$label" "$dcode" "$gcode"
    echo "      --- diff (django < , go > ) truncated to 40 lines ---"
    diff "$dj" "$go" | head -40 | sed 's/^/      /'
    fail=$((fail + 1))
  fi
}

echo "Django : $DJANGO_BASE"
echo "Go     : $GO_BASE"
echo "----------------------------------------------------------------------"

# List endpoints (the shapes the sheka ingestor depends on).
compare "sports (list)"            "/api/v1/sports/"
compare "events nba by date"       "/api/v1/events/?league=nba&ordering=date"
compare "news nba"                 "/api/v1/news/?league=nba"
compare "injuries nfl"             "/api/v1/injuries/?league=nfl"

# By-id probe: use EVENT_ID if given, else discover the first id from Go's list.
event_id="${EVENT_ID:-}"
if [ -z "$event_id" ]; then
  event_id="$(curl -sS -H "X-API-Key: $API_KEY" \
    "$GO_BASE/api/v1/events/?league=nba&ordering=date" 2>/dev/null \
    | jq -r '.results[0].id // empty' 2>/dev/null)"
fi
if [ -n "$event_id" ]; then
  compare "event by id ($event_id)" "/api/v1/events/$event_id/"
else
  echo "SKIP  event by id                          (no id discovered; set EVENT_ID)"
fi

echo "----------------------------------------------------------------------"
echo "PASS=$pass FAIL=$fail"
[ "$fail" -eq 0 ]
