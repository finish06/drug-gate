# Decision: U-004 — Indexer parser choice

**Status:** OPEN — needs human decision
**Drafted by:** claude (away session 2026-05-17)
**Backlog item:** PRD §6 backlog, U-004 (Medium)
**Files of interest:** `internal/spl/indexer.go`, `internal/service/spl.go`, `internal/spl/parser.go`

## The problem

`internal/spl/indexer.go:157` calls `ParseInteractions(xmlData)` — this returns **only Section 7** (Drug Interactions). It writes the result to Redis at `cache:spl:interactions:{drug}` as a `model.SPLDetail`.

`internal/service/spl.go:113-147` (`GetInteractionsForDrug`) calls `ParseSections(xmlData)` — this returns **all four sections** (Contraindications, Warnings, Adverse Reactions, Interactions). It writes to the **same Redis key** as the indexer.

## What I found that wasn't in the backlog write-up

This isn't just a "narrow vs complete" cleanliness issue. **The two writers share a cache key and emit different data shapes.** Consequences:

- If the **indexer** populates the key first (which it tries to do for the top 200 drugs at boot), `GetInteractionsForDrug` returns a `SPLDetail` with empty `Contraindications`, `Warnings`, and `AdverseReactions` slices until the cache TTL expires (60m sliding).
- The `/v1/drugs/info` endpoint surfaces those four sections in its response. So a user hitting `/v1/drugs/info?name=warfarin` could legitimately receive **empty contraindications/warnings/adverse reactions** when warfarin's SPL actually has all four populated — purely because the indexer got there first.
- The `CacheAside[T].Get` path doesn't validate shape, so the inconsistency is silent.

The probability this is currently happening in production is high: the indexer runs on boot, has a 5-minute timeout, and warms 200 drugs. Any of those drugs requested via `/v1/drugs/info` within ~60 minutes after a boot will get the truncated payload.

## Options

### Option A — Make the indexer call `ParseSections` (recommended)

**Change:** `internal/spl/indexer.go:157` → call `ParseSections(xmlData)`, populate all four slices in the `SPLDetail` write.

**Pros:**
- Fixes the data-shape inconsistency at the writer.
- Indexer and service produce identical payloads → no surprise empty sections.
- No cache-key change, no migration concerns for already-cached data (TTL expires it naturally).
- Tiny diff (~5 lines).

**Cons:**
- Slightly more CPU per indexer iteration (3 extra regex passes per drug). Indexer runs on a 5-min budget for 200 drugs, so even 4× the work per drug is well within budget.
- Slightly more bytes in Redis per cached entry (4 sections instead of 1). Per-drug bytes are still small (text payloads, not XML).

**Migration cost:** Zero. Existing cached entries naturally expire in ≤60m; new writes use the new shape immediately. No coordinated flush needed. If you want clean cutover, hit `DELETE /admin/cache` once after deploy.

**Coverage impact:** `indexer_test.go` already exists. Add 1-2 assertions that contraindications/warnings/adverse_reactions are populated when the test XML has them. Probably 30 min of test work.

### Option B — Indexer writes a "narrow" payload to a different key

**Change:** Rename the indexer's cache key to `cache:spl:interactions-narrow:{drug}`. Keep service writing to the original key. Add a fast path in `GetInteractionsForDrug` that checks the narrow key first for partial hits (or just delete the indexer's purpose entirely if narrow data is useless to consumers).

**Pros:**
- Preserves the (notional) original intent — indexer pre-warms only what it needs to.

**Cons:**
- The "original intent" is unclear from git history. The cross-reference flow (`POST /v1/drugs/interactions`) does call `GetInteractionsForDrug` repeatedly — and `GetInteractionsForDrug` writes the full payload. So the indexer's narrow payload is essentially useless once a single user request hits any of those drugs. Net: more complexity, no benefit.
- Cache key migration needed if any tooling/dashboards reference the existing key.

### Option C — Delete the indexer

**Change:** Remove `internal/spl/indexer.go` and its test file. Drop the `splIndexer := spl.NewIndexer(...)` lines in `cmd/server/main.go`.

**Pros:**
- One fewer background goroutine and one fewer source of cache writes.
- Removes the entire class of indexer-vs-service consistency bugs.
- The on-demand `GetInteractionsForDrug` already populates the cache correctly for every drug a user actually asks about.

**Cons:**
- Loses the cold-cache warming on boot. First request for popular drugs pays the full XML fetch+parse latency (~200-500ms). With singleflight + 200 popular drugs, this is a real but bounded warm-up cost.
- Removes a feature that ships in v0.10.0 — visible to anyone watching deploy logs ("indexer: run complete...").
- The k6 baselines may shift slightly if they measured warmed-cache paths.

## Recommendation

**Option A (5-line fix to use ParseSections).** Reasons:

1. Closes a real data-shape inconsistency that's likely already affecting production responses.
2. Lowest churn, smallest diff, zero migration cost.
3. Doesn't remove the indexer's warming behavior (which has real value for the cross-reference flow on boot).
4. Test addition is mechanical.

Option C is the right call only if you've decided the indexer's CPU/Redis cost outweighs its warming value. Given v0.10.0 just shipped with the indexer in place, that's a bigger call than U-004 needs to be.

## What this doc does NOT decide

- Whether to keep the indexer at all (Option C is a separable architectural question).
- Whether to change the cache TTL or the `maxDrugs=200` boot tuning.
- Whether `SPLDetail` should grow more sections (sections 1-3 or 8+ aren't currently parsed — separate concern).

## Acceptance, if you pick Option A

1. Edit `internal/spl/indexer.go:157-165` — replace `ParseInteractions(xmlData)` with a call to `ParseSections(xmlData)` and populate all four `model.SPLDetail` slices from the result.
2. Add 2 assertions to `internal/spl/indexer_test.go` verifying contraindications + warnings populated when the test XML has them.
3. Verify the existing `parser_test.go::TestParseSections_*` tests still pass.
4. Manually `DELETE /admin/cache?prefix=cache:spl:interactions:` once on staging after deploy to flush mixed-shape entries.
5. No spec needed (already covered by M8's "Drug info card includes contraindications, warnings, and adverse reactions" success criterion, which this fully closes for indexer-populated drugs).

## Open questions for human

- Q1: Do you want to fix this U-004 as Option A this cycle, or defer to a broader M-something cleanup?
- Q2: Is there any external consumer (Grafana dashboard, k6 baseline, downstream service) reading the `cache:spl:interactions:*` keys directly, or making assumptions about the data shape? If yes, name it so the migration plan can include it.
