# Tasks: code-cleanup-dead

## 1. Remove NarrativeSummaryScheduler

- [x] 1.1 Delete file `backend-go/internal/jobs/narrative_summary.go` entirely (390 lines)
- [x] 1.2 Remove `NarrativeSummaryScheduler` startup/registration from `backend-go/internal/app/runtime.go`
- [x] 1.3 Remove `narrativeSummaryDescriptor` entry from `schedulerDescriptors()` in `backend-go/internal/jobs/handler.go`
- [x] 1.4 Remove `NarrativeSummarySchedulerInterface` variable from `backend-go/internal/app/runtimeinfo/schedulers.go`
- [x] 1.5 Grep frontend (`front/`) for any `narrative_summary` or `NarrativeSummary` references and remove if found

## 2. Remove dead narrative methods and database functions

- [x] 2.1 Remove `GenerateAndSaveGlobal` method from `backend-go/internal/domain/narrative/service.go`
- [x] 2.2 Remove `GenerateAndSaveForAllCategories` method from `backend-go/internal/domain/narrative/service.go`
- [x] 2.3 Delete `backend-go/internal/domain/narrative/watched_narrative.go` entirely (contains `GenerateWatchedTagNarratives`, `generateSingleWatchedNarrative`, `WatchedTagNarrativeOutput` — all dead)
- [x] 2.4 Remove `SaveNarrativesForBoard` function from `backend-go/internal/domain/narrative/board_narrative_generator.go` (~60 lines)
- [x] 2.5 Remove `LoadBoardEventTags` function from `backend-go/internal/domain/narrative/board_narrative_generator.go` (~40 lines)
- [x] 2.6 Delete `backend-go/internal/domain/narrative/board_collector.go` entirely (97 lines — `CollectPreviousDayBoards` and `CollectPreviousBoardNarratives` both zero callers)
- [x] 2.7 Remove `CollectCategoryNarrativeSummaries` function from `backend-go/internal/domain/narrative/collector.go` (~30 lines)
- [x] 2.8 Remove `GenerateNarratives` function from `backend-go/internal/domain/narrative/generator.go` (~50 lines); check if `NarrativeOutput` type can also be removed
- [x] 2.9 Remove `Migrate()` function from `backend-go/internal/platform/database/db.go`
- [x] 2.10 Remove `EnsureTables()` function from `backend-go/internal/platform/database/db.go`
- [x] 2.11 Remove `autoMigrateModels()` function from `backend-go/internal/platform/database/migrator.go` (deprecated wrapper, zero callers)

## 3. Remove entirely dead packages and types

- [x] 3.1 Delete entire directory `backend-go/internal/platform/database/datamigrate/` (~350 lines, 8 functions, zero imports)
- [x] 3.2 Delete file `backend-go/internal/jobs/cleanup_budget.go` (5 methods, zero production callers)
- [x] 3.3 Delete file `backend-go/internal/jobs/cleanup_budget_test.go` (tests for deleted type)

## 4. Remove deprecated tests

- [x] 4.1 Remove ~13 deprecated test cases from `backend-go/internal/domain/narrative/service_test.go` that test `GenerateAndSaveGlobal`, `GenerateAndSaveForAllCategories`, `GenerateWatchedTagNarratives`, and related deleted methods
- [x] 4.2 Check if `backend-go/internal/domain/narrative/service_test.go` becomes empty after removals and delete it if so
- [x] 4.3 Check if any other test files under `backend-go/internal/domain/narrative/` become empty and delete them if so

## 5. Unify CST timezone

