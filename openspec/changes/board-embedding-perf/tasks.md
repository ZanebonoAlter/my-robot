# Tasks: Board Embedding Performance Optimization

## 1. Board Auxiliaries & Embeddings Cache

- [x] 1.1 Create `backend-go/internal/domain/tagging/semantic_board_cache.go` — define `boardCache` struct with `sync.RWMutex`, fields for board auxiliaries (`[]boardAuxiliaryLabel`), board embeddings (`map[uint][]float64`), and `time.Time` stamps; add `GetBoardAuxiliaries` / `SetBoardAuxiliaries` / `GetBoardEmbeddings` / `SetBoardEmbeddings` / `InvalidateBoardData` methods
- [x] 1.2 Add `cache *boardCache` field to `SemanticBoardMatchingService` struct in `semantic_board_matching.go:19`; initialize in `NewSemanticBoardMatchingService` (line 23)
- [x] 1.3 Refactor `loadBoardAuxiliaries` (line 118) — check cache first; on miss, query DB, store result in cache, return; wrap existing DB logic
- [x] 1.4 Refactor `loadBoardEmbeddings` (line 146) — check cache first; on miss, query DB, call `parsePgVector` per row, store parsed `map[uint][]float64` in cache, return
- [x] 1.5 In `ConfirmSuggestion` (`semantic_board_upgrade.go:148`), call `cache.InvalidateBoardData()` after successful DB transaction commit

## 2. AI Config Cache

- [x] 2.1 Add AI config cache entry to `boardCache` — field `config *SemanticBoardMatchConfig` with `configTime time.Time`, TTL constant `configTTL = 5 * time.Minute`; add `GetConfig` / `SetConfig` methods
- [x] 2.2 Refactor `loadConfig` (`semantic_board_matching.go:427`) — check cache + TTL; on miss or expired, query DB, store result with `time.Now()`, return

## 3. AuxiliaryLabelService Cache (Active Labels + Merge Embeddings)

- [x] 3.1 Define `auxLabelCache` struct in `auxiliary_label_service.go` — `sync.RWMutex`, fields for `activeLabels []models.SemanticLabel` + `activeLabelsAt time.Time` (TTL 5min), and `mergeEmbeddings map[uint][]float64` + `mergeEmbeddingsAt time.Time` (TTL 10min)
- [x] 3.2 Add `cache *auxLabelCache` field to `AuxiliaryLabelService` struct (line ~38); initialize in constructor
- [x] 3.3 Refactor `loadActiveAuxiliaryLabels` (line 460) — check `cache.activeLabels` + TTL; on miss or expired, query DB, store result with `time.Now()`, return
- [x] 3.4 Add `getOrLoadMergeEmbeddings(ctx) (map[uint][]float64, error)` method — double-check locking pattern: RLock check → miss → Lock → re-check → `SELECT id, merge_embedding FROM semantic_labels WHERE label_type='auxiliary' AND status='active' AND merge_embedding IS NOT NULL` → `ParsePgVector` per row → store `map[uint][]float64` in cache → return
- [x] 3.5 Refactor `sqlMergeMatcher` (line 475) — replace DB query with `s.getOrLoadMergeEmbeddings(ctx)` call; iterate cached `map[uint][]float64` for cosine comparison; remove `db *gorm.DB` parameter (now uses `s.db` internally or pass through)
- [x] 3.6 Add invalidation methods: `invalidateOnCreate()` (clear both caches), `invalidateOnDisable()` (clear both), `invalidateOnAliasChange()` (clear activeLabels only, keep mergeEmbeddings)
- [x] 3.7 Wire invalidation: call `invalidateOnCreate()` after L3 create in `ResolveAuxiliaryLabel` (line ~210); call `invalidateOnDisable()` in `DisableAuxiliaryLabel` (line ~170); call `invalidateOnAliasChange()` in `addAlias` (line ~175)

## 4. Optimize MatchTopicTag

- [x] 4.1 In `MatchTopicTag` (`semantic_board_matching.go:73`), replace direct calls to `loadBoardAuxiliaries` / `loadBoardEmbeddings` / `loadConfig` with cache-backed versions from tasks 1–2
- [x] 4.2 Add metadata pre-filtering before cosine comparison loop — compute candidate board set using ref_count threshold and auxiliary count bounds from config; skip boards with zero auxiliaries or outlier composition size; reduce the O(N×T×B×d) loop to only candidate subset

