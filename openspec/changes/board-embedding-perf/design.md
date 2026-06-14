# Design: Board Embedding Performance Optimization

## Context

Two independent performance bottlenecks exist in the tag processing pipeline:

### Bottleneck A: MatchTopicTag (版块匹配)

Every event tag that gets embedded automatically triggers `MatchTopicTag`, which:

1. **Loads all board auxiliaries** from `board_composition` + `semantic_labels` (full table scan)
2. **Loads all board embeddings** and calls `parsePgVector` on each (expensive string→float64 conversion)
3. **Loads AI config** (13 rows from `ai_settings`) via `loadConfig` on every call
4. **Loads active auxiliary labels** via `loadActiveAuxiliaryLabels` (another full table scan)
5. Performs O(N×T×B×d) brute-force cosine comparison entirely in Go memory

With ~200+ boards, ~3000+ auxiliary labels, and 2560-dim embeddings, each `MatchTopicTag` call incurs 4 DB queries and ~10ms of vector parsing overhead. During backfill, this is called hundreds of times sequentially.

### Bottleneck B: sqlMergeMatcher (辅助标签 L2 匹配) — 最严重

`ResolveAuxiliaryLabel` 的 L2 阶段调用 `sqlMergeMatcher`，每次从 DB 加载全部活跃辅助标签的 `merge_embedding` 向量：

```sql
SELECT id, merge_embedding FROM semantic_labels WHERE id IN (...10823 IDs...)
-- 5243ms, rows=10823, ~216 MB 传输
```

调用链：`tagArticle → AttachAuxiliaryLabels → per label → ResolveAuxiliaryLabel → sqlMergeMatcher`。一篇文章 2~5 个 tag × 1~3 个 label = 4~15 次调用 × 5.2s = **20~78 秒仅此一个查询**。

Additionally, board upgrade only supports discovering new boards. The `collectCandidates` query hard-filters out labels already in `board_composition`, and `filterSemanticBoardUpgradeSuggestions` rejects `merge_into_existing` decisions from LLM. Users who want to expand existing boards have no automated path.

## Goals

1. Eliminate redundant DB loads in `MatchTopicTag` via in-process caching
2. **Eliminate redundant `sqlMergeMatcher` DB loads** via merge embedding cache (~216 MB → ~0ms on hit)
3. Add "expand existing board" mode to board upgrade
4. Parallelize backfill processing with configurable concurrency
5. Reduce `parsePgVector` redundant calls

## Non-Goals

- Rebuilding HNSW index (2560-dim exceeds pgvector 2000 limit; not feasible)
- Replacing Go-side cosine with SQL-side matching (already benchmarked: SQL-side equally slow for high-dim)
- Introducing Redis or external cache (overkill for single-user system)
- Changing the matching algorithm itself (rules, scoring)
- Auto-triggering board upgrade (remains manual)

## Decisions

### D1: Cache Architecture — `sync.RWMutex` + map with timestamp invalidation

**Decision:** Use a simple Go struct with `sync.RWMutex`, `map`, and `time.Time` per cache entry. No external dependencies.

**Six cache entries across two services:**

*BoardMatchCache* (owned by `SemanticBoardMatchingService`):

| Cache | Key Type | Value | Invalidation |
|-------|----------|-------|-------------|
| Board auxiliaries | `"all"` | `[]boardAuxiliaryLabel` | On board upgrade confirm / board composition change |
| Board embeddings | `"all"` | `map[uint][]float64` | On board upgrade confirm / board composition change |
| AI config | `"config"` | `SemanticBoardMatchConfig` | TTL 5 min |

*AuxLabelCache* (owned by `AuxiliaryLabelService`, see D6):

| Cache | Key Type | Value | Invalidation |
|-------|----------|-------|-------------|
| Active auxiliary labels | `"all"` | `[]models.SemanticLabel` | TTL 5 min + event-based (create/disable) |
| Merge embeddings | `"all"` | `map[uint][]float64` | TTL 10 min + event-based (create/disable) |

**Why not `sync.Map`**: We need read-heavy concurrency with occasional writes. `RWMutex` + typed map gives zero-allocation reads and type safety.

**Why TTL for config/labels**: These change rarely and unpredictably (user edits settings, new labels emerge). TTL avoids complex invalidation wiring across services. 5 min is conservative — worst case a stale value for 5 min.

**Why event-based for board data**: Board auxiliaries/embeddings only change on upgrade confirm, which is a known code path. Event-based gives instant consistency.

