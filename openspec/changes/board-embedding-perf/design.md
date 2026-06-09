# Design: Board Embedding Performance Optimization

## Context

Tag-to-board matching is the most frequently triggered heavy operation in the system. Every event tag that gets embedded automatically triggers `MatchTopicTag`, which:

1. **Loads all board auxiliaries** from `board_composition` + `semantic_labels` (full table scan)
2. **Loads all board embeddings** and calls `parsePgVector` on each (expensive string→float64 conversion)
3. **Loads AI config** (13 rows from `ai_settings`) via `loadConfig` on every call
4. **Loads active auxiliary labels** via `loadActiveAuxiliaryLabels` (another full table scan)
5. Performs O(N×T×B×d) brute-force cosine comparison entirely in Go memory

With ~200+ boards, ~3000+ auxiliary labels, and 2560-dim embeddings, each `MatchTopicTag` call incurs 4 DB queries and ~10ms of vector parsing overhead. During backfill, this is called hundreds of times sequentially.

Additionally, board upgrade only supports discovering new boards. The `collectCandidates` query hard-filters out labels already in `board_composition`, and `filterSemanticBoardUpgradeSuggestions` rejects `merge_into_existing` decisions from LLM. Users who want to expand existing boards have no automated path.

## Goals

1. Eliminate redundant DB loads in `MatchTopicTag` via in-process caching
2. Add "expand existing board" mode to board upgrade
3. Parallelize backfill processing with configurable concurrency
4. Reduce `parsePgVector` redundant calls

## Non-Goals

- Rebuilding HNSW index (2560-dim exceeds pgvector 2000 limit; not feasible)
- Replacing Go-side cosine with SQL-side matching (already benchmarked: SQL-side equally slow for high-dim)
- Introducing Redis or external cache (overkill for single-user system)
- Changing the matching algorithm itself (rules, scoring)
- Auto-triggering board upgrade (remains manual)

## Decisions

### D1: Cache Architecture — `sync.RWMutex` + map with timestamp invalidation

**Decision:** Use a simple Go struct with `sync.RWMutex`, `map`, and `time.Time` per cache entry. No external dependencies.

**Four cache entries:**

| Cache | Key Type | Value | Invalidation |
|-------|----------|-------|-------------|
| Board auxiliaries | `"all"` | `[]boardAuxiliaryLabel` | On board upgrade confirm / board composition change |
| Board embeddings | `"all"` | `map[uint][]float64` | On board upgrade confirm / board composition change |
| AI config | `"config"` | `SemanticBoardMatchConfig` | TTL 5 min |
| Active auxiliary labels | `"auxiliary_labels"` | `[]models.SemanticLabel` | TTL 5 min |

**Why not `sync.Map`**: We need read-heavy concurrency with occasional writes. `RWMutex` + typed map gives zero-allocation reads and type safety.

**Why TTL for config/labels**: These change rarely and unpredictably (user edits settings, new labels emerge). TTL avoids complex invalidation wiring across services. 5 min is conservative — worst case a stale value for 5 min.

**Why event-based for board data**: Board auxiliaries/embeddings only change on upgrade confirm, which is a known code path. Event-based gives instant consistency.

**Location:** New file `semantic_board_cache.go` in the tagging domain. Cache instance owned by `SemanticBoardMatchingService` (injected).

**Invalidation trigger:** `ConfirmSuggestion` calls `cache.InvalidateBoardData()` after DB transaction commits.

### D2: Board Upgrade Dual Mode — Mode Parameter on `suggestUpgrades` API

**Decision:** Add `mode` query parameter to `POST /api/semantic-boards/upgrade/suggest`:
- `mode=discover_new` (default): Current behavior — `collectCandidates` excludes labels already in `board_composition`; LLM prompt omits `merge_into_existing`; filter rejects it.
- `mode=expand_existing`: `collectCandidates` includes labels already assigned to boards (separate query); LLM prompt includes `merge_into_existing` with target board context; filter allows it.

**Why not two separate endpoints:** The pipeline (collect → cluster → prompt → filter) is identical; only three steps differ. A mode flag is cleaner than duplicating the entire pipeline.

**Changes to `collectCandidates`:**
- `mode=discover_new`: Current query (NOT EXISTS in board_composition)
- `mode=expand_existing`: New query — labels that ARE in board_composition, grouped by board, with board context attached to each candidate

**Changes to LLM prompt:**
- `mode=expand_existing`: Add `merge_into_existing` as valid decision; include target board name + existing composition in the cluster context

