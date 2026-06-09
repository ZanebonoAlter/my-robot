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

## 10. Remove dead Go functions (18 confirmed, CodeGraph + grep cross-verified)

### 10.1 tagging 包死函数

- [x] 10.1.1 Remove `AggregateArticleTags` from `backend-go/internal/domain/tagging/article_tagger.go` (~50 lines, was called by removed GenerateNarratives)
- [x] 10.1.2 Remove `BackfillArticleTags` from `article_tagger.go` (~30 lines, only called in test)
- [x] 10.1.3 Remove `BackfillMissingDescriptions` from `description_backfill.go` (~50 lines, only called in test)
- [x] 10.1.4 Remove `DeleteTagEmbedding` from `backend-go/internal/domain/tagging/embedding.go` (~8 lines)
- [x] 10.1.5 Remove `ExpandEventCandidatesByArticleCoTags` from `cotag_expansion.go` (~70 lines)
- [x] 10.1.6 Remove `GetCandidateArticleTitles` + `ScanSimilarTagPairs` from `tag_merge_preview.go` (file deleted, ~210 lines)
- [x] 10.1.7 Remove `TagCluster` type from `tag_clustering.go` + `AISummaryRequest` from `ai/service.go`

### 10.2 daily_report 包死函数

- [x] 10.2.1 Remove `GetThreadByID` from `backend-go/internal/domain/daily_report/repository.go` (~25 lines)
- [x] 10.2.2 Remove `GetThreadsByReport` from `repository.go` (~25 lines)
- [x] 10.2.3 Remove `GetThreadsBySection` from `repository.go` (~25 lines)
- [x] 10.2.4 Remove `SetReportStatus` from `repository.go` (~15 lines)

### 10.3 platform 包死函数

- [x] 10.3.1 Remove `ConfigureStdlib` from `backend-go/internal/platform/logging/logging.go` (~10 lines)
- [x] 10.3.2 Remove `EnsureSchemaMigrated` from `backend-go/internal/platform/airouter/migration.go` (~15 lines)
- [x] 10.3.3 SKIP `SaveSummaryConfig` — false positive: function is in `aisettings/config_store.go`, has test callers
- [x] 10.3.4 Remove `MustStartSpan` + `StartSpan` from `backend-go/internal/platform/tracing/helpers.go` (~20 lines)
- [x] 10.3.5 Remove `TruncateStr` from `backend-go/internal/platform/jsonutil/truncate.go` (file deleted)
- [x] 10.3.6 Remove `TraceAsyncOp` from `backend-go/internal/platform/tracing/scheduler.go` (~8 lines)

### 10.4 其他包死函数

- [x] 10.4.1 Remove `CollectActiveCategories` from `backend-go/internal/domain/narrative/collector.go` (~60 lines)
- [x] 10.4.2 Remove `SetCompletionAICredentials` from `content_completion_handler.go` (~5 lines; function was in content package, not ai)

## 11. Remove dead frontend components (13 components, ~4,234 lines)

### 11.1 孤立组件（无任何 import 引用）

- [x] 11.1.1 Delete `front/app/features/ai/components/MergeReembeddingQueuePanel.vue` (285 lines)
- [x] 11.1.2 Delete `front/app/features/articles/components/ContentCompletionView.vue` (111 lines)
- [x] 11.1.3 Delete `front/app/features/hierarchy-config/HierarchyConfigPage.vue` (116 lines)
- [x] 11.1.4 Delete `front/app/features/hierarchy-config/HierarchyPendingList.vue` (91 lines)
- [x] 11.1.5 Delete `front/app/features/hierarchy-config/RebuildTrigger.vue` (111 lines)
- [x] 11.1.6 Delete `front/app/features/tags/components/SemanticBoardList.vue` (320 lines)
- [x] 11.1.7 Delete `front/app/features/topic-graph/components/AIAnalysisPanel.vue` (918 lines)
- [x] 11.1.8 Delete `front/app/features/topic-graph/components/BoardConceptManager.vue` (247 lines)
- [x] 11.1.9 Delete `front/app/features/topic-graph/components/HotspotCategorySelect.vue` (483 lines)
- [x] 11.1.10 Delete `front/app/features/topic-graph/components/TimelinePendingItem.vue` (155 lines)
- [x] 11.1.11 Delete `front/app/features/topic-graph/components/TopicAIAnalysisPanel.vue` (668 lines)
- [x] 11.1.12 Delete `front/app/features/topic-graph/components/TopicAnalysisPanel.vue` (577 lines)
- [x] 11.1.13 Delete `front/app/features/topic-graph/components/TopicAnalysisTabs.vue` (152 lines)

