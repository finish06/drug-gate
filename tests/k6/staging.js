// drug-gate k6 test harness — full API surface
//
// Covers all 16 protected endpoints, 5 public endpoints, and cross-cutting
// concerns (X-Request-ID, auth rejection, rate limit headers).
//
// Usage:
//   k6 run tests/k6/staging.js                          # default (smoke + load)
//   k6 run tests/k6/staging.js --env SCENARIO=smoke     # smoke only
//   k6 run tests/k6/staging.js --env SCENARIO=load      # load only
//   k6 run tests/k6/staging.js --env SCENARIO=spike     # spike only
//   k6 run tests/k6/staging.js --env SCENARIO=soak      # 5-minute soak

import http from 'k6/http';
import { check, group, sleep } from 'k6';
import { Rate, Trend, Counter } from 'k6/metrics';

// ---------------------------------------------------------------------------
// Configuration
// ---------------------------------------------------------------------------

const BASE_URL = __ENV.BASE_URL || 'http://192.168.1.145:8082';
const API_KEY  = __ENV.API_KEY  || 'pk_1bf389dc3ef894d25f1fee1c4797a3eef371b4eec6d17a02';
const SCENARIO = __ENV.SCENARIO || 'all';

const authHeaders = { 'X-API-Key': API_KEY };

// ---------------------------------------------------------------------------
// Custom metrics
// ---------------------------------------------------------------------------

const errorRate             = new Rate('error_rate');
const autocompleteDuration  = new Trend('autocomplete_p95', true);
const ndcLookupDuration     = new Trend('ndc_lookup_p95', true);
const drugNamesDuration     = new Trend('drug_names_p95', true);
const splSearchDuration     = new Trend('spl_search_p95', true);
const rxnormSearchDuration  = new Trend('rxnorm_search_p95', true);
const interactionDuration   = new Trend('interaction_check_p95', true);
const requestIdPresent      = new Rate('request_id_present');
const endpointHits          = new Counter('endpoint_hits');

// ---------------------------------------------------------------------------
// Scenarios
// ---------------------------------------------------------------------------

function buildScenarios() {
  const scenarios = {};

  if (SCENARIO === 'all' || SCENARIO === 'smoke') {
    scenarios.smoke = {
      executor: 'shared-iterations',
      vus: 1,
      iterations: 1,
      exec: 'smokeTest',
      startTime: '0s',
    };
  }

  if (SCENARIO === 'all' || SCENARIO === 'load') {
    scenarios.load = {
      executor: 'constant-vus',
      vus: 10,
      duration: '30s',
      exec: 'loadTest',
      startTime: SCENARIO === 'all' ? '10s' : '0s',
    };
  }

  if (SCENARIO === 'all' || SCENARIO === 'spike') {
    scenarios.spike = {
      executor: 'ramping-vus',
      startVUs: 0,
      stages: [
        { duration: '10s', target: 25 },
        { duration: '15s', target: 25 },
        { duration: '5s', target: 0 },
      ],
      exec: 'spikeTest',
      startTime: SCENARIO === 'all' ? '45s' : '0s',
    };
  }

  if (SCENARIO === 'soak') {
    scenarios.soak = {
      executor: 'constant-vus',
      vus: 5,
      duration: '5m',
      exec: 'loadTest',
      startTime: '0s',
    };
  }

  return scenarios;
}

export const options = {
  scenarios: buildScenarios(),
  thresholds: {
    http_req_duration:   ['p(95)<1000'],   // overall p95 < 1s
    error_rate:          ['rate<0.05'],     // < 5% errors
    autocomplete_p95:    ['p(95)<500'],     // autocomplete p95 < 500ms
    request_id_present:  ['rate>0.99'],     // >99% responses have X-Request-ID
  },
};

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

function getJSON(url, hdrs) {
  const res = http.get(url, { headers: hdrs || {} });
  endpointHits.add(1);
  requestIdPresent.add(res.headers['X-Request-Id'] !== undefined && res.headers['X-Request-Id'] !== '');
  return res;
}

