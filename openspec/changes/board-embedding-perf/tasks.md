# Tasks: Board Embedding Performance Optimization

## 1. Board Auxiliaries & Embeddings Cache

- [ ] 1.1 Create `backend-go/internal/domain/tagging/semantic_board_cache.go` — define `boardCache` struct with `sync.RWMutex`, fields for board auxiliaries (`[]boardAuxiliaryLabel`), board embeddings (`map[uint][]float64`), and `time.Time` stamps; add `GetBoardAuxiliaries` / `SetBoardAuxiliaries` / `GetBoardEmbeddings` / `SetBoardEmbeddings` / `InvalidateBoardData` methods
- [ ] 1.2 Add `cache *boardCache` field to `SemanticBoardMatchingService` struct in `semantic_board_matching.go:19`; initialize in `NewSemanticBoardMatchingService` (line 23)
- [ ] 1.3 Refactor `loadBoardAuxiliaries` (line 118) — check cache first; on miss, query DB, store result in cache, return; wrap existing DB logic
- [ ] 1.4 Refactor `loadBoardEmbeddings` (line 146) — check cache first; on miss, query DB, call `parsePgVector` per row, store parsed `map[uint][]float64` in cache, return
- [ ] 1.5 In `ConfirmSuggestion` (`semantic_board_upgrade.go:148`), call `cache.InvalidateBoardData()` after successful DB transaction commit

## 2. AI Config Cache

- [ ] 2.1 Add AI config cache entry to `boardCache` — field `config *SemanticBoardMatchConfig` with `configTime time.Time`, TTL constant `configTTL = 5 * time.Minute`; add `GetConfig` / `SetConfig` methods
- [ ] 2.2 Refactor `loadConfig` (`semantic_board_matching.go:427`) — check cache + TTL; on miss or expired, query DB, store result with `time.Now()`, return

## 3. Active Auxiliary Labels Cache

- [ ] 3.1 Add active auxiliary labels cache entry to `boardCache` — field `activeLabels *[]models.SemanticLabel` with `activeLabelsTime time.Time`, TTL `activeLabelsTTL = 5 * time.Minute`; add `GetActiveLabels` / `SetActiveLabels` methods
- [ ] 3.2 Refactor `loadActiveAuxiliaryLabels` (`auxiliary_label_service.go:460`) — accept `*boardCache` or expose as method; check cache + TTL; on miss or expired, query DB, store result, return

## 4. Optimize MatchTopicTag

- [ ] 4.1 In `MatchTopicTag` (`semantic_board_matching.go:73`), replace direct calls to `loadBoardAuxiliaries` / `loadBoardEmbeddings` / `loadConfig` with cache-backed versions from tasks 1–2
- [ ] 4.2 Add metadata pre-filtering before cosine comparison loop — compute candidate board set using ref_count threshold and auxiliary count bounds from config; skip boards with zero auxiliaries or outlier composition size; reduce the O(N×T×B×d) loop to only candidate subset

## 5. Board Upgrade Mode 2 (Expand Existing)

- [ ] 5.1 Add `mode` field to `SemanticBoardUpgradeConfig` struct (string: `"discover_new"` or `"expand_existing"`, default `"discover_new"`)
- [ ] 5.2 Update `collectCandidates` (`semantic_board_upgrade.go:222`) — when `mode == "expand_existing"`, run a separate query that selects labels already in `board_composition`, grouped by board, attaching board name and existing composition to each candidate
- [ ] 5.3 Update LLM prompt construction in `GenerateSuggestions` (line 114) — when `mode == "expand_existing"`, include `merge_into_existing` as a valid decision option and provide target board context (name + current composition) in the cluster description
- [ ] 5.4 Update `filterSemanticBoardUpgradeSuggestions` — accept `mode` parameter; when `mode == "expand_existing"`, allow `merge_into_existing` decisions instead of filtering them out; remove the hard filter at ~line 530
- [ ] 5.5 Update `ConfirmSuggestion` (`semantic_board_upgrade.go:148`) — ensure `merge_into_existing` handling adds auxiliary labels to the target board's `board_composition`; call `cache.InvalidateBoardData()` after commit (already in task 1.5, confirm it fires for both modes)
- [ ] 5.6 Update `suggestUpgrades` handler (`semantic_board_handler.go:1220`) — read `?mode` query param from request, default to `"discover_new"`, pass through to `GenerateSuggestions`

## 6. Frontend Mode Selector

- [ ] 6.1 Add mode state (radio group or toggle: "发现新版块" / "扩充已有版块") to `UpgradeSuggestionPanel.vue` before the "Generate Suggestions" trigger button
- [ ] 6.2 Pass selected mode as `?mode=discover_new|expand_existing` query parameter when calling the suggest API in `front/app/api/semanticBoards.ts`
- [ ] 6.3 Update the API client function signature in `front/app/api/semanticBoards.ts` to accept optional `mode` parameter

## 7. Batch Backfill Concurrency

- [ ] 7.1 Add `semantic_board_backfill_concurrency` config key to `ai_settings` (default 4); load once at job start in `processJob` (`semantic_board_backfill.go:256`)
- [ ] 7.2 Rewrite the sequential `for range topicTagIDs` loop (lines ~186–196 in `semantic_board_backfill.go`) to use `errgroup.Group` with `g.SetLimit(concurrency)`; each iteration calls `MatchTopicTag` in a goroutine; individual failures recorded via `recordFailure` without cancelling the group
- [ ] 7.3 Ensure `golang.org/x/sync` is available (already transitive via Gin); verify in `go.sum`

## 8. Deduplicate parsePgVector

- [ ] 8.1 Verify that `loadBoardEmbeddings` cache (task 1.4) stores parsed `[]float64` directly — not raw pgvector strings — so `parsePgVector` is called only on cache miss; this is covered by task 1.4 but call out explicitly for review
- [ ] 8.2 Audit other callers of `parsePgVector` in `auxiliary_label_service.go:569` and `daily_report/generator.go:510` for redundant parsing opportunities (note-only, no changes unless obvious win)

## 9. Verify

- [ ] 9.1 Run `cd backend-go && go build ./...` — confirm compilation
- [ ] 9.2 Run `cd backend-go && golangci-lint run ./internal/domain/tagging/...` — lint the changed package
- [ ] 9.3 Run `cd backend-go && go test ./internal/domain/tagging/...` — unit tests pass
- [ ] 9.4 Run `cd front && pnpm lint` — frontend lint clean
- [ ] 9.5 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` — typecheck clean
- [ ] 9.6 Manual benchmark: call `MatchTopicTag` before and after cache; verify cache-hit calls skip DB queries and `parsePgVector`