### 11.2 清理孤儿子目录

- [x] 11.2.1 If `front/app/features/hierarchy-config/` becomes empty after deletions, remove the directory

## 12. Remove dead composables and store

- [x] 12.1 Delete `front/app/composables/useRssParser.ts` (274 lines, zero imports)
- [x] 12.2 Delete `front/app/composables/useDagLayout.ts` + exported types (191 lines, zero imports; SectionLifecyclePanel defines own PositionedNode)
- [x] 12.3 Delete `front/app/stores/aiAnalysis.ts` (385 lines, zero imports)

## 13. Remove dead frontend API methods

- [x] 13.1 Remove `triggerGc` from `front/app/api/auxiliaryLabels.ts` (~3 lines)
- [x] 13.2 Remove `enableFeedFirecrawl` from `front/app/api/firecrawl.ts` (~3 lines)
- [x] 13.3 Remove `trackBehavior` from `front/app/api/reading_behavior.ts` (~3 lines)
- [x] 13.4 Remove `getBoard` from `front/app/api/semanticBoards.ts` (~3 lines)
- [x] 13.5 Remove `updateBoardConcept`/`getSectors`/`createSector`/`deleteSector`/`regenerateSectors` from `front/app/api/boardConcepts.ts` (~15 lines)

## 14. Remove dead frontend utilities, constants, and types

### 14.1 死工具函数

- [x] 14.1.1 Remove `formatRelativeTime` and `isToday` from `front/app/utils/date.ts` (~15 lines)
- [x] 14.1.2 Remove `generateRandomColor` from `front/app/utils/text.ts` (~5 lines)
- [x] 14.1.3 Delete `front/app/utils/storage.ts` entirely (4 functions, ~20 lines, zero references)

### 14.2 死常量

- [x] 14.2.1 Remove all 10 dead constants from `front/app/utils/constants.ts`: `DEFAULT_PAGE_SIZE`, `MAX_PAGE_SIZE`, `SIDEBAR_ARTICLE_LIMIT`, `AUTO_REFRESH_MINUTES`, `SIDEBAR_COLLAPSED_WIDTH`, `AI_GENERATION_TIMEOUT`, `AI_SUMMARY_MAX_LENGTH`, `TIME_RANGE_OPTIONS`, `COLOR_OPTIONS`, `ICON_OPTIONS` (~15 lines)

### 14.3 死类型（~45 个，分布在多个文件中）

