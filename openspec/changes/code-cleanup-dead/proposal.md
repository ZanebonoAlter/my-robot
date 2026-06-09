## Why

后端和前端经过多轮迭代积累了大量死代码和已废弃但仍在运行的功能，增加了维护负担和新人理解成本。尤其是 `NarrativeSummaryScheduler` 已标记废弃但仍在每天运行、`TopicTag.Kind` 已废弃但仍有 12+ 处在写入、`narrative/service.go` 中约 700 行方法链已无实际调用者、`platform/database/datamigrate/` 整个包零引用、前端 `useNarrativeApi()` 模块和相关组件全部死代码。趁测试规范化和定时任务重构之前先清理死代码，减少后续改动的干扰面。

## What Changes

### 后端

- 删除 `NarrativeSummaryScheduler`（`jobs/narrative_summary.go`，390 行）及其在 `runtime.go`、`handler.go`、`runtimeinfo/schedulers.go` 中的注册
- 删除 `narrative/service.go` 中零调用者的废弃方法：`GenerateAndSaveGlobal`、`GenerateAndSaveForAllCategories`
- 删除 `narrative/watched_narrative.go` 中零调用者的废弃函数：`GenerateWatchedTagNarratives`、`generateSingleWatchedNarrative`（~200 行）
- 删除 `narrative/board_narrative_generator.go` 中零调用者的废弃函数：`SaveNarrativesForBoard`（~60 行）、`LoadBoardEventTags`（~40 行）
- 删除 `narrative/board_collector.go` 整个文件（97 行，2 个函数零调用者）
- 删除 `narrative/collector.go` 中零调用者的函数：`CollectCategoryNarrativeSummaries`（~30 行）
- 删除 `narrative/generator.go` 中零调用者的函数：`GenerateNarratives`（~50 行）
- 删除 `database/db.go` 中零调用者的废弃函数：`Migrate()`、`EnsureTables()`
- 删除 `database/migrator.go` 中零调用者的废弃函数：`autoMigrateModels()`
- 删除 `platform/database/datamigrate/` 整个包（~350 行，8 个函数零导入）
- 删除 `jobs/cleanup_budget.go` 整个文件 + 测试（~150 行，`CleanupBudget` 类型零生产调用者）
- 删除上述废弃功能对应的无效测试（`narrative/service_test.go` 中约 13 个测试用例）
- 替换 `airouter/embedding.go` 中手写的 Newton's method `sqrt` 为 `math.Sqrt`
- 统一 CST timezone 定义：将 6+ 处重复的 `time.FixedZone("CST", 8*3600)` 收敛到 `domain/models/utils.go`
- 移除 `TopicTag.Kind` 的所有写入点

### 前端

- 删除 `front/app/api/topicGraph.ts` 中 `useNarrativeApi()` 整个函数及其 8 个 API 方法（~70 行）
- 删除 `front/app/api/topicGraph.ts` 中 7 个仅被死 API/composable 使用的类型定义（~70 行）
- 删除 `front/app/features/topic-graph/components/NarrativeDetailCard.vue` 整个文件（~210 行，零导入）
- 删除 `front/app/features/tags/components/BoardNarrativeTimeline.vue` 整个文件（~120 行，零导入）
- 删除 `front/app/api/semanticBoards.ts` 中 `getBoardNarratives`（~3 行）、`triggerNarrativeGeneration`（~3 行）方法
- 删除 `front/app/api/semanticBoards.ts` 中 `BoardNarrative`、`BoardNarrativeTag` 等仅被死组件使用的类型

### 后续扫描新增（CodeGraph + grep 交叉验证）

#### 后端死函数（18个确认，非 Gin handler 误报）
- **tagging 包**: `AggregateArticleTags`、`BackfillArticleTags`、`BackfillMissingDescriptions`、`DeleteTagEmbedding`、`ExpandEventCandidatesByArticleCoTags`、`GetCandidateArticleTitles`、`ScanSimilarTagPairs`
- **daily_report 包**: `GetThreadByID`、`GetThreadsByReport`、`GetThreadsBySection`、`SetReportStatus`
- **platform 包**: `ConfigureStdlib`(logging)、`EnsureSchemaMigrated`(airouter)、`SaveSummaryConfig`(airouter)、`MustStartSpan`+`StartSpan`(tracing)、`TruncateStr`(jsonutil)、`TraceAsyncOp`(tracing)
- **narrative 包**: `CollectActiveCategories`
- **content 包**: `SetCompletionAICredentials`
- **死类型**: `AISummaryRequest` (ai/service.go)、`TagCluster` (tag_clustering.go)

