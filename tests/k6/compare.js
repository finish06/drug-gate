#!/usr/bin/env node
//
// k6 baseline comparison tool
//
// Usage:
//   node tests/k6/compare.js <scenario> <results.json>
//
// Example:
//   k6 run tests/k6/staging.js --env SCENARIO=load --summary-export=/tmp/run.json
//   node tests/k6/compare.js load /tmp/run.json
//
// Exit code 0 = pass, 1 = regression detected

const fs = require('fs');
const path = require('path');

const TOLERANCE = 0.15; // 15% regression tolerance

const scenario = process.argv[2];
const resultsFile = process.argv[3];

if (!scenario || !resultsFile) {
  console.error('Usage: node tests/k6/compare.js <scenario> <results.json>');
  console.error('Scenarios: smoke, load, spike, soak');
  process.exit(2);
}

const baselineFile = path.join(__dirname, 'baselines', `${scenario}.json`);

if (!fs.existsSync(baselineFile)) {
  console.error(`No baseline found: ${baselineFile}`);
  process.exit(2);
}

if (!fs.existsSync(resultsFile)) {
  console.error(`Results file not found: ${resultsFile}`);
  process.exit(2);
}

const baseline = JSON.parse(fs.readFileSync(baselineFile, 'utf8'));
const results = JSON.parse(fs.readFileSync(resultsFile, 'utf8'));

// Metrics to compare (key in summary JSON → display name)
const metricsToCompare = [
  { key: 'http_req_duration', field: 'p(95)', name: 'HTTP p95' },
  { key: 'http_req_duration', field: 'avg', name: 'HTTP avg' },
  { key: 'autocomplete_p95', field: 'p(95)', name: 'Autocomplete p95' },
  { key: 'drug_names_p95', field: 'p(95)', name: 'Drug names p95' },
  { key: 'ndc_lookup_p95', field: 'p(95)', name: 'NDC lookup p95' },
  { key: 'rxnorm_search_p95', field: 'p(95)', name: 'RxNorm search p95' },
  { key: 'spl_search_p95', field: 'p(95)', name: 'SPL search p95' },
  { key: 'error_rate', field: 'value', name: 'Error rate', absolute: true, maxValue: 0.05 },
  { key: 'request_id_present', field: 'value', name: 'X-Request-ID rate', absolute: true, minValue: 0.99 },
];

// Check pass/fail counts
const checksToCompare = [];
if (baseline.root_group && baseline.root_group.checks) {
  for (const [name, check] of Object.entries(baseline.root_group.checks)) {
    checksToCompare.push({ name, baselinePasses: check.passes, baselineFails: check.fails });
  }
}

let regressions = 0;
let passed = 0;
let skipped = 0;

console.log(`\n  k6 BASELINE COMPARISON — ${scenario}`);
console.log(`  Baseline: tests/k6/baselines/${scenario}.json`);
console.log(`  Current:  ${resultsFile}`);
console.log(`  Tolerance: ${TOLERANCE * 100}% regression allowed\n`);
console.log('  ─────────────────────────────────────────────────────────');

// Compare metrics
for (const m of metricsToCompare) {
  const bMetric = baseline.metrics?.[m.key];
  const rMetric = results.metrics?.[m.key];

  if (!bMetric || bMetric[m.field] === undefined) {
    skipped++;
    continue;
  }
  if (!rMetric || rMetric[m.field] === undefined) {
    skipped++;
    continue;
  }

  const bVal = bMetric[m.field];
  const rVal = rMetric[m.field];

  if (m.absolute) {
    // Absolute threshold check
    let ok = true;
    if (m.maxValue !== undefined && rVal > m.maxValue) ok = false;
    if (m.minValue !== undefined && rVal < m.minValue) ok = false;

    const status = ok ? '✓' : '✗';
    if (!ok) regressions++;
    else passed++;

    const constraint = m.maxValue !== undefined ? `max ${m.maxValue}` : `min ${m.minValue}`;
    console.log(`  ${status}  ${m.name.padEnd(25)} ${rVal.toFixed(4).padStart(10)}  (${constraint})`);
    continue;
  }

  // Relative comparison — higher is worse for latency
  const threshold = bVal * (1 + TOLERANCE);
  const diff = bVal > 0 ? ((rVal - bVal) / bVal * 100) : 0;
  const ok = rVal <= threshold;

  const status = ok ? '✓' : '✗';
  if (!ok) regressions++;
  else passed++;

  const diffStr = diff >= 0 ? `+${diff.toFixed(1)}%` : `${diff.toFixed(1)}%`;
  console.log(`  ${status}  ${m.name.padEnd(25)} ${rVal.toFixed(1).padStart(10)}ms  (baseline: ${bVal.toFixed(1)}ms, ${diffStr})`);
}

// Compare checks
if (checksToCompare.length > 0 && results.root_group?.checks) {
  console.log('  ─────────────────────────────────────────────────────────');
  for (const c of checksToCompare) {
    const rCheck = results.root_group.checks[c.name];
    if (!rCheck) {
      console.log(`  ✗  CHECK: ${c.name} — MISSING in current run`);
      regressions++;
      continue;
    }
    if (rCheck.fails > 0) {
      console.log(`  ✗  CHECK: ${c.name} — ${rCheck.fails} failures (baseline: ${c.baselineFails})`);
      regressions++;
    } else {
      passed++;
    }
  }
}

console.log('  ─────────────────────────────────────────────────────────');
console.log(`\n  RESULT: ${passed} passed, ${regressions} regressions, ${skipped} skipped\n`);

if (regressions > 0) {
  console.log('  REGRESSION DETECTED — performance is worse than baseline.\n');
  process.exit(1);
} else {
  console.log('  ALL CLEAR — performance meets or exceeds baseline.\n');
  process.exit(0);
}
