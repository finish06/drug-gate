# Staging Environment — drug-gate

## Overview

drug-gate staging runs on the same host as cash-drugs staging, connecting over the shared Docker `internal` network. It pulls the `:beta` image on every push to `main`, providing a pre-production validation layer for the API gateway.

## Environment Details

| Property | Value |
|----------|-------|
| **Host** | 192.168.1.145 |
| **Port** | 8082 (mapped from internal 8081) |
| **Base URL** | http://192.168.1.145:8082 |
| **Image** | `dockerhub.calebdunn.tech/finish06/drug-gate:beta` |
| **Redis** | `drug-gate-redis` (alpine, on `internal` network) |
| **Upstream** | `http://cash-drugs:8080` (container DNS on `internal` network) |
| **Docker Network** | `internal` (shared with cash-drugs, newt/pangolin) |
| **Deploy Path** | `/opt/drug-gate/` |

### Environment Comparison

| Setting | Staging | Production |
|---------|---------|------------|
| Host | 192.168.1.145:8082 | drug-gate.calebdunn.tech |
| Image tag | `:beta` (every main push) | `:latest` or `:vX.Y.Z` (release tags) |
| CASHDRUGS_URL | `http://cash-drugs:8080` (container DNS) | `http://host1.du.nn:8083` |
| Redis | `drug-gate-redis` (container) | `redis` (container) |
| ADMIN_SECRET | Set in env | Set in env |

## Initial Setup

### 1. Deploy files to staging host

```bash
# From the drug-gate repo root
scp -i staging-key docker-compose.staging.yml 192.168.1.145:/opt/drug-gate/compose.yaml
```

### 2. Set the admin secret

```bash
ssh -i staging-key 192.168.1.145 "echo 'ADMIN_SECRET=your-staging-secret' > /opt/drug-gate/.env"
```

### 3. Start the stack

```bash
ssh -i staging-key 192.168.1.145 "cd /opt/drug-gate && docker compose pull && docker compose up -d"
```

### 4. Verify

```bash
# Health check
curl http://192.168.1.145:8082/health
# → {"status":"ok","version":"beta-..."}

# Check cash-drugs connectivity (should return drug data, not 502)
ADMIN_SECRET=your-staging-secret
KEY=$(curl -s -X POST http://192.168.1.145:8082/admin/keys \
  -H "Authorization: Bearer $ADMIN_SECRET" \
  -H "Content-Type: application/json" \
  -d '{"app_name":"staging-test","origins":[],"rate_limit":250}' | jq -r .key)

curl -s http://192.168.1.145:8082/v1/drugs/ndc/0069-3150 \
  -H "X-API-Key: $KEY" | jq .

# RxNorm search
curl -s "http://192.168.1.145:8082/v1/drugs/rxnorm/search?name=lipitor" \
  -H "X-API-Key: $KEY" | jq .
```

### 5. (Optional) Warm up cash-drugs cache first

The first drug-gate request will be slow if cash-drugs hasn't cached the upstream data yet:

```bash
# Trigger cash-drugs cache warmup before testing drug-gate
curl -X POST http://192.168.1.145:8083/api/warmup

# Wait for readiness
curl http://192.168.1.145:8083/ready
# → {"status":"ready"} when warm
```

## How to Update Staging

### Pull latest beta image
```bash
ssh -i staging-key 192.168.1.145 "cd /opt/drug-gate && docker compose pull && docker compose up -d"
```

### View logs
```bash
ssh -i staging-key 192.168.1.145 "cd /opt/drug-gate && docker compose logs -f --tail 50"
```

### Full restart (clears Redis cache)
```bash
ssh -i staging-key 192.168.1.145 "cd /opt/drug-gate && docker compose down && docker compose pull && docker compose up -d"
```

### Flush Redis cache only
```bash
ssh -i staging-key 192.168.1.145 "docker exec drug-gate-redis redis-cli FLUSHALL"
```

## Network Topology

```
                    ┌─────────────────────────────────────────┐
                    │          192.168.1.145 (staging)        │
                    │          Docker: internal network        │
                    │                                         │
  :8082 ──────────▶│  drug-gate ──▶ drug-gate-redis           │
                    │      │                                  │
                    │      ▼                                  │
  :8083 ──────────▶│  cash-drugs ──▶ cash-drugs-mongo         │
                    │                                         │
                    └─────────────────────────────────────────┘
```

drug-gate connects to cash-drugs via container DNS (`http://cash-drugs:8080`) on the shared `internal` network. No port mapping needed for inter-container communication.

## Running E2E Tests Against Staging

```bash
DRUG_GATE_URL=http://192.168.1.145:8082 ADMIN_SECRET=your-staging-secret \
  go test -tags=e2e -count=1 -v ./tests/e2e/...
```

This runs the full E2E suite against the staging stack instead of spinning up a local docker-compose.
