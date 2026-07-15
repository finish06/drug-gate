# Decision: U-002 — GetWithStale wiring (or removal)

**Status:** OPEN — needs human decision
**Drafted by:** claude (away session 2026-05-17)
**Backlog item:** PRD §6 backlog, U-002 (Medium)
**Files of interest:** `internal/cache/aside.go:95-153`, all of `internal/service/*.go`

## Current state

`CacheAside[T].GetWithStale` exists and is fully tested (3 dedicated test cases in `aside_test.go`). It writes a no-TTL `stale:{key}` backup alongside the fresh `{key}` write, and on `ErrCircuitOpen` returns the no-TTL backup with `Result{Stale: true}`.

**No service calls it.** All 11 cache callers (drugdata.go × 3, rxnorm.go × 5, spl.go × 3) use the regular `Get`, which propagates `ErrCircuitOpen` straight to the handler → 502.

So when the circuit is open:
- Today: every cached endpoint returns 502 from the first miss onward until the upstream recovers and the breaker half-opens successfully.
- With GetWithStale wired up: cached endpoints can serve last-known-good data (no TTL on the backup, so it survives indefinitely), with a `Result{Stale: true}` signal the handler could surface as `X-Cache-Status: stale` or similar.

## How often does this matter?

Breaker config: 10 consecutive failures → 30s cooldown → half-open probe. In healthy operation, the breaker is closed and `GetWithStale` is indistinguishable from `Get` (same fresh-cache path, plus one extra Redis write per cache miss for the no-TTL backup).

When the breaker is open, it's typically open for at least 30 seconds — that's the cooldown window before a probe attempt. Real upstream outages last longer (minutes to hours). During that window, the fallback turns 502s into stale-but-useful responses for every endpoint with cached data.

For drug-gate's data model — drug names, classes, RxNorm codes, SPL interactions — **stale is almost always fine**. The FDA doesn't push intra-day updates. Yesterday's interaction data is the same as today's. The "freshness" of cached drug data is a non-issue compared to the cost of returning 502 to clinicians/apps that need any answer.

## Cost of the wiring

The handlers and services don't deal in `Result[T]` today — they deal in `T` or `*T`. To wire GetWithStale up, you have three shapes:

### Shape 1: Change service signatures to return `(Result[T], error)`

**Effort:** Touch every service method that wraps a CacheAside call (11 methods) + their handler callers. Each handler decides whether to surface staleness (e.g., set `X-Cache-Status: stale` response header, add `stale: true` to response JSON).

**Pros:** Honest end-to-end. Clients can opt in to caching decisions ("I'd rather get stale than nothing").

**Cons:** Spreads "do you care if it's stale?" plumbing through every endpoint. Adds a response field that has to be documented in Swagger.

### Shape 2: Wrap GetWithStale internally, drop the Stale flag silently

**Effort:** Add a `GetOrStale` helper inside `internal/cache` that calls `GetWithStale` and returns just `(T, error)`, swallowing the staleness signal. Swap `.Get(...)` → `.GetOrStale(...)` in services.

**Pros:** Minimal churn. Existing service / handler signatures unchanged. Behaviour change is invisible to callers — they just stop getting 502s when the breaker is open.

**Cons:** Clients have no way to tell whether they got fresh or stale data. For drug-gate's threat model (read-only public-data API), this is probably fine, but it's a one-way door.

### Shape 3: Per-endpoint opt-in

**Effort:** Pick 2-3 high-blast-radius endpoints (drug names, classes, drug info) and wire just those. Leave RxNorm and SPL search on plain `Get` for now.

**Pros:** Smallest blast radius. Targets the endpoints where stale fallback adds the most uptime value.

**Cons:** Inconsistency — some endpoints survive an upstream outage, others don't, and the difference isn't predictable to callers.

## Options on the table

### Option A — Wire GetWithStale globally via Shape 2 (recommended)

Add `internal/cache/aside.go::GetOrStale(...) (T, error)` that calls `GetWithStale` and returns `(result.Value, err)`. Swap all 11 service callers from `.Get(...)` → `.GetOrStale(...)`. Total diff ~15 lines + ~30 lines of test additions.

