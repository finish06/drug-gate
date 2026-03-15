# drug-gate Sequence Diagrams

## Middleware Chain Overview

All `/v1/*` routes pass through the following middleware chain in order:

1. **RequestLogger** -- logs method, path, status, duration
2. **APIKeyAuth** -- validates X-API-Key header against Redis store
3. **PerKeyCORS** -- sets CORS headers based on the key's allowed origins
4. **RateLimit** -- enforces per-key rate limits via Redis sliding window

Admin `/admin/*` routes use a separate chain:

1. **RequestLogger** -- same global logger
2. **AdminAuth** -- validates Bearer token against ADMIN_SECRET env var

Public routes (`/health`, `/swagger/*`, `/openapi.json`) only pass through **RequestLogger**.

---

## Health Check

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant GW as drug-gate<br/>:8081
    participant LOG as RequestLogger
    participant HC as HealthCheck

    Client->>GW: GET /health
    GW->>LOG: Pass request
    LOG->>HC: Next handler
    HC-->>Client: 200 {"status": "ok", "version": "..."}
    LOG->>LOG: Log {method, path, status, duration_ms}
```

---

## NDC Drug Lookup (GET /v1/drugs/ndc/{ndc})

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant GW as drug-gate<br/>:8081
    participant LOG as RequestLogger
    participant AUTH as APIKeyAuth
    participant RDS as Redis<br/>(API Key Store)
    participant CORS as PerKeyCORS
    participant RL as RateLimit
    participant RDL as Redis<br/>(Rate Limiter)
    participant DH as DrugHandler
    participant NDC as NDC Parser
    participant DC as HTTPDrugClient
    participant CD as cash-drugs<br/>:8083

    Client->>GW: GET /v1/drugs/ndc/{ndc}<br/>X-API-Key: pk_...
    GW->>LOG: Pass request
    LOG->>AUTH: Next handler

    AUTH->>AUTH: Extract X-API-Key header

    alt Missing API key
        AUTH-->>Client: 401 {"error": "unauthorized", "message": "API key required"}
    end

    AUTH->>RDS: Get(ctx, key)

    alt Invalid / unknown key
        RDS-->>AUTH: nil, nil
        AUTH-->>Client: 401 {"error": "unauthorized", "message": "Invalid API key"}
    end

    alt Inactive key
        RDS-->>AUTH: APIKey{Active: false}
        AUTH-->>Client: 401 {"error": "unauthorized", "message": "API key is inactive"}
    end

    alt Expired key (past grace period)
        RDS-->>AUTH: APIKey{ExpiresAt: past}
        AUTH-->>Client: 401 {"error": "unauthorized", "message": "API key has expired"}
    end

    RDS-->>AUTH: APIKey (valid)
    AUTH->>AUTH: Set APIKey in context

    AUTH->>CORS: Next handler
    CORS->>CORS: Check Origin against APIKey.Origins
    CORS->>CORS: Set CORS headers if allowed

    CORS->>RL: Next handler
    RL->>RDL: Allow(ctx, key, limit)

    alt Rate limit exceeded
        RDL-->>RL: Result{Allowed: false}
        RL-->>Client: 429 {"error": "rate_limited"}<br/>Retry-After, X-RateLimit-Remaining
    end

    RDL-->>RL: Result{Allowed: true, Remaining: N}
    RL->>RL: Set X-RateLimit-Remaining, X-RateLimit-Reset headers

    RL->>DH: HandleNDCLookup(w, r)
    DH->>NDC: Parse(raw)

    alt Invalid NDC
        NDC-->>DH: error
        DH-->>Client: 400 {"error": "invalid_ndc"}
    end

    NDC-->>DH: ProductNDC{Labeler, Product, Format}

    DH->>DC: LookupByNDC(ctx, "labeler-product")
    DC->>CD: GET /api/cache/fda-ndc?NDC={ndc}

    alt Upstream unreachable / 5xx
        CD-->>DC: error / non-200
        DC-->>DH: ErrUpstream
        DH-->>Client: 502 {"error": "upstream_error"}
    end

    alt Exact match found
        CD-->>DC: 200 {"data": [{...}]}
        DC-->>DH: &DrugResult
        DH-->>Client: 200 DrugDetailResponse
    end

    alt No exact match (fallback)
        CD-->>DC: 404 / empty data
        DC-->>DH: nil, nil
        DH->>NDC: FallbackNDC()
        DH->>DC: LookupByNDC(ctx, "padded-ndc")
        DC->>CD: GET /api/cache/fda-ndc?NDC={padded}

        alt Fallback found
            CD-->>DC: 200
            DH-->>Client: 200 DrugDetailResponse
        end

        alt Not found
            DH-->>Client: 404 {"error": "not_found"}
        end
    end

    LOG->>LOG: Log {method, path, status, duration_ms}
```