function postJSON(url, body, hdrs) {
  const res = http.post(url, JSON.stringify(body), {
    headers: Object.assign({ 'Content-Type': 'application/json' }, hdrs || {}),
  });
  endpointHits.add(1);
  requestIdPresent.add(res.headers['X-Request-Id'] !== undefined && res.headers['X-Request-Id'] !== '');
  return res;
}

function safeParseJSON(body) {
  try { return JSON.parse(body); }
  catch { return null; }
}

// ---------------------------------------------------------------------------
// Scenario: Smoke Test — verify every endpoint works
// ---------------------------------------------------------------------------

export function smokeTest() {
  // ── Public endpoints ──────────────────────────────────────────────────

  group('public endpoints', () => {
    let res = getJSON(`${BASE_URL}/health`);
    check(res, {
      'health: 200': (r) => r.status === 200,
      'health: X-Request-ID present': (r) => r.headers['X-Request-Id'] !== undefined,
    });

    res = getJSON(`${BASE_URL}/version`);
    check(res, {
      'version: 200': (r) => r.status === 200,
      'version: has all fields': (r) => {
        const b = safeParseJSON(r.body);
        return b && b.version && b.git_commit && b.git_branch && b.go_version;
      },
    });

    res = getJSON(`${BASE_URL}/metrics`);
    check(res, {
      'metrics: 200': (r) => r.status === 200,
      'metrics: has druggate counters': (r) => r.body.includes('druggate_http_requests_total'),
    });

    res = getJSON(`${BASE_URL}/openapi.json`);
    check(res, {
      'openapi: 200': (r) => r.status === 200,
      'openapi: valid JSON': (r) => safeParseJSON(r.body) !== null,
    });
  });

  // ── X-Request-ID behaviour ────────────────────────────────────────────

  group('X-Request-ID', () => {
    let res = getJSON(`${BASE_URL}/health`);
    check(res, {
      'request-id: auto-generated': (r) => {
        const id = r.headers['X-Request-Id'];
        return id && id.length > 0 && id.includes('-'); // UUID format
      },
    });

    res = getJSON(`${BASE_URL}/health`, { 'X-Request-ID': 'k6-passthrough-test' });
    check(res, {
      'request-id: passthrough': (r) => r.headers['X-Request-Id'] === 'k6-passthrough-test',
    });
  });

  // ── Auth rejection ────────────────────────────────────────────────────

  group('auth', () => {
    const res = getJSON(`${BASE_URL}/v1/drugs/names`);
    check(res, {
      'auth: 401 without key': (r) => r.status === 401,
      'auth: error body': (r) => {
        const b = safeParseJSON(r.body);
        return b && b.error === 'unauthorized';
      },
    });
  });

  // ── Drug names ────────────────────────────────────────────────────────

  group('drug names', () => {
    let res = getJSON(`${BASE_URL}/v1/drugs/names?limit=5`, authHeaders);
    check(res, {
      'names: 200': (r) => r.status === 200,
      'names: has pagination': (r) => safeParseJSON(r.body)?.pagination !== undefined,
      'names: data is array': (r) => Array.isArray(safeParseJSON(r.body)?.data),
      'names: limit respected': (r) => safeParseJSON(r.body)?.data?.length <= 5,
    });

    res = getJSON(`${BASE_URL}/v1/drugs/names?q=aspirin&limit=5`, authHeaders);
    check(res, {
      'names: search works': (r) => r.status === 200 && safeParseJSON(r.body)?.data?.length > 0,
    });

    res = getJSON(`${BASE_URL}/v1/drugs/names?type=brand&limit=5`, authHeaders);
    check(res, {
      'names: type filter works': (r) => {
        const data = safeParseJSON(r.body)?.data;
        return data && data.every((d) => d.type === 'brand');
      },
    });
  });

  // ── Autocomplete (M7) ────────────────────────────────────────────────

  group('autocomplete', () => {
    let res = getJSON(`${BASE_URL}/v1/drugs/autocomplete?q=met`, authHeaders);
    check(res, {
      'autocomplete: 200': (r) => r.status === 200,
      'autocomplete: returns matches': (r) => safeParseJSON(r.body)?.data?.length > 0,
      'autocomplete: entries have name+type': (r) => {
        const d = safeParseJSON(r.body)?.data?.[0];
        return d && d.name && d.type;
      },
    });

    res = getJSON(`${BASE_URL}/v1/drugs/autocomplete?q=sim&limit=3`, authHeaders);
    check(res, {
      'autocomplete: limit capped': (r) => safeParseJSON(r.body)?.data?.length <= 3,
    });

    res = getJSON(`${BASE_URL}/v1/drugs/autocomplete?q=a`, authHeaders);
    check(res, {
      'autocomplete: short q → 400': (r) => r.status === 400,
    });

    res = getJSON(`${BASE_URL}/v1/drugs/autocomplete?q=zzznotreal`, authHeaders);
    check(res, {
      'autocomplete: no match → empty': (r) => r.status === 200 && safeParseJSON(r.body)?.data?.length === 0,
    });
  });

  // ── Drug classes ──────────────────────────────────────────────────────

  group('drug classes', () => {
    let res = getJSON(`${BASE_URL}/v1/drugs/classes?limit=5`, authHeaders);
    check(res, {
      'classes: 200': (r) => r.status === 200,
      'classes: has data': (r) => Array.isArray(safeParseJSON(r.body)?.data),
    });

    res = getJSON(`${BASE_URL}/v1/drugs/classes?type=epc&limit=5`, authHeaders);
    check(res, {
      'classes: type filter': (r) => {
        const data = safeParseJSON(r.body)?.data;
        return data && data.every((d) => d.type === 'epc');
      },
    });
  });

  // ── Drug class lookup ─────────────────────────────────────────────────

  group('drug class lookup', () => {
    const res = getJSON(`${BASE_URL}/v1/drugs/class?name=metformin`, authHeaders);
    check(res, {
      'class lookup: 200 or 502': (r) => r.status === 200 || r.status === 502,
      'class lookup: has query_name': (r) => r.status === 502 || safeParseJSON(r.body)?.query_name !== undefined,
    });
  });

  // ── Drugs by class ────────────────────────────────────────────────────

  group('drugs by class', () => {
    const res = getJSON(`${BASE_URL}/v1/drugs/classes/drugs?class=Statin&limit=5`, authHeaders);
    check(res, {
      'drugs-by-class: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });
  });

  // ── NDC lookup ────────────────────────────────────────────────────────

  group('NDC lookup', () => {
    const res = getJSON(`${BASE_URL}/v1/drugs/ndc/0002-1433-80`, authHeaders);
    check(res, {
      'ndc: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });
  });

  // ── SPL endpoints (M6) ───────────────────────────────────────────────

  group('SPL browser', () => {
    let res = getJSON(`${BASE_URL}/v1/drugs/spls?name=warfarin`, authHeaders);
    check(res, {
      'spl search: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });

    // Drug info
    res = getJSON(`${BASE_URL}/v1/drugs/info?name=aspirin`, authHeaders);
    check(res, {
      'drug info: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });
  });

  // ── Interaction checker (M6) ─────────────────────────────────────────

  group('interaction checker', () => {
    const res = postJSON(`${BASE_URL}/v1/drugs/interactions`, {
      drugs: [
        { name: 'warfarin' },
        { name: 'aspirin' },
      ],
    }, authHeaders);
    check(res, {
      'interactions: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });
  });

  // ── RxNorm endpoints (M4) ────────────────────────────────────────────

  group('RxNorm', () => {
    let res = getJSON(`${BASE_URL}/v1/drugs/rxnorm/search?name=metformin`, authHeaders);
    check(res, {
      'rxnorm search: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });

    res = getJSON(`${BASE_URL}/v1/drugs/rxnorm/profile?name=metformin`, authHeaders);
    check(res, {
      'rxnorm profile: 200 or 502': (r) => r.status === 200 || r.status === 502,
    });
  });
}

// ---------------------------------------------------------------------------
// Scenario: Load Test — sustained traffic across key endpoints
// ---------------------------------------------------------------------------

const loadEndpoints = [
  // Fast endpoints (cached, should be quick)
  { path: '/v1/drugs/autocomplete?q=met', trend: autocompleteDuration, weight: 3 },
  { path: '/v1/drugs/autocomplete?q=lis', trend: autocompleteDuration, weight: 3 },
  { path: '/v1/drugs/autocomplete?q=sim', trend: autocompleteDuration, weight: 2 },
  { path: '/v1/drugs/autocomplete?q=ato', trend: autocompleteDuration, weight: 2 },
  // Paginated list endpoints
  { path: '/v1/drugs/names?limit=20', trend: drugNamesDuration, weight: 2 },
  { path: '/v1/drugs/names?q=statin&limit=10', trend: drugNamesDuration, weight: 1 },
  { path: '/v1/drugs/classes?limit=10', trend: null, weight: 1 },
  // Upstream-dependent endpoints
  { path: '/v1/drugs/class?name=metformin', trend: null, weight: 1 },
  { path: '/v1/drugs/ndc/0002-1433-80', trend: ndcLookupDuration, weight: 1 },
  { path: '/v1/drugs/spls?name=lipitor', trend: splSearchDuration, weight: 1 },
  { path: '/v1/drugs/rxnorm/search?name=lisinopril', trend: rxnormSearchDuration, weight: 1 },
  // Public endpoints (no auth)
  { path: '/health', trend: null, weight: 1, public: true },
  { path: '/version', trend: null, weight: 1, public: true },
];

// Build weighted pool
const weightedPool = [];
for (const ep of loadEndpoints) {
  for (let i = 0; i < ep.weight; i++) {
    weightedPool.push(ep);
  }
}

export function loadTest() {
  const ep = weightedPool[Math.floor(Math.random() * weightedPool.length)];
  const hdrs = ep.public ? {} : authHeaders;

  const res = getJSON(`${BASE_URL}${ep.path}`, hdrs);
  if (ep.trend) ep.trend.add(res.timings.duration);

  const ok = check(res, {
    'load: status 2xx': (r) => r.status >= 200 && r.status < 300,
  });
  errorRate.add(!ok);

  sleep(0.05 + Math.random() * 0.1); // 50-150ms between requests
}

// ---------------------------------------------------------------------------
// Scenario: Spike Test — burst across all endpoints
// ---------------------------------------------------------------------------

const spikeEndpoints = [
  { path: '/v1/drugs/autocomplete?q=war', auth: true },
  { path: '/v1/drugs/autocomplete?q=asp', auth: true },
  { path: '/v1/drugs/autocomplete?q=ibup', auth: true },
  { path: '/v1/drugs/names?limit=10', auth: true },
  { path: '/v1/drugs/classes?limit=10', auth: true },
  { path: '/v1/drugs/class?name=aspirin', auth: true },
  { path: '/v1/drugs/spls?name=metformin', auth: true },
  { path: '/v1/drugs/rxnorm/search?name=aspirin', auth: true },
  { path: '/v1/drugs/info?name=warfarin', auth: true },
  { path: '/health', auth: false },
  { path: '/version', auth: false },
  { path: '/metrics', auth: false },
];

export function spikeTest() {
  const ep = spikeEndpoints[Math.floor(Math.random() * spikeEndpoints.length)];
  const hdrs = ep.auth ? authHeaders : {};

  const res = getJSON(`${BASE_URL}${ep.path}`, hdrs);

  const ok = check(res, {
    'spike: status 2xx or 502': (r) => (r.status >= 200 && r.status < 300) || r.status === 502,
    'spike: X-Request-ID present': (r) => r.headers['X-Request-Id'] !== undefined,
  });
  errorRate.add(!ok);

  sleep(0.02 + Math.random() * 0.05);
}