**Why event-based for board data + merge embeddings**: Board auxiliaries/embeddings only change on upgrade confirm. Merge embeddings only change on label create/disable. Both are known code paths — event-based gives instant consistency.

**Locations:**
- BoardMatchCache: New file `semantic_board_cache.go` in the tagging domain. Cache instance owned by `SemanticBoardMatchingService`.
- AuxLabelCache: Inline in `auxiliary_label_service.go`. Cache instance owned by `AuxiliaryLabelService`.

**Invalidation triggers:**
- BoardMatchCache: `ConfirmSuggestion` calls `cache.InvalidateBoardData()` after DB transaction commits.
- AuxLabelCache: `ResolveAuxiliaryLabel` (L3 create) / `DisableAuxiliaryLabel` / `addAlias` call respective invalidation methods.

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

Same principle applies to merge embedding cache (D6) — `ParsePgVector` called once on cache miss, cached result reused.

### D6: Merge Embedding Cache — `AuxiliaryLabelService` 内部缓存

**Decision:** 在 `AuxiliaryLabelService` 中新增 `auxLabelCache` 结构，同时缓存 `activeLabels`（Task 3）和 `mergeEmbeddings`。两个缓存共享同一把 `sync.RWMutex`，在标签新增/禁用时统一失效。

```go
type auxLabelCache struct {
    mu sync.RWMutex

    // 标签元数据缓存 (Task 3 复用)
    activeLabels     []models.SemanticLabel
    activeLabelsAt   time.Time
    activeLabelsTTL  = 5 * time.Minute

    // merge embedding 向量缓存 (新增)
    mergeEmbeddings     map[uint][]float64  // labelID → parsed vector
    mergeEmbeddingsAt   time.Time
    mergeEmbeddingsTTL  = 10 * time.Minute
}
```

**Why in `AuxiliaryLabelService` not `boardCache`:** 职责内聚 — `sqlMergeMatcher` 和 `loadActiveAuxiliaryLabels` 都在 `AuxiliaryLabelService` 中，缓存应由同一 service 管理。避免跨服务依赖。

**Why TTL 10 min (比 activeLabels 长):** merge embedding 向量变化频率更低（仅新增/禁用标签时变化），更长的 TTL 减少 216 MB 传输的频率。

**Why 200 MB 常驻内存可接受:** 单用户系统，内存充裕。200 MB 换取 5.2s → ~0ms 的提速，ROI 极高。

**Invalidation matrix:**

| Event | activeLabels | mergeEmbeddings | 原因 |
|-------|-------------|-----------------|------|
| L3 新增标签 | 失效 | 失效 | 新 ID + 新 vector |
| DisableAuxiliaryLabel | 失效 | 失效 | ID 集合变化 |
| addAlias | 失效 | **保留** | vector 不变 |
| TTL 过期 | 重新加载 | 重新加载 | 兜底 |

**Cache loading flow:**

```
getOrLoadMergeEmbeddings(ctx)
  ├─ cache hit (TTL 内): return cached map[uint][]float64  ~0ms
  └─ cache miss:
       ├─ SELECT id, merge_embedding FROM semantic_labels
       │   WHERE label_type='auxiliary' AND status='active'
       │   AND merge_embedding IS NOT NULL
       ├─ ParsePgVector × N
       ├─ store map[uint][]float64 in cache
       └─ return  ~5.2s (首次)
```

**Simplified `sqlMergeMatcher`:**

```go
func (s *AuxiliaryLabelService) sqlMergeMatcher(ctx context.Context, labels []models.SemanticLabel, _ string, mergeVector []float64) (*models.SemanticLabel, error) {
    embeddings, err := s.getOrLoadMergeEmbeddings(ctx)
    if err != nil {
        return nil, err
    }

    var best *models.SemanticLabel
    for _, label := range labels {
        existingVec, ok := embeddings[label.ID]
        if !ok {
            continue
        }
        sim, err := airouter.CosineSimilarity(mergeVector, existingVec)
        if err != nil || sim < auxiliaryLabelMergeThreshold {
            continue
        }
        if best == nil || label.RefCount > best.RefCount || (label.RefCount == best.RefCount && label.ID < best.ID) {
            best = &label
        }
    }
    return best, nil
}
```

No DB query on cache hit. Cosine similarity computed entirely in memory against cached vectors.

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
- **Additional:** Merge embedding cache adds ~200 MB (10K labels × 2560 dims × 8 bytes).
- **Mitigation:** Single-user system with ample memory. 200 MB is acceptable for 5.2s → ~0ms improvement. If memory becomes a concern, could add LRU eviction or reduce TTL.

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