---

## Drug Class Lookup by Name (GET /v1/drugs/class?name=)

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant MW as Middleware Chain<br/>(Logger→Auth→CORS→RateLimit)
    participant DCH as DrugClassHandler
    participant DC as HTTPDrugClient
    participant PH as pharma.Parse
    participant CD as cash-drugs<br/>:8083

    Client->>MW: GET /v1/drugs/class?name=atorvastatin<br/>X-API-Key: pk_...
    MW->>MW: Auth + CORS + RateLimit (see NDC flow)

    alt Missing name param
        MW->>DCH: HandleDrugClassLookup(w, r)
        DCH-->>Client: 400 {"error": "validation_error", "message": "name query parameter is required"}
    end

    MW->>DCH: HandleDrugClassLookup(w, r)

    DCH->>DC: LookupByGenericName(ctx, "atorvastatin")
    DC->>CD: GET /api/cache/fda-ndc?GENERIC_NAME=atorvastatin

    alt Upstream error
        CD-->>DC: error / non-200
        DC-->>DCH: ErrUpstream
        DCH-->>Client: 502 {"error": "upstream_error"}
    end

    alt Generic name found
        CD-->>DC: 200 {"data": [...]}
        DC-->>DCH: []DrugResult
    end

    alt Generic name not found — fallback to brand name
        CD-->>DC: 404 / empty data
        DC-->>DCH: empty slice
        DCH->>DC: LookupByBrandName(ctx, "atorvastatin")
        DC->>CD: GET /api/cache/fda-ndc?BRAND_NAME=atorvastatin

        alt Brand name found
            CD-->>DC: 200 {"data": [...]}
            DC-->>DCH: []DrugResult
        end

        alt Not found at all
            DC-->>DCH: empty slice
            DCH-->>Client: 404 {"error": "not_found"}
        end
    end

    DCH->>PH: DeduplicateBrandNames(brandNames)
    PH-->>DCH: deduplicated, title-cased brand names
    DCH->>PH: ParsePharmClasses(pharmClass)
    PH-->>DCH: []PharmClass{Name, Type}

    DCH-->>Client: 200 DrugClassResponse{query_name, generic_name, brand_names, classes}
```

---

## Drug Names Listing (GET /v1/drugs/names)

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant MW as Middleware Chain<br/>(Logger→Auth→CORS→RateLimit)
    participant DNH as DrugNamesHandler
    participant SVC as DrugDataService
    participant RDS as Redis<br/>(Data Cache)
    participant DC as HTTPDrugClient
    participant CD as cash-drugs<br/>:8083

    Client->>MW: GET /v1/drugs/names?type=brand&q=lipitor&page=1&limit=50<br/>X-API-Key: pk_...
    MW->>MW: Auth + CORS + RateLimit (see NDC flow)
    MW->>DNH: HandleDrugNames(w, r)

    DNH->>SVC: GetDrugNames(ctx)
    SVC->>RDS: GET cache:drugnames

    alt Cache hit
        RDS-->>SVC: cached JSON
        SVC->>RDS: EXPIRE cache:drugnames 60m (sliding TTL)
        SVC-->>DNH: []DrugNameEntry
    end

    alt Cache miss
        RDS-->>SVC: nil
        SVC->>DC: FetchDrugNames(ctx)
        DC->>CD: GET /api/cache/drugnames
        CD-->>DC: 200 {"data": [...]}
        DC-->>SVC: []DrugNameRaw
        SVC->>SVC: Transform name_type "B"→"brand", else→"generic"
        SVC->>RDS: SET cache:drugnames {json} EX 60m
        SVC-->>DNH: []DrugNameEntry
    end

    alt Upstream error (cache miss path)
        CD-->>DC: error / non-200
        DC-->>SVC: ErrUpstream
        SVC-->>DNH: ErrUpstream
        DNH-->>Client: 502 {"error": "upstream_error"}
    end

    DNH->>DNH: Filter by ?type= (brand/generic)
    DNH->>DNH: Filter by ?q= (substring search)
    DNH->>DNH: Paginate (page, limit; default 50, max 100)
    DNH-->>Client: 200 {"data": [...], "pagination": {page, limit, total, total_pages}}
```

---

