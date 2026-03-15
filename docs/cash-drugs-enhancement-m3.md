# cash-drugs Integration Notes — M3

**Date:** 2026-03-09
**Status:** No changes needed

## Summary

cash-drugs **already has all the data drug-gate needs** for M3. Both `drugnames` and `drugclasses` slugs are configured and return full datasets from DailyMed, pre-downloaded into MongoDB. The `fda-ndc` endpoint supports `GENERIC_NAME`, `BRAND_NAME`, and `PHARM_CLASS` search parameters.

**No cash-drugs changes required.**

## Available Endpoints

### `GET /api/cache/drugnames`

Returns the complete DailyMed drug names dataset (~104K entries, ~7MB).

**Response shape:**
```json
{
  "data": [
    {"name_type": "G", "drug_name": "simvastatin"},
    {"name_type": "B", "drug_name": "Zocor"}
  ],
  "meta": {
    "slug": "drugnames",
    "source_url": "https://dailymed.nlm.nih.gov/dailymed/services/v2/drugnames",
    "fetched_at": "2026-03-09T10:56:34Z",
    "page_count": 1045,
    "stale": true,
    "stale_reason": "ttl_expired"
  }
}
```

**Fields per entry:**
- `drug_name` — drug name string
- `name_type` — `"G"` (generic) or `"B"` (brand)

### `GET /api/cache/drugclasses`

Returns the complete DailyMed drug classes dataset (~1,216 entries). Response shape TBD (needs verification — likely same `data`/`meta` wrapper).

### `GET /api/cache/fda-ndc`

Existing endpoint. Supports search by `NDC`, `GENERIC_NAME`, `BRAND_NAME`, `PHARM_CLASS`. Returns `pharm_class` array.

- `?GENERIC_NAME={name}` — lookup by generic name
- `?BRAND_NAME={name}` — lookup by brand name
- `?PHARM_CLASS={class}` — lookup drugs in a pharmacological class

## drug-gate Architecture for M3

cash-drugs dumps full datasets in a single response. drug-gate caches this data in Redis on first request and serves filtered/paginated queries from its own cache.

### Data Flow

```
Frontend request arrives
    │
    ▼
drug-gate checks Redis cache
    │
    ├── HIT → reset 60-min TTL → filter/paginate → respond
    │
    └── MISS → fetch from cash-drugs → store in Redis (60-min TTL) → filter/paginate → respond

Upstream sources:
    GET /api/cache/drugnames      (full dump, ~104K entries)
    GET /api/cache/drugclasses    (full dump, ~1.2K entries)
    GET /api/cache/fda-ndc?PHARM_CLASS={class}  (per-class lookup)

Frontend APIs:
    GET /v1/drugs/names?q=simva&page=1&limit=50
    GET /v1/drugs/classes?type=epc&page=1&limit=50
    GET /v1/drugs/classes/drugs?class=HMG-CoA+Reductase+Inhibitor
    GET /v1/drugs/class?name=simvastatin
```

### Redis Caching Strategy

**Lazy-load on first request** — data is NOT preloaded at startup. If it's not needed, it doesn't sit in Redis.

| Data | Redis Key | Structure | TTL |
|------|-----------|-----------|-----|
| Drug names | `cache:drugnames` | String (JSON blob) | 60 min sliding |
| Drug classes | `cache:drugclasses` | String (JSON blob) | 60 min sliding |
| Drugs by class | `cache:drugsbyclass:{class}` | String (JSON blob) | 60 min sliding |

**Sliding TTL:** Each cache read resets the 60-minute expiry. If no requests arrive for 60 minutes, the data is evicted. Next request triggers a fresh fetch from cash-drugs.

Filtering (substring search, type filter) and pagination are performed in-memory after loading from Redis.

## Data Volumes

| Dataset | Records | Raw Size | Notes |
|---------|---------|----------|-------|
| Drug names | ~104,448 | ~7MB | All DailyMed names (generic + brand) |
| Drug classes | ~1,216 | ~50KB (est.) | All DailyMed classes (EPC, MoA, PE, CS) |
| FDA NDC | ~132,397 | On-demand | Queried per-class, cached individually |