#### 前端死组件（13个，~4,234行）
- `MergeReembeddingQueuePanel` (285行)、`ContentCompletionView` (111行)
- `HierarchyConfigPage` + `HierarchyPendingList` + `RebuildTrigger` (完整未引用的 feature 模块，318行)
- `SemanticBoardList` (320行)、`AIAnalysisPanel` (918行)
- `BoardConceptManager` (247行)、`HotspotCategorySelect` (483行)
- `TimelinePendingItem` (155行)、`TopicAIAnalysisPanel` (668行)
- `TopicAnalysisPanel` (577行)、`TopicAnalysisTabs` (152行)

#### 前端 Composables/Store（3个，~660行）
- `useRssParser` (274行)、`useDagLayout` (191行)、`useAIAnalysisStore` (385行)

#### 前端死 API 方法（9个）
- `triggerGc` (auxiliaryLabels)、`enableFeedFirecrawl` (firecrawl)、`trackBehavior` (reading_behavior)
- `getBoard` (semanticBoards)、`updateBoardConcept`/`getSectors`/`createSector`/`deleteSector`/`regenerateSectors` (boardConcepts)

#### 前端死工具/常量/类型（~60+ 项）
- 工具函数: `formatRelativeTime`、`isToday`、`generateRandomColor`、整个 `storage.ts` 模块(4函数)
- 常量: `DEFAULT_PAGE_SIZE`、`MAX_PAGE_SIZE`、`SIDEBAR_ARTICLE_LIMIT`、`AUTO_REFRESH_MINUTES` 等 10 个
- 死类型: ~45 个分布在 14 个 API/类型文件中
- 重复定义: `RefreshStatus`、`ViewMode`、`MessageType` 在 `constants.ts` 和 `common.ts` 各定义一份均未被引用

## Capabilities

### New Capabilities

（无新能力引入）

### Modified Capabilities

- `narrative-board-generation`: 移除已废弃的全局和分类级别叙事生成方法，删除 `board_collector.go`、`LoadBoardEventTags`、`CollectCategoryNarrativeSummaries`、`GenerateNarratives` 等零调用者函数
- `tagging-domain`: 移除 `TopicTag.Kind` 的所有写入点，统一使用 `Category`；移除 `Kind` 相关的 DTO 字段和 JSON 序列化
- `tag-embedding-management`: 将手写 `sqrt` 替换为 `math.Sqrt`，优化余弦相似度计算性能
- `frontend-narrative-cleanup`: 移除前端零调用者的叙事 API 模块、类型定义和死组件
- `platform-dead-code`: 删除 `datamigrate/` 整个未使用包、`CleanupBudget` 类型、`autoMigrateModels` 废弃函数

## Impact

- `backend-go/internal/jobs/narrative_summary.go`：整个文件删除
- `backend-go/internal/jobs/cleanup_budget.go`：整个文件删除 + 对应测试
- `backend-go/internal/domain/narrative/`：删除 ~700 行废弃方法 + ~200 行 watched_narrative 废弃函数 + board_collector.go(97 行) + collector.go 死函数(~30 行) + generator.go 死函数(~50 行) + board_narrative_generator.go 死函数(~100 行)
- `backend-go/internal/domain/narrative/service_test.go`：删除 ~13 个无效测试
- `backend-go/internal/platform/database/datamigrate/`：整个目录删除（~350 行）
- `backend-go/internal/platform/database/db.go`：删除 `Migrate()`、`EnsureTables()`
- `backend-go/internal/platform/database/migrator.go`：删除 `autoMigrateModels()`
- `backend-go/internal/domain/models/topic_graph.go`：`Kind` 字段标记不可写入
- `backend-go/internal/domain/models/utils.go`：统一 CST timezone
- `backend-go/internal/app/runtime.go`：移除 NarrativeSummaryScheduler 启动
- `backend-go/internal/app/runtimeinfo/schedulers.go`：移除对应 interface{} 变量
- `backend-go/internal/jobs/handler.go`：移除 `narrative_summary` 描述符
- `backend-go/internal/platform/airouter/embedding.go`：替换 sqrt 实现
- 多处使用 CST timezone 的文件需更新 import
- `front/app/api/topicGraph.ts`：删除 `useNarrativeApi()` + 7 个死类型（~140 行）
- `front/app/features/topic-graph/components/NarrativeDetailCard.vue`：整个文件删除（~210 行）
- `front/app/features/tags/components/BoardNarrativeTimeline.vue`：整个文件删除（~120 行）
- `front/app/api/semanticBoards.ts`：删除 2 个死 API 方法 + 关联类型（~20 行）
- 新增后端 ~18 个死函数删除: `article_tagger.go`、`tag_clustering.go`、`tagging/embedding.go`、`daily_report/repository.go`、`logging/logging.go`、`airouter/migration.go`、`tracing/helpers.go`、`tracing/scheduler.go`、`jsonutil/truncate.go`、`ai/service.go`、`narrative/collector.go`、`content/content_completion_service.go`
- 新增前端 13 个死组件 + 3 个 composable/store + 9 个死 API 方法 + ~60 项死类型/工具/常量的删除
- 不影响数据库 schema