- [x] 5.1 Verify `shanghaiTZ` in `backend-go/internal/domain/models/utils.go` is the canonical definition; rename/export as `CST` if needed for clarity
- [x] 5.2 Replace duplicate CST timezone definitions in `backend-go/internal/platform/database/db.go` with import from `models/utils.go`
- [x] 5.3 Replace duplicate CST timezone definitions in `backend-go/internal/domain/tagging/services.go` with import from `models/utils.go`
- [x] 5.4 Replace inline CST timezone in `backend-go/internal/domain/content/content_completion_service.go` (~line 459)
- [x] 5.5 Replace inline CST timezone in `backend-go/internal/domain/feed/handler.go` (~line 584)
- [x] 5.6 Replace inline CST timezone in `backend-go/internal/domain/feed/rss_parser.go` (~line 156)
- [x] 5.7 Replace inline CST timezone in `backend-go/internal/domain/feed/service.go` (~lines 52, 101, 148)
- [x] 5.8 Grep all Go files for `time.FixedZone("CST"`, `8*3600`, and `28800` to find remaining inline CST definitions; replace each with import from `models/utils.go`
- [x] 5.9 Verify no remaining duplicate CST timezone definitions: `grep -r "FixedZone" backend-go/`

## 6. Replace hand-rolled sqrt

- [x] 6.1 Replace Newton's method `sqrt` function (lines 43-57) in `backend-go/internal/platform/airouter/embedding.go` with `math.Sqrt`; remove the local `sqrt` function
- [x] 6.2 Add `"math"` to imports in `backend-go/internal/platform/airouter/embedding.go` if not already present

## 7. Remove TopicTag.Kind writes

- [x] 7.1 Find and remove all `.Kind =` assignments in `backend-go/internal/domain/topicgraph/service.go` (12+ places)
- [x] 7.2 Find and remove all `.Kind =` assignments in `backend-go/internal/domain/tagging/tagger.go`
- [x] 7.3 Find and remove `NormalizeTopicKind()` function if it exists (likely in `backend-go/internal/domain/tagging/helpers.go`)
- [x] 7.4 Add or update deprecation comment on `Kind` field in `TopicTag` struct (`backend-go/internal/domain/models/topic_graph.go`) noting it is no longer written; `.Category` is the authoritative field

## 8. Clean up frontend dead code

- [x] 8.1 Delete file `front/app/features/topic-graph/components/NarrativeDetailCard.vue` entirely (~210 lines, zero imports)
- [x] 8.2 Delete file `front/app/features/tags/components/BoardNarrativeTimeline.vue` entirely (~120 lines, zero imports)
- [x] 8.3 Remove `useNarrativeApi()` function and all 8 API methods from `front/app/api/topicGraph.ts` (~70 lines)
- [x] 8.4 Remove dead types from `front/app/api/topicGraph.ts`: `NarrativeItem`, `NarrativeTimelineDay`, `NarrativeScopesResponse`, `BoardNarrativeItem`, `BoardItem` (topicGraph.ts `BoardItem`), `TagBrief` (topicGraph.ts), `BoardTimelineDay` (~70 lines)
- [x] 8.5 Remove `getBoardNarratives` method from `front/app/api/semanticBoards.ts`
- [x] 8.6 Remove `triggerNarrativeGeneration` method from `front/app/api/semanticBoards.ts`
- [x] 8.7 Remove `BoardNarrative` and `BoardNarrativeTag` types from `front/app/api/semanticBoards.ts` if only used by deleted methods
- [x] 8.8 Remove unused `TopicKind` import from `front/app/features/topic-graph/pages/TopicGraphPage.vue` (line 12)

## 9. Verify

- [x] 9.1 Run `cd backend-go && go build ./...` — must compile with zero errors
- [x] 9.2 Run `cd backend-go && go vet ./...` — must pass with zero warnings
- [x] 9.3 Run `cd backend-go && golangci-lint run ./...` — must pass
- [x] 9.4 Run targeted `go test` for modified packages (`narrative`, `topicgraph`, `tagging`, `airouter`, `database`, `jobs`) — all must pass
- [x] 9.5 Run `cd front && pnpm lint` — must pass
- [x] 9.6 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` — must pass
- [x] 9.7 Run `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` — must pass
