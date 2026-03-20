# k6 Performance Baselines

Captured: 2026-03-20
Build: `beta-716332a6` (M7 Operational Hardening)
Target: staging (192.168.1.145:8082)

## Baseline Summary

### Smoke (1 VU, single pass)
- All 36 checks pass
- HTTP p95: 145ms

### Load (10 VUs, 30s)
- HTTP p95: 351ms
- Autocomplete p95: 365ms
- NDC lookup p95: 91ms
- RxNorm search p95: 68ms
- SPL search p95: 76ms
- Drug names p95: 333ms
- Error rate: 0.00%
- Throughput: ~35 req/s

### Spike (0→25 VUs ramp, 30s)
- HTTP p95: 727ms
- Error rate: 0.00%
- Throughput: ~70 req/s

### Soak (5 VUs, 5 min)
- HTTP p95: 197ms
- Autocomplete p95: 207ms
- NDC lookup p95: 28ms
- RxNorm search p95: 19ms
- SPL search p95: 20ms
- Drug names p95: 195ms
- Error rate: 0.00%
- Throughput: ~16 req/s

## Usage

```bash
# Run a scenario and compare against baseline
k6 run tests/k6/staging.js --env SCENARIO=load --summary-export=/tmp/k6-run.json
node tests/k6/compare.js load /tmp/k6-run.json
```