## 5. Board Upgrade Mode 2 (Expand Existing)

- [x] 5.1 Add `mode` field to `SemanticBoardUpgradeConfig` struct (string: `"discover_new"` or `"expand_existing"`, default `"discover_new"`)
- [x] 5.2 Update `collectCandidates` (`semantic_board_upgrade.go:222`) — when `mode == "expand_existing"`, run a separate query that selects labels already in `board_composition`, grouped by board, attaching board name and existing composition to each candidate
- [x] 5.3 Update LLM prompt construction in `GenerateSuggestions` (line 114) — when `mode == "expand_existing"`, include `merge_into_existing` as a valid decision option and provide target board context (name + current composition) in the cluster description
- [x] 5.4 Update `filterSemanticBoardUpgradeSuggestions` — accept `mode` parameter; when `mode == "expand_existing"`, allow `merge_into_existing` decisions instead of filtering them out; remove the hard filter at ~line 530
- [x] 5.5 Update `ConfirmSuggestion` (`semantic_board_upgrade.go:148`) — ensure `merge_into_existing` handling adds auxiliary labels to the target board's `board_composition`; call `cache.InvalidateBoardData()` after commit (already in task 1.5, confirm it fires for both modes)
- [x] 5.6 Update `suggestUpgrades` handler (`semantic_board_handler.go:1220`) — read `?mode` query param from request, default to `"discover_new"`, pass through to `GenerateSuggestions`

## 6. Frontend Mode Selector

- [x] 6.1 Add mode state (radio group or toggle: "发现新版块" / "扩充已有版块") to `UpgradeSuggestionPanel.vue` before the "Generate Suggestions" trigger button
- [x] 6.2 Pass selected mode as `?mode=discover_new|expand_existing` query parameter when calling the suggest API in `front/app/api/semanticBoards.ts`
- [x] 6.3 Update the API client function signature in `front/app/api/semanticBoards.ts` to accept optional `mode` parameter

## 7. Batch Backfill Concurrency

- [x] 7.1 Add `semantic_board_backfill_concurrency` config key to `ai_settings` (default 4); load once at job start in `processJob` (`semantic_board_backfill.go:256`)
- [x] 7.2 Rewrite the sequential `for range topicTagIDs` loop (lines ~186–196 in `semantic_board_backfill.go`) to use `errgroup.Group` with `g.SetLimit(concurrency)`; each iteration calls `MatchTopicTag` in a goroutine; individual failures recorded via `recordFailure` without cancelling the group
- [x] 7.3 Ensure `golang.org/x/sync` is available (already transitive via Gin); verify in `go.sum`

## 8. Deduplicate parsePgVector

- [x] 8.1 Verify that `loadBoardEmbeddings` cache (task 1.4) stores parsed `[]float64` directly — not raw pgvector strings — so `parsePgVector` is called only on cache miss; this is covered by task 1.4 but call out explicitly for review
- [x] 8.2 Verify that `getOrLoadMergeEmbeddings` cache (task 3.4) stores parsed `[]float64` directly — `ParsePgVector` called once on cache miss, subsequent `sqlMergeMatcher` calls use cached vectors
- [x] 8.3 Audit other callers of `parsePgVector` in `auxiliary_label_service.go:569` and `daily_report/generator.go:510` for redundant parsing opportunities (note-only, no changes unless obvious win)

## 9. Verify

- [x] 9.1 Run `cd backend-go && go build ./...` — confirm compilation
- [x] 9.2 Run `cd backend-go && golangci-lint run ./internal/domain/tagging/...` — lint the changed package
- [x] 9.3 Run `cd backend-go && go test ./internal/domain/tagging/...` — unit tests pass
- [x] 9.4 Run `cd front && pnpm lint` — frontend lint clean
- [x] 9.5 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` — typecheck clean
- [x] 9.6 Manual benchmark: call `MatchTopicTag` before and after cache; verify cache-hit calls skip DB queries and `parsePgVector`
- [x] 9.7 Manual benchmark: call `ResolveAuxiliaryLabel` twice; verify second call skips `SELECT id, merge_embedding` DB query (cache hit ~0ms vs first call ~5.2s)