**Changes to `filterSemanticBoardUpgradeSuggestions`:**
- `mode=discover_new`: Current filter (reject merge_into_existing)
- `mode=expand_existing`: Accept create_new, skip, and merge_into_existing

**Frontend:** Add a mode selector (radio/toggle) before "Generate Suggestions" button. Pass `?mode=discover_new` or `?mode=expand_existing` in the API call.

**DB layer:** `ConfirmSuggestion` already handles `merge_into_existing` — adds auxiliary labels to existing board's `board_composition`. No schema change needed.

### D3: Batch Backfill Concurrency — Worker Pool with Configurable Size

**Decision:** Replace sequential `for range topicTagIDs` loop in `processJob` with a worker pool pattern using `errgroup.Group` with `SetLimit()`.

```go
g, gctx := errgroup.WithContext(ctx)
g.SetLimit(concurrency) // default 4, configurable via ai_settings

for _, id := range topicTagIDs {
    id := id
    g.Go(func() error {
        if _, err := s.matcher.MatchTopicTag(gctx, id); err != nil {
            s.recordFailure(jobID, id, err)
        }
        s.markProcessed(jobID)
        return nil // never cancel group for individual failures
    })
}
_ = g.Wait()
```

**Concurrency config:** New `ai_settings` key `semantic_board_backfill_concurrency` (default 4). Loaded once at job start.

**Progress tracking:** Already exists via `job.Processed` / `job.Failed` / `job.Total` in-memory struct. No WebSocket changes needed — frontend already polls `GET /api/semantic-boards/backfill/:id`.

**Why `errgroup` over manual goroutines:** Built-in concurrency limit, context propagation, clean error handling. Already in stdlib (`golang.org/x/sync`).

**Why not cancel-on-error:** Individual tag match failures should not abort the entire backfill. Failed tags are recorded for retry; remaining tags continue processing.

### D4: `parsePgVector` Deduplication

**Decision:** Board embedding cache stores parsed `[]float64` instead of raw pgvector strings. This eliminates redundant `parsePgVector` calls on every `MatchTopicTag` invocation.

Current flow: `loadBoardEmbeddings` → query DB → `parsePgVector` per row → return map.
Cached flow: First call loads + parses; subsequent calls return cached `map[uint][]float64` directly.

No changes to `parsePgVector` itself — just ensure it's called only on cache miss.

### D5: API Changes Summary

| Endpoint | Change |
|----------|--------|
| `POST /api/semantic-boards/upgrade/suggest` | Add `?mode=discover_new\|expand_existing` query param (default `discover_new`) |
| `POST /api/semantic-boards/upgrade/candidates` | Add `?mode` query param for consistent candidate listing |
| All other endpoints | No change |

No new endpoints. No schema changes.

## Risks & Trade-offs

### Cache Staleness
- **Risk:** Board data cache could go stale if `ConfirmSuggestion` fails after partial commit.
- **Mitigation:** Invalidate after successful transaction commit only. Worst case: stale data until TTL (5 min for config/labels; board data only stale if invalidation callback fails, which would be a bug).

### Memory Pressure
- **Risk:** Board auxiliaries + embeddings cached in-process. With ~200 boards × 2560 dims × 8 bytes = ~4 MB. Negligible.
- **Mitigation:** None needed. Single-user system, memory footprint is trivial.

### Concurrency During Backfill
- **Risk:** 4 concurrent `MatchTopicTag` calls each hit DB for tag-specific data (tag auxiliaries, tag embedding). These are per-tag queries and cannot be cached globally.
- **Mitigation:** Per-tag queries are lightweight (indexed lookups by topic_tag_id). Concurrent load is bounded by worker pool size.

### Expand Mode LLM Hallucination
- **Risk:** LLM might suggest merging unrelated labels into existing boards.
- **Mitigation:** User must confirm each suggestion. Board affinity metadata already helps users assess fit. The LLM prompt should emphasize conservatism — only merge when labels clearly belong.

### Mode Parameter Backward Compatibility
- **Risk:** Existing frontend doesn't pass `mode`. Default is `discover_new` (current behavior). Fully backward compatible.
- **Mitigation:** Default value preserves current behavior.

### `errgroup` Dependency
- **Risk:** `golang.org/x/sync` is already a transitive dependency (Gin uses it). No new dependency.
- **Mitigation:** Verify it's already in `go.sum`.