## Drug Classes Listing (GET /v1/drugs/classes)

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant MW as Middleware Chain<br/>(Logger→Auth→CORS→RateLimit)
    participant DCH as DrugClassesHandler
    participant SVC as DrugDataService
    participant RDS as Redis<br/>(Data Cache)
    participant DC as HTTPDrugClient
    participant CD as cash-drugs<br/>:8083

    Client->>MW: GET /v1/drugs/classes?type=epc&page=1&limit=50<br/>X-API-Key: pk_...
    MW->>MW: Auth + CORS + RateLimit (see NDC flow)
    MW->>DCH: HandleDrugClasses(w, r)

    DCH->>SVC: GetDrugClasses(ctx)
    SVC->>RDS: GET cache:drugclasses

    alt Cache hit
        RDS-->>SVC: cached JSON
        SVC->>RDS: EXPIRE cache:drugclasses 60m (sliding TTL)
        SVC-->>DCH: []DrugClassEntry
    end

    alt Cache miss
        RDS-->>SVC: nil
        SVC->>DC: FetchDrugClasses(ctx)
        DC->>CD: GET /api/cache/drugclasses
        CD-->>DC: 200 {"data": [...]}
        DC-->>SVC: []DrugClassRaw
        SVC->>SVC: Transform class_type to lowercase
        SVC->>RDS: SET cache:drugclasses {json} EX 60m
        SVC-->>DCH: []DrugClassEntry
    end

    alt Upstream error
        DC-->>SVC: ErrUpstream
        DCH-->>Client: 502 {"error": "upstream_error"}
    end

    DCH->>DCH: Filter by ?type= (default: "epc"; or "all")
    DCH->>DCH: Paginate (page, limit; default 50, max 100)
    DCH-->>Client: 200 {"data": [...], "pagination": {page, limit, total, total_pages}}
```

---

## Drugs by Class Listing (GET /v1/drugs/classes/drugs?class=)

```mermaid
sequenceDiagram
    actor Client as Frontend Client
    participant MW as Middleware Chain<br/>(Logger→Auth→CORS→RateLimit)
    participant DBH as DrugsByClassHandler
    participant SVC as DrugDataService
    participant RDS as Redis<br/>(Data Cache)
    participant DC as HTTPDrugClient
    participant CD as cash-drugs<br/>:8083

    Client->>MW: GET /v1/drugs/classes/drugs?class=Statin&page=1&limit=100<br/>X-API-Key: pk_...
    MW->>MW: Auth + CORS + RateLimit (see NDC flow)
    MW->>DBH: HandleDrugsByClass(w, r)

    alt Missing class param
        DBH-->>Client: 400 {"error": "validation_error", "message": "class query parameter is required"}
    end

    DBH->>SVC: GetDrugsByClass(ctx, "Statin")
    SVC->>RDS: GET cache:drugsbyclass:statin

    alt Cache hit
        RDS-->>SVC: cached JSON
        SVC->>RDS: EXPIRE cache:drugsbyclass:statin 60m (sliding TTL)
        SVC-->>DBH: []DrugInClassEntry
    end

    alt Cache miss
        RDS-->>SVC: nil
        SVC->>DC: LookupByPharmClass(ctx, "Statin")
        DC->>CD: GET /api/cache/fda-ndc?PHARM_CLASS=Statin
        CD-->>DC: 200 {"data": [...]}
        DC-->>SVC: []DrugResult
        SVC->>SVC: Transform to []DrugInClassEntry{generic_name, brand_name}
        SVC->>RDS: SET cache:drugsbyclass:statin {json} EX 60m
        SVC-->>DBH: []DrugInClassEntry
    end

    alt Upstream error
        DC-->>SVC: ErrUpstream
        DBH-->>Client: 502 {"error": "upstream_error"}
    end

    DBH->>DBH: Paginate (page, limit; default 100, max 500)
    DBH-->>Client: 200 {"data": [...], "pagination": {page, limit, total, total_pages}}
```

---

## Admin Key Management

```mermaid
sequenceDiagram
    actor Admin as Admin User
    participant GW as drug-gate<br/>:8081
    participant LOG as RequestLogger
    participant ADM as AdminAuth
    participant AH as AdminHandler
    participant RDS as Redis<br/>(API Key Store)

    Admin->>GW: POST /admin/keys<br/>Authorization: Bearer {secret}
    GW->>LOG: Pass request
    LOG->>ADM: Next handler
    ADM->>ADM: Validate Bearer token against ADMIN_SECRET

    alt Missing / invalid token
        ADM-->>Admin: 401 {"error": "unauthorized", "message": "Admin authorization required"}
    end

    alt Invalid secret
        ADM-->>Admin: 401 {"error": "unauthorized", "message": "Invalid admin secret"}
    end

    ADM->>AH: CreateKey(w, r)
    AH->>AH: Validate {app_name, rate_limit}

    alt Validation failure
        AH-->>Admin: 400 {"error": "validation_error"}
    end

    AH->>RDS: Create(ctx, appName, origins, rateLimit)
    RDS->>RDS: GenerateKey() -> pk_...
    RDS-->>AH: *APIKey
    AH-->>Admin: 201 {key, app_name, origins, rate_limit, active, created_at}

    Note over Admin, RDS: Other admin endpoints follow the same auth pattern:
    Note over Admin, RDS: GET /admin/keys -- ListKeys (list all keys)
    Note over Admin, RDS: GET /admin/keys/{key} -- GetKey (get single key)
    Note over Admin, RDS: DELETE /admin/keys/{key} -- DeactivateKey (soft delete)
    Note over Admin, RDS: POST /admin/keys/{key}/rotate -- RotateKey (see below)
