# Staging Environment

The staging environment documentation for drug-gate is in [staging-drug-gate.md](staging-drug-gate.md).

## Staging Host Overview

Both drug-gate and cash-drugs staging run on the same host (`192.168.1.145`) sharing the Docker `internal` network. A cron job (`/opt/staging-autopull.sh`) runs every 5 minutes, pulling the latest `:beta` images for all services and restarting any that changed.

| Service | Port | Deploy Path | Image |
|---------|------|-------------|-------|
| **drug-gate** | 8082 | `/opt/drug-gate/` | `dockerhub.calebdunn.tech/finish06/drug-gate:beta` |
| **drug-gate-redis** | (internal) | `/opt/drug-gate/` | `redis:alpine` |
| **cash-drugs** | 8083 | `/opt/cash-drugs/` | `dockerhub.calebdunn.tech/finish06/cash-drugs:beta` |
| **cash-drugs-mongo** | (internal) | `/opt/cash-drugs/` | `mongo:4.4` |

### Auto-Deploy Mechanism

```
Push to main → CI builds :beta image → Registry
                                          ↓
Staging host cron (every 5m) → docker compose pull → restart if changed
```

The cron script is at `/opt/staging-autopull.sh` and covers all staging services (cash-drugs, drug-gate, drugs-quiz).

### SSH Access

```bash
ssh -i staging-key finish06@192.168.1.145
```

The staging key is stored locally at `staging-key` (gitignored — never committed).

### Network Topology

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

### Production Comparison

| Setting | Staging | Production |
|---------|---------|------------|
| drug-gate | 192.168.1.145:8082 | drug-gate.calebdunn.tech |
| cash-drugs | 192.168.1.145:8083 | 192.168.1.86:8083 (host1.du.nn) |
| Image tags | `:beta` (every main push) | `:latest` / `:vX.Y.Z` (release tags) |
| Auto-deploy | Cron every 5m | Manual / CI on tag |
| MongoDB | 4.4 (no AVX) | latest |