**Pros:**
- Every cached endpoint gains stale-fallback for free.
- No external API change. Swagger unchanged. Handlers unchanged.
- One-line decision point: do we expose staleness or not? (We don't.)
- Aligns with how the breaker was originally pitched (M9 success criteria mentions "stale-cache serving" — that was unwired in the original ship).

**Cons:**
- Adds one Redis write per cache miss (the no-TTL `stale:{key}` write). At our miss rate this is negligible — drug-gate's miss rate is low because data is stable.
- The stale-key namespace will grow unbounded over time. Practically this means `stale:cache:drugnames`, `stale:cache:drugclasses`, ~50 `stale:cache:drugsbyclass:*` (one per pharm class), thousands of `stale:cache:rxnorm:*` and `stale:cache:spl:*` keys. At Redis prices for an embedded instance, irrelevant. Worth tracking on the Redis memory dashboard.

### Option B — Wire only the heavyweight global caches (Shape 3)

Wire GetWithStale just for `GetDrugNames` and `GetDrugClasses` (the two singletons that affect every list endpoint + autocomplete). Leave per-drug and per-RxCUI caches on plain `Get`.

**Pros:**
- Even smaller diff (~6 lines).
- Targets the cases where a missing fallback hurts most (autocomplete is unusable without drugnames).

**Cons:**
- Mixed behavior is harder to explain to consumers ("autocomplete works during outage but interactions don't").

### Option C — Remove GetWithStale

Delete `GetWithStale`, `Result[T]`, the stale-key path, and the dedicated tests. ~80 lines removed.

**Pros:**
- Less surface area to maintain.
- Honest about current behavior — nothing actually does stale-cache serving today.

**Cons:**
- Throws away the M9 "stale-cache serving" feature that the PRD success criteria claim is done. (It is done at the cache layer; it's just unwired. Removing it breaks that claim.)
- If we ever do want stale-fallback, we'd rewrite this.

## Recommendation

**Option A.** Reasons:

1. The implementation already exists and is tested. We're choosing to leave 80 lines of working code unused, or to wire it in for ~15 lines of glue.
2. Drug data is exactly the kind of read-mostly, stale-tolerant data this pattern was designed for.
3. The cost (one Redis write per miss) is in the noise for our traffic profile.
4. It closes a documented M9 success criterion that's currently technically-true-but-effectively-false.

Option B is the right call only if you want to test the fallback behavior on a couple of endpoints before committing to all of them — but Option A's blast radius is already tiny since it's a Shape-2 wrap.

Option C is the right call only if you've decided the operational complexity (extra Redis writes, monitoring the stale namespace) isn't worth it. I don't see a strong case for that given current scale.

## Acceptance, if you pick Option A

1. Add `GetOrStale` to `internal/cache/aside.go`:
   ```go
   func (c *CacheAside[T]) GetOrStale(ctx context.Context, fetch func(ctx context.Context) (T, error)) (T, error) {
       result, err := c.GetWithStale(ctx, fetch)
       return result.Value, err
   }
   ```
2. Swap all 11 call sites in `internal/service/*.go` from `.Get(...)` → `.GetOrStale(...)`.
3. Add 2 unit tests: (a) `GetOrStale` drops the Stale flag on fresh hit, (b) returns last-good value when fetch returns `ErrCircuitOpen` after a prior successful write.
4. Add a small operational note to `ops/redis-persistence.md` (if it exists) or runbook: "On `ErrCircuitOpen`, cached endpoints serve from `stale:` no-TTL backup keys. These grow as the working set grows. Monitor Redis memory."
5. No spec needed; this is implementing a previously-declared M9 success criterion.

## Open questions for human

- Q1: Do you want any staleness signal exposed to clients (response header, JSON field), or is Shape 2's silent fallback fine?
- Q2: Should the stale namespace ever be reaped? Today it's no-TTL by design (survive cache expiry). A monthly pruning job — "delete `stale:*` keys whose fresh counterpart hasn't been written in N days" — would prevent unbounded growth, but adds complexity. Defer until we see actual memory pressure?
- Q3: Is there any downstream that should know about stale responses (e.g., the SPL drug-info endpoint feeding a clinical UI that should warn the user)? If yes, Shape 1 instead of Shape 2.
