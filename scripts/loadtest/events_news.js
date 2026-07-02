// k6 load test for Go-ESPN-API — exercises the two hottest read endpoints the
// sheka backend polls: GET /api/v1/events/ and GET /api/v1/news/.
//
// Run (do NOT run against production without coordination):
//   BASE_URL=http://localhost:8080 API_KEY=your-key k6 run scripts/loadtest/events_news.js
//
// Tunables (env):
//   BASE_URL   target base (default http://localhost:8080)
//   API_KEY    X-API-Key value (required)
//   LEAGUE     league filter (default nba)
//   VUS        peak virtual users (default 50)
//   DURATION   steady-state duration (default 1m)
//
// What to record from the summary (and from the host / DB while it runs):
//   - http_req_duration p50 / p95 / p99   (k6 prints these)
//   - http_req_failed rate                 (should be ~0)
//   - process RSS + CPU of the app container:  docker stats espn-app
//   - peak Postgres connections:
//       SELECT count(*) FROM pg_stat_activity WHERE datname = current_database();
//   Compare these against the Django deployment under the same profile to
//   demonstrate the performance goal.

import http from 'k6/http';
import { check, sleep } from 'k6';

const BASE_URL = __ENV.BASE_URL || 'http://localhost:8080';
const API_KEY = __ENV.API_KEY;
const LEAGUE = __ENV.LEAGUE || 'nba';
const VUS = parseInt(__ENV.VUS || '50', 10);
const DURATION = __ENV.DURATION || '1m';

export const options = {
  scenarios: {
    ramp: {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '15s', target: VUS },
        { duration: DURATION, target: VUS },
        { duration: '10s', target: 0 },
      ],
      gracefulRampDown: '5s',
    },
  },
  thresholds: {
    http_req_failed: ['rate<0.01'],       // <1% errors
    http_req_duration: ['p(95)<300'],     // p95 under 300ms (tune to your infra)
  },
};

const params = {
  headers: {
    'X-API-Key': API_KEY,
    Accept: 'application/json',
  },
};

export default function () {
  const events = http.get(
    `${BASE_URL}/api/v1/events/?league=${LEAGUE}&ordering=date`,
    params,
  );
  check(events, {
    'events 200': (r) => r.status === 200,
    'events json': (r) => (r.headers['Content-Type'] || '').includes('json'),
  });

  const news = http.get(`${BASE_URL}/api/v1/news/?league=${LEAGUE}`, params);
  check(news, {
    'news 200': (r) => r.status === 200,
  });

  sleep(1);
}