```

---

## Key Rotation Flow

```mermaid
sequenceDiagram
    actor Admin as Admin User
    participant ADM as AdminAuth
    participant AH as AdminHandler
    participant RDS as Redis

    Admin->>ADM: POST /admin/keys/{old_key}/rotate<br/>Authorization: Bearer {secret}<br/>{"grace_period": "24h"}
    ADM->>ADM: Validate Bearer token
    ADM->>AH: RotateKey(w, r)
    AH->>AH: Parse grace_period duration

    alt Invalid duration
        AH-->>Admin: 400 {"error": "validation_error", "message": "Invalid grace_period duration"}
    end

    AH->>RDS: Rotate(ctx, oldKey, 24h)
    RDS->>RDS: Set old key ExpiresAt = now + 24h
    RDS->>RDS: Create new key (same app_name, origins, rate_limit)
    RDS-->>AH: *APIKey (new)
    AH->>RDS: Get(ctx, oldKey) -- read ExpiresAt
    AH-->>Admin: 200 {old_key, new_key, old_key_expires_at}

    Note over Admin, RDS: During grace period, both old and<br/>new keys work. After ExpiresAt,<br/>old key is rejected by APIKeyAuth.
```

---

## System Overview

```mermaid
sequenceDiagram
    actor Dev as Developer
    participant SW as Swagger UI
    participant GW as drug-gate<br/>:8081
    participant RD as Redis
    participant CD as cash-drugs<br/>:8083

    Dev->>GW: GET /swagger/index.html
    GW-->>SW: Swagger UI HTML
    Dev->>GW: GET /openapi.json
    GW-->>Dev: OpenAPI spec JSON

    Dev->>GW: GET /v1/drugs/ndc/00069-3150<br/>X-API-Key: pk_abc123
    GW->>RD: Validate API key
    RD-->>GW: Key valid, limit=100
    GW->>RD: Check rate limit
    RD-->>GW: Allowed (95 remaining)
    GW->>CD: GET /api/cache/fda-ndc?NDC=00069-3150
    CD-->>GW: {"data": [{product_ndc, brand_name, ...}]}
    GW-->>Dev: 200 {"ndc": "00069-3150", "name": "Lipitor", ...}

    Dev->>GW: GET /v1/drugs/names?type=brand&q=lip<br/>X-API-Key: pk_abc123
    GW->>RD: Validate key + rate limit
    GW->>RD: GET cache:drugnames
    alt Cache miss
        GW->>CD: GET /api/cache/drugnames
        CD-->>GW: {"data": [...]}
        GW->>RD: SET cache:drugnames (TTL 60m)
    end
    GW-->>Dev: 200 {"data": [...], "pagination": {...}}
```

---

## Route Table

| Method | Path | Auth | Handler | Description |
|--------|------|------|---------|-------------|
| GET | `/health` | None | `HealthCheck` | Service health + version |
| GET | `/swagger/*` | None | `httpSwagger.WrapHandler` | Swagger UI |
| GET | `/openapi.json` | None | `OpenAPIJSON` | OpenAPI spec JSON |
| GET | `/v1/drugs/ndc/{ndc}` | API Key | `DrugHandler.HandleNDCLookup` | NDC drug lookup with fallback |
| GET | `/v1/drugs/class` | API Key | `DrugClassHandler.HandleDrugClassLookup` | Drug class lookup by name |
| GET | `/v1/drugs/names` | API Key | `DrugNamesHandler.HandleDrugNames` | Paginated drug names listing |
| GET | `/v1/drugs/classes` | API Key | `DrugClassesHandler.HandleDrugClasses` | Paginated drug classes listing |
| GET | `/v1/drugs/classes/drugs` | API Key | `DrugsByClassHandler.HandleDrugsByClass` | Paginated drugs-by-class listing |
| POST | `/admin/keys` | Admin Bearer | `AdminHandler.CreateKey` | Create API key |
| GET | `/admin/keys` | Admin Bearer | `AdminHandler.ListKeys` | List all API keys |
| GET | `/admin/keys/{key}` | Admin Bearer | `AdminHandler.GetKey` | Get single API key |
| DELETE | `/admin/keys/{key}` | Admin Bearer | `AdminHandler.DeactivateKey` | Deactivate API key |
| POST | `/admin/keys/{key}/rotate` | Admin Bearer | `AdminHandler.RotateKey` | Rotate API key with grace period |