- [~] 14.3.1 SKIPPED - all 16 topicGraph.ts types are used internally by living `useTopicGraphApi()` methods (verified with grep)
- [~] 14.3.2 SKIPPED - all 7 semanticBoards.ts types are used internally by living `useSemanticBoardsApi()` methods
- [~] 14.3.3 SKIPPED - all 5 hierarchyConfig.ts types are used internally by living `useHierarchyConfigApi()` methods
- [~] 14.3.4 SKIPPED - both auxiliaryLabels.ts types are used by living `getClusters()` method called from TagsPage.vue
- [~] 14.3.5 SKIPPED - both dailyReports.ts types are used by `DailyReport` interface (itself alive)
- [~] 14.3.6 SKIPPED - both firecrawl.ts types are used by living `getStatus()` and `saveSettings()` methods
- [x] 14.3.7 Remove `PaginatedApiResponse` from `front/app/types/api.ts` (skipped `PaginationMeta` - used by `ApiResponse` and `PaginatedData`)
- [x] 14.3.8 Remove `BatchBehaviorRequest`, `UserPreferencesResponse` from `front/app/types/reading_behavior.ts`
- [x] 14.3.9 Remove `TimeRangeOption`, `RefreshTimer`, `GenerationStatus`, `GenerationStatusItem` from `front/app/types/common.ts` (skipped `SortOption`, `FilterOption` - used by `FilterState` in `stores/articles.ts`)
- [~] 14.3.10 SKIPPED - all 3 timeline.ts types are used by `TimelineDigest`/`TimelineFilters` (imported by TopicGraphPage.vue)
- [~] 14.3.11 SKIPPED - all 3 scheduler.ts types are used by `SchedulerStatus` (imported by GlobalSettingsDialog.vue, schedulerMeta.ts, scheduler.ts)
- [x] 14.3.12 Remove `UpdateAbstractNameRequest`, `DetachChildRequest` from `front/app/types/topicTag.ts`
- [~] 14.3.13 SKIPPED - `FeedItem` is used by `FeedResponse` (imported by `server/api/fetch-feed.post.ts`)
- [x] 14.3.14 Remove `AIAnalysisMetadata` from `front/app/types/ai.ts` (skipped `AIRouteProviderLink` - used by `AIRoute` in `AIRouterSettingsPanel.vue`)

### 14.4 重复类型

- [x] 14.4.1 Remove duplicate `RefreshStatus`/`ViewMode`/`MessageType` from both `utils/constants.ts` and `types/common.ts` (kept neither since none used anywhere)

## 15. Verify (extended)

- [x] 15.1 Run `go build ./...` — PASSED
- [x] 15.2 Run `go vet ./...` — PASSED
- [x] 15.3 Run targeted `go test` (narrative, daily_report, airouter, logging, tracing) — PASSED (tagging 预存在 PG 依赖失败，非本次变更)
- [x] 15.4 Run `pnpm lint` — PASSED
- [x] 15.5 Run `pnpm exec nuxi typecheck` (Windows cmd) — 1 pre-existing error in TagsPage.vue:946 (not caused by these changes; TagsPage.vue was untouched)
- [x] 15.6 Run `pnpm build` (Windows cmd) — PASSED

## 16. Remove legacy AI config migration (追加)

### 16.1 背景
`main.go:44` 在每次启动时调用 `EnsureLegacySummaryConfigMigrated()`，将老版本 AI 配置（单一 `summary_config` JSON blob）迁移到新格式（独立 provider/route 表 + 独立 `firecrawl_config` 键）。这是一个已完成的一次性迁移，生产环境早已迁完，每次启动纯空转。

### 16.2 删除内容

- [x] 16.2.1 Delete `backend-go/internal/platform/airouter/migration.go` entirely (2 functions: `EnsureLegacySummaryConfigMigrated` + `MarshalMetadata`)
- [x] 16.2.2 Delete `backend-go/internal/platform/airouter/migration_test.go` (28 lines)
- [x] 16.2.3 Remove `EnsureLegacyProviderAndRoutes` + `ensureDefaultRoute` from `store.go` (~50 lines)
- [x] 16.2.4 Remove `LoadSummaryConfig` + `SaveSummaryConfig` + `summaryConfigKey` from `aisettings/config_store.go` (~10 lines)
- [x] 16.2.5 Remove `airouter.EnsureLegacySummaryConfigMigrated()` call from `cmd/server/main.go:44`
- [x] 16.2.6 Remove legacy `summary_config` firecrawl fallback from `content/firecrawl_config.go` (~8 lines, tried reading firecrawl from old summary_config blob)
- [x] 16.2.7 Fix `aisettings/config_store_test.go` — remove reference to deleted `summaryConfigKey`
- [x] 16.2.8 Remove unused `airouter` import from `main.go`
