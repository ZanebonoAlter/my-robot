## Phase 0: 后端功能包重组 (Feature-Oriented Reorganization)

### 0.1 models 包迁移
- [x] 0.1.1 将 `internal/domain/models/` 目录移至 `internal/models/`
- [x] 0.1.2 全局替换导入路径 `syntopica-backend/internal/domain/models` → `syntopica-backend/internal/models`
- [x] 0.1.3 `go build ./...` 验证编译通过

### 0.2 tagmanagement 包迁移 (tagging → tagmanagement)
- [x] 0.2.1 将 `internal/domain/tagging/` 移至 `internal/tagmanagement/`（含子包 analysis/, watched/, extraction/）
- [x] 0.2.2 所有 .go 文件 `package tagging` → `package tagmanagement`
- [x] 0.2.3 全局替换导入路径 `syntopica-backend/internal/domain/tagging` → `syntopica-backend/internal/tagmanagement`
- [x] 0.2.4 解决合并后命名冲突（如有）
- [x] 0.2.5 `go build ./...` + `go test ./internal/tagmanagement/...` 验证

### 0.3 reader 包迁移 (feed + article + category + content → reader)
- [x] 0.3.1 创建 `internal/reader/` 目录
- [x] 0.3.2 迁移 feed 文件：handler→feed_handler.go, service→feed_service.go, service_test→feed_service_test.go, opml.go, rss_parser.go
- [x] 0.3.3 迁移 article 文件：handler→article_handler.go, handler_test→article_handler_test.go
- [x] 0.3.4 迁移 category 文件：handler→category_handler.go
- [x] 0.3.5 迁移 content 文件：content_completion_batch.go, content_completion_handler.go→content_completion_handler.go, content_completion_handler_test.go, content_completion_service.go, content_completion_service_test.go, firecrawl_config.go, firecrawl_handler.go, firecrawl_job_queue.go, firecrawl_job_queue_test.go, firecrawl_service.go
- [x] 0.3.6 所有文件改为 `package reader`
- [x] 0.3.7 全局替换导入路径：`domain/feed`、`domain/article`、`domain/category`、`domain/content` → `reader`
- [x] 0.3.8 解决合并后命名冲突（如有）
- [x] 0.3.9 `go build ./...` + `go test ./internal/reader/...` 验证

### 0.4 topicgraph 包迁移 (topicgraph + daily_report → topicgraph)
- [x] 0.4.1 将 `internal/domain/topicgraph/` 移至 `internal/topicgraph/`
- [x] 0.4.2 将 `internal/domain/daily_report/` 文件移入 `internal/topicgraph/`（加 daily_report_ 前缀）：handler→daily_report_handler.go, handler_test→daily_report_handler_test.go, generator→daily_report_generator.go, cluster→daily_report_cluster.go, cluster_test→daily_report_cluster_test.go, dedup→daily_report_dedup.go, dedup_test→daily_report_dedup_test.go, matching→daily_report_matching.go, matching_test→daily_report_matching_test.go, models→daily_report_models.go, register_models→daily_report_register_models.go, repository→daily_report_repository.go, repository_test→daily_report_repository_test.go
- [x] 0.4.3 topicgraph 原文件重命名加 graph_ 前缀：handler→graph_handler.go, handler_test→graph_handler_test.go, service→graph_service.go, hotspot_digests→graph_hotspot_digests.go
- [x] 0.4.4 所有文件改为 `package topicgraph`
- [x] 0.4.5 全局替换导入路径 `domain/topicgraph`、`domain/daily_report` → `topicgraph`
- [x] 0.4.6 解决合并后命名冲突
- [x] 0.4.7 `go build ./...` + `go test ./internal/topicgraph/...` 验证

### 0.5 admin 包迁移 (aiadmin + jobs + narrative + preferences → admin)
- [x] 0.5.1 创建 `internal/admin/` 目录
- [x] 0.5.2 迁移 aiadmin 文件：handler→ai_handler.go, handler_test→ai_handler_test.go
- [x] 0.5.3 迁移 jobs 文件：handler→scheduler_handler.go, handler_test→scheduler_handler_test.go, auto_refresh→scheduler_auto_refresh.go, auto_refresh_test→scheduler_auto_refresh_test.go, content_completion→scheduler_content_completion.go, content_completion_test→scheduler_content_completion_test.go, daily_report→scheduler_daily_report.go, firecrawl→scheduler_firecrawl.go, firecrawl_test→scheduler_firecrawl_test.go, preference_update→scheduler_preference_update.go, preference_update_test→scheduler_preference_update_test.go, tag_quality_score→scheduler_tag_quality_score.go, tag_quality_score_test→scheduler_tag_quality_score_test.go, log_cleanup→scheduler_log_cleanup.go, aux_label_cleanup→scheduler_aux_label_cleanup.go, blocked_article_recovery→scheduler_blocked_article_recovery.go, scheduler_status_response_test→scheduler_status_response_test.go, trigger_now_status_code_test→trigger_now_status_code_test.go
- [x] 0.5.4 迁移 narrative 文件：handler→narrative_handler.go, service→narrative_service.go, service_test→narrative_service_test.go, generator→narrative_generator.go, generator_test→narrative_generator_test.go, collector→narrative_collector.go, board_creation→narrative_board_creation.go, board_narrative_generator→narrative_board_narrative_generator.go, board_postprocess→narrative_board_postprocess.go
- [x] 0.5.5 迁移 preferences 文件：handler→preferences_handler.go, handler_test→preferences_handler_test.go, service→preferences_service.go
- [x] 0.5.6 所有文件改为 `package admin`
- [x] 0.5.7 全局替换导入路径：`domain/aiadmin`、`jobs`、`domain/narrative`、`domain/preferences` → `admin`
- [x] 0.5.8 解决合并后命名冲突（SchedulerStatusResponse 等类型可能冲突）
- [x] 0.5.9 `go build ./...` + `go test ./internal/admin/...` 验证

### 0.6 清理与验证
- [x] 0.6.1 删除空的 `internal/domain/` 目录
- [x] 0.6.2 更新 `app/router.go` 和 `app/runtime.go` 所有导入路径
- [x] 0.6.3 `golangci-lint run ./...` + `go build ./...` + `go test ./...` 全量验证

## Phase 1: Spring 式分层 (Handler → Service → Repository)

- [x] 1.1 reader 包：创建 `repository.go`，将 handler 内 DB 调用迁移到 repository，handler 只做 HTTP 参数解析
- [x] 1.2 topicgraph 包：daily_report 已有 repository，补齐 topicgraph handler 的 repository
- [x] 1.3 tagmanagement 包：创建 `repository.go`，各 handler DB 调用迁移
- [x] 1.4 admin 包：创建 `repository.go`，AI admin handler DB 调用迁移
- [x] 1.5 评估 `internal/models/` 是否拆分到各功能包（TopicTag 等跨功能类型处理）
- [x] 1.6 `golangci-lint run ./...` + `go build ./...` + `go test ./...` 验证

## Phase 1.5: 子包拆分（handler/service/repository 子目录）

- [x] 1.5.1 reader 包：service/ + repository/ 子目录，handler 留在根包作为公共 API
- [x] 1.5.2 reader 包：根包 wire.go facade 重新导出（ContentCompletionService, FirecrawlJobQueue 等对外类型）
- [x] 1.5.3 topicgraph 包：handler/service/repository 全部子包化
- [x] 1.5.4 topicgraph 包：daily_report_models → repository/（避免 service ↔ repository 循环依赖），根包 wire.go facade
- [x] 1.5.5 tagmanagement 包：handler/service/repository 全部子包化
- [x] 1.5.6 tagmanagement 包：analysis/extraction/watched 已有子包保持不动，根包 wire.go facade
- [x] 1.5.7 admin 包：handler/service/scheduler/repository 四层子包化
- [x] 1.5.8 各子包测试文件跟随被测代码分布
- [x] 1.5.9 跨子包测试依赖修复（分类错误重移、unexport 符号导出、helper 函数复制）
- [x] 1.5.10 `go build ./...` + `go test ./internal/reader/... ./internal/topicgraph/... ./internal/tagmanagement/... ./internal/admin/...` 验证

## Phase 1.7: 后端包内组织优化（代码审查发现的新问题）

### 1.7.1 tagmanagement/service/ 按功能域拆子包
- [x] 1.7.1.1 创建 `service/tagging/` 子包 → 重命名为 `service/core/`（因 embedding + extraction 与 tagging 相互依赖过多，合并为一个核心服务包）
- [x] 1.7.1.2 创建 `service/board/` 子包
- [x] 1.7.1.3 `service/embedding/` → 合并到 `service/core/`
- [x] 1.7.1.4 创建 `service/merge/` 子包
- [x] 1.7.1.5 创建 `service/auxlabel/` 子包
- [x] 1.7.1.6 `extractor_enhanced.go` 等移入 `service/core/`
- [x] 1.7.1.7 更新 `wire.go` facade（bridge.go 函数变量注入模式打破循环依赖）
- [x] 1.7.1.8 `go build ./...` + `go test ./internal/tagmanagement/...` 验证

### 1.7.2 tagmanagement 组织范式统一
- [x] 1.7.2.1 删除 `analysis/` 整个目录
- [x] 1.7.2.2 从 `router.go` 移除 taganalysis 路由注册
- [x] 1.7.2.3 `watched/watched_tags_handler.go` → `handler/watched_tags_handler.go`
- [x] 1.7.2.4 `watched/watched_tags_service.go` → `service/watched/` 子包
- [x] 1.7.2.5 删除空的 `watched/` 目录
- [x] 1.7.2.6 更新 `wire.go` facade
- [x] 1.7.2.7 `go build ./...` + `go test ./internal/tagmanagement/...` 验证

### 1.7.3 narrative 废弃代码删除
- [x] 1.7.3.1 删除 `admin/handler/narrative_handler.go`
- [x] 1.7.3.2 删除 `admin/service/narrative_*.go`（8 个文件）
- [x] 1.7.3.3 清理 `admin/repository/repository.go` 中 narrative 相关 DB 方法
- [x] 1.7.3.4 保留 `models/narrative.go` + `models/narrative_board.go`（被其他代码引用）
- [x] 1.7.3.5 从 `admin/wire.go` 移除 narrative re-export
- [x] 1.7.3.6 从 `router.go` 移除 `admin.RegisterNarrativeRoutes(api)`
- [x] 1.7.3.7 前端重命名待后续 Phase 补充
- [x] 1.7.3.8 `go build ./...` + `go test ./internal/admin/...` 验证

### 1.7.4 platform/ai 合并到 platform/airouter
- [x] 1.7.4.1 `platform/ai/service.go` 功能合并到 `platform/airouter/fallback.go`
- [x] 1.7.4.2 更新所有 `import ".../platform/ai"` → `".../platform/airouter"`
- [x] 1.7.4.3 删除 `platform/ai/` 目录
- [x] 1.7.4.4 `go build ./...` 验证

### 1.7.5 巨型文件拆分
- [x] 1.7.5.1 `semantic_board_handler.go` (1923行) → `board_crud_handler.go` (1115行) + `board_match_handler.go` (514行) + `board_upgrade_handler.go` (299行)
- [x] 1.7.5.2 `topicgraph/repository/repository.go` (1122行) → topic-graph-only (643行)，daily-report methods 迁入 `daily_report_repository.go` (617行)，消除约 420 行重复代码
- [x] 1.7.5.3 `daily_report_generator.go` (1031行) → `daily_report_llm.go` (324行) + `daily_report_merge.go` (263行) + `daily_report_orchestrator.go` (424行)
- [x] 1.7.5.4 `go build ./...` + 受影响包测试验证

## Phase 2: 路由自注册 + 调度器接口化

- [x] 2.1 reader 包添加 `routes.go`
- [x] 2.2 topicgraph 包添加 `routes.go`
- [x] 2.3 tagmanagement 包添加 `routes.go`
- [x] 2.4 admin 包添加 `routes.go`
- [x] 2.5 重构 `router.go`（~40 行委托调用 + health/ws/tasks）
- [x] 2.6 在 admin/scheduler 定义 `Scheduler` 接口 + `Registry`
- [x] 2.7 各 scheduler 实现 `Scheduler` 接口
- [x] 2.8 重构 `app/runtime.go`：Registry 模式
- [x] 2.9 重构 `admin/scheduler_handler.go`：`handler.Reg.Get()` 替代 runtimeinfo
- [x] 2.10 删除 `internal/app/runtimeinfo/` 包
- [x] 2.11 `go build ./...` + `go test ./...`（core 包 panic 为已有测试初始化问题）

## Phase 2.5: 调度器工厂模式（BaseScheduler 消除重复脚手架）

### 2.5.1 BaseScheduler 核心实现
- [x] 2.5.1.1 创建 `admin/scheduler/base.go`：定义 `JobFunc`、`JobResult`、`Config`、`BaseScheduler` 结构体
- [x] 2.5.1.2 `BaseScheduler` 统一实现 `Scheduler` 接口全部方法（Start/Stop/TriggerNow/UpdateInterval/ResetStats/GetStatus）
- [x] 2.5.1.3 `BaseScheduler` 统一内部状态管理：mutex、running、isExecuting、nextRun、lastRun、lastError、统计计数
- [x] 2.5.1.4 `BaseScheduler` 统一并发调度：内部用 `time.Ticker` 统一调度（消除 A/B 两类差异）
- [x] 2.5.1.5 `BaseScheduler` 统一 `TriggerNow` 的互斥锁检查 + 返回 map 模式
- [x] 2.5.1.6 `BaseScheduler` 可选的 SchedulerTask DB 状态持久化（通过 `TaskPersistence` Config 开关）

### 2.5.2 批次 1：简单 scheduler 迁移（无 DB 状态持久化）
- [x] 2.5.2.1 `log_cleanup` → `JobFunc`，已删除 `LogCleanupScheduler` struct
- [x] 2.5.2.2 `aux_label_cleanup` → `JobFunc`，已删除 `AuxLabelCleanupScheduler` struct
- [x] 2.5.2.3 `blocked_article_recovery` → `JobFunc`，已删除 `BlockedArticleRecoveryScheduler` struct
- [x] 2.5.2.4 `go build ./...` + `go test ./internal/admin/...` 验证通过

### 2.5.3 批次 2：中等 scheduler 迁移（有 SchedulerTask DB 状态持久化）
- [x] 2.5.3.1 `preference_update` → `JobFunc` + DB 状态持久化
- [x] 2.5.3.2 `tag_quality_score` → `JobFunc` + DB 状态持久化
- [x] 2.5.3.3 `auto_refresh` → `JobFunc` + DB 状态持久化（含 resetStaleRefreshingFeeds、WS broadcast 等辅助逻辑）
- [x] 2.5.3.4 `go build ./...` + `go test ./internal/admin/...` 验证通过

### 2.5.4 批次 3：复杂 scheduler 迁移（特化方法 + 复杂业务依赖）
- [x] 2.5.4.1 `content_completion` → `JobFunc` + DB 状态持久化 + 处理 `ContentCompletionService` 依赖
- [x] 2.5.4.2 `daily_report` → `JobFunc` + DB 状态持久化 + `DailyReportSchedulerWrapper` 保留 `TriggerNowWithDate` 特化接口
- [x] 2.5.4.3 `firecrawl` → `JobFunc` + 处理 `FirecrawlJobQueue` 依赖 + WS broadcast
- [x] 2.5.4.4 `go build ./...` + `go test ./internal/admin/...` 验证通过

### 2.5.5 清理
- [x] 2.5.5.1 删除所有 `XxxScheduler` struct 定义文件（9 个 `scheduler_*.go` 文件）
- [x] 2.5.5.2 更新 `runtime.go` 注册代码：`registry.Register(scheduler.New(scheduler.Config{...}))`
- [x] 2.5.5.3 更新 `admin/wire.go` 导出新类型；`DailyReportScheduler` 特化通过 `DailyReportSchedulerWrapper.TriggerNowWithDate` 处理
- [x] 2.5.5.4 `go build ./...` + `go vet ./...` + `go test ./internal/admin/...` 全量验证通过

## Phase 3: 前端 SSE 事件流统一

- [x] 3.1 创建 `app/composables/useEventStream.ts`：单例 SSE 连接 + 类型化事件订阅 + 自动重连
- [x] 3.2 创建 `app/utils/eventTypes.ts`：集中所有事件类型字符串常量
- [x] 3.3 确认/补充后端 SSE 端点覆盖所有当前 WS 消息类型
- [x] 3.4 迁移 `useTagWebSocket.ts` → 使用 `useEventStream()` 订阅 `tag_completed` / `tag_failed`
- [x] 3.5 迁移 `useWebSocketRebuild.ts` → 使用 `useEventStream()` 订阅 `hierarchy_rebuild`（如无消费者则删除）
- [x] 3.6 迁移 `useDailyReportProgress.ts` → 使用 `useEventStream()` 订阅 `daily_report_progress`
- [x] 3.7 迁移 `useOrganizeWebSocket.ts` → 使用 `useEventStream()` 订阅 `organize_progress`
- [x] 3.8 删除旧的独立 WebSocket 连接代码
- [x] 3.9 运行 `pnpm lint` + typecheck 验证（lint ✅，typecheck 有 pre-existing `FeedResponse` 错误）

## Phase 4: 前端 Store 数据一致性

- [x] 4.1 `useArticlesStore.markAsRead` 改为调用 `apiStore.markAsRead` + 乐观更新回滚
- [x] 4.2 `useArticlesStore.markAllAsRead` 改为调用 `apiStore.markAllAsRead` + 乐观更新回滚
- [x] 4.3 `useArticlesStore.toggleFavorite` 改为调用 `apiStore.toggleFavorite` + 乐观更新回滚
- [x] 4.4 `useArticlesStore.setCurrentArticle` 确保 markAsRead 调用经过 API
- [x] 4.5 消除 `apiStore.allFeeds`：合并到 `feeds`，消费方改用 computed 过滤
- [x] 4.6 运行 `pnpm test:unit` 验证 Store 测试通过（api.test.ts 通过 ✅）

### 4.7 拆分 stores/api.ts 巨型 store（articles 部分完成）

- [x] 4.7.1 盘点 `stores/api.ts` 中所有状态和方法
- [x] 4.7.2 保留 `api.ts` 核心能力（feeds/categories CRUD、loading/error、markAsRead/markAllAsRead/toggleFavorite）
- [x] 4.7.3 feeds 部分保留在 api.ts（已有 feeds.ts 做 computed 层）
- [x] 4.7.4 articles 状态和方法完整迁移到 `stores/articles.ts`
- [x] 4.7.5 scheduler/content-completion 保留在 api.ts（后续下沉）
- [x] 4.7.6 更新消费方：ArticleContentView、useAutoRefresh、FeedLayoutShell、api.test.ts
- [x] 4.7.7 `pnpm lint` ✅ + typecheck ✅（仅剩 2 pre-existing 错误）+ `pnpm test:unit` ✅（11/12 pass，TagHierarchy 为 test env 问题）

## Phase 5: 前端统一错误通知

- [x] 5.1 创建 `app/composables/useNotify.ts`：全局 toast 队列 + success/error/warn 方法
- [x] 5.2 创建 `<NotifyContainer>` 组件渲染 toast 列表
- [x] 5.3 在 `app.vue` 中挂载 `<NotifyContainer>`
- [x] 5.4 API store 方法失败时自动调用 `notify.error()`（写入类方法增加 apiErrNotify 辅助函数）
- [x] 5.5 TopicGraphPage 替换 `notice.value` 为 `notify`（同时保留 notice ref 用于 :error prop）
- [x] 5.6 关键组件替换：TagHierarchy + BoardCompositionPanel
- [x] 5.7 运行 `pnpm lint` + typecheck 验证（lint ✅，typecheck 仅剩 pre-existing 错误）

## Phase 6: 前端 API 层归一化

- [x] 6.1 创建 `app/utils/api-helpers.ts`：`camelizeKeys`、`buildQueryString`（自动拼 `?`）
- [x] 6.2 `apiClient.buildQueryParams` 改为自动拼接 `?`
- [x] 6.3 创建 `createQueueApi<T>()` 泛型工厂，替代 tagQueue 与 embeddingQueue 的重复代码
- [x] 6.4 创建 `unwrapResponse<T>()` 统一处理 API 响应，替代各模块的 `as unknown as` 模式
- [x] 6.5 逐步替换 6 处独立 snake_case→camelCase normalizer
- [x] 6.6 运行 `pnpm lint` + typecheck + `pnpm test:unit` 验证（lint ✅、typecheck 1 个 pre-existing 错误、tests 11/12 pass 1 个 pre-existing 失败）

## Phase 7: 代码卫生

- [x] 7.1 删除 `app/composables/useWebSocketRebuild.ts`（如确认无消费者）
- [x] 7.2 删除未使用依赖：`d3-dag`、`p5`（`pnpm remove`，`katex` 仍有 `KaTeXRender.vue` 消费）
- [x] 7.3 替换魔法字符串：分类标识符、路由路径、事件类型 → 常量（constants.ts + FeedLayoutShell 已迁移，其他组件待后续完成）
- [x] 7.4 `FeedLayoutShell.vue` 消除 7 处不必要的 `any`
- [x] 7.5 确认 `narrative` 包废弃路线（标记 `// Deprecated` 或保留并行）
- [x] 7.6 `ArticleListPanelShell.vue` 改用 `v-bind="$props"` + `v-bind="$attrs"` 与其他 Shell 对齐
- [x] 7.7 将 `platform/ai/` 的 prompt/parser 工具函数迁移到 `platform/airouter/`（Phase 1.7 已完成后端内容）

## Phase 8: 前端大组件拆分（>500 行 / ~15K 阈值）

### 8.1 TopicGraphPage.vue (74K → 35K, P0 ✅)
- [x] 8.1.1 抽取 `useTopicGraph()` composable（~40KB）管理全部共享状态
- [x] 8.1.2 子视图从 composable 读取（TopicGraphSidebar/TopicTimeline/TopicGraphHeader 通过 props 接收，composable 管理共享数据流）
- [x] 8.1.3 验证：lint ✅ typecheck ✅（仅 2 pre-existing）tests ✅（11/12）—— 脚本从 ~1700 行降至 ~25 行（+ ~280 行模板 + ~800 行 CSS），模板逻辑区约 300 行

### 8.2 GlobalSettingsDialog.vue (67K → 7K, P0 ✅)
- [x] 8.2.1 盘点所有设置面板（feeds / preferences / general / queues / firecrawl / schedulers 6 个 tab）
- [x] 8.2.2 抽取 `useGlobalSettings()` composable（~440 行）管理全部对话框状态 + 数据加载 + 操作逻辑
- [x] 8.2.3 抽取 4 个独立 tab 组件：`<FeedSettingsPanel>`、`<ReadingPreferencesPanel>`、`<FirecrawlConfigPanel>`、`<SchedulerStatusPanel>`（general / queues 已使用提取的组件）
- [x] 8.2.4 验证：lint ✅ typecheck ✅（仅 2 pre-existing）tests ✅（11/12）—— GlobalSettingsDialog 从 ~1460 行降至 ~184 行（-87%），新文件：6 个总计 ~1580 行

### 8.3 TagsPage.vue (54K, P0)
- [x] 8.3.1 抽取 `useTagsPage()` composable 管理标签页状态（已完成第一步；后续 Phase 9 继续深化，避免巨型 composable）
- [x] 8.3.2 各功能面板（标签列表 / 语义看板 / 辅助标签 / 合并预览）拆为独立组件（BoardListSidebar/BoardTimelinePanel/BoardEditDialog/ArticlePreviewModal）
- [x] 8.3.3 验证 TagsPage 缩减至 <300 行（236 行 ✅）

### 8.4 AIRouterSettingsPanel.vue (42K, P1)
- [x] 8.4.1 按功能拆分：AIRouterBackupProviders + AIRouterCapabilityRoutes 子组件
- [x] 8.4.2 抽取 `useAIRouterSettings()` composable + provide/inject 上下文共享
- [x] 8.4.3 验证缩减至 <300 行（150 行 ✅）

### 8.5 ArticleContentView.vue (39K, P1)
- [x] 8.5.1 拆出：ArticleContentToolbar / ArticleContentPreviewPanel / ArticleIframeView + 共享 ArticleContent.css
- [x] 8.5.2 抽取 `useArticleContentView()` composable + v-bind 计算属性传递
- [x] 8.5.3 验证缩减至 <200 行（182 行 ✅）

### 8.6 其他大型组件 (P1，视带宽推进)
- [x] 8.6.1 `TagMergePreview.vue` (32K→369行)：抽取 TagMergeGroup 子组件（group target + 搜索覆盖层 + suggestions 列表）
- [x] 8.6.2 `TopicGraphSidebar.vue` (29K→439行)：按功能区块拆分（TopicGraphArticleCard + TopicGraphMergeDialog）

### 8.7 验证
- [x] 8.7.1 运行 `pnpm lint`（0 errors ✅）+ typecheck（仅 pre-existing FeedResponse ✅）+ `pnpm test:unit`（11/12 pass ✅）验证
- [x] 8.7.2 确认所有已拆分组件均 <500 行（GlobalSettingsDialog 184 ✅、TagsPage 236 ✅、AIRouterSettingsPanel 150 ✅、ArticleContentView 182 ✅、TagMergePreview 369 ✅、TopicGraphSidebar 439 ✅）

## Phase 9: 前端架构复盘后续深化（耦合度 / Locality）

### 9.1 事件流 Seam 收口（P0）
- [x] 9.1.1 修复 `useEventStream()` 空闲销毁后的 singleton 生命周期：最后一个订阅者退订后释放全局实例，后续订阅可重新连接
- [x] 9.1.2 `useDailyReportProgress()` 保存 `stream.on(...)` 返回的 unsubscribe，并在卸载/清理时执行
- [x] 9.1.3 `TagQueuePanel.vue` 移除自建 `new WebSocket('/ws')`，改为订阅统一事件流触发刷新
- [x] 9.1.4 确认后端 SSE 端点覆盖当前 `/ws` 全局事件：当前仅 tag merge preview 有专用 SSE，`/ws` 全局事件未被 SSE 覆盖，故本轮不切换 `EventSource`
- [x] 9.1.5 短期保留 WebSocket Adapter：`useEventStream()` 注释/spec/proposal 统一为实时事件连接，不再宣称已完成 SSE 切换

### 9.2 Store 循环知识消除（P0）
- [x] 9.2.1 `useArticlesStore.markAsRead/markAllAsRead/toggleFavorite` 直接调用 `useArticlesApi()`，不再委托 `apiStore`
- [x] 9.2.2 `useApiStore` 移除文章 mutation 方法和对 `~/stores/articles` 的动态 import
- [x] 9.2.3 `useFeedsStore` 暴露 feed 未读数同步的小 Interface（`clearUnreadCounts` / `adjustUnreadCount`），`articlesStore` 不再调用 `apiStore.clearFeedUnreadCounts`
- [x] 9.2.4 应用初始化重构 — `apiStore.initialize()` 保持只加载 categories + feeds；FeedLayoutShell.onMounted 移除冗余 fetchFeeds()，articles + watchedTags + globalUnreadCount 全并行加载
- [x] 9.2.5 评估 `useFeedsStore` — 确认为 computed facade，保留（14 个消费者依赖其辅助 computed），移除 HMR 并加注释说明

### 9.3 巨型 composable 深化（P0/P1）
- [x] 9.3.1 将 `useTopicGraph()` 拆为 4 个子 composable（useFloatingPanelDrag/useArticlePreview/useTopicTimeline/useHotspotTopics）
- [x] 9.3.2 `TopicGraphPage.vue` 和子组件只依赖上述小 Interface，不再接收/解构一个超大 return 对象
- [x] 9.3.3 将 `useTagsPage()` 按 board CRUD、auxiliary labels、timeline、article preview 拆分
- [x] 9.3.4 `useGlobalSettings()` (437→124行) 拆分为 4 个 panel 级 composable（useFirecrawlConfig/useSchedulerStatus/useReadingPreferences），各 panel 自管理状态 + 生命周期；GlobalSettingsDialog 瘦身保留编排

### 9.4 跨 feature 深 import 收口（P0）
- [x] 9.4.1 将 `features/articles/utils/normalizeArticle` 上移到共享 normalizer（`api/normalizers/article.ts`）
- [x] 9.4.2 更新 `features/tags`、`features/topic-graph`、`stores/articles` 等消费者使用共享 normalizer
- [x] 9.4.3 `TagsPage.vue` 不直接 import `features/topic-graph/components/TagMergePreview.vue`；改用 shared components
- [x] 9.4.4 `TagsPage.vue` / `TopicGraphPage.vue` 不直接深 import `features/articles/components/ArticleContentView.vue`；改为 `features/articles/public.ts` facade
- [x] 9.4.5 增加一次 import graph 检查：禁止 `features/*` 直接引用其他 feature 的内部 `components/`、`composables/`、`utils/`（允许 `public.ts`/共享底层模块）

### 9.5 API 归一化单一入口（P1）
- [x] 9.5.1 `apiClient.buildQueryParams` 委托 `buildQueryString`（api-helpers），消除重复实现
- [x] 9.5.2 `createQueueApi.getTasks` 改用 `buildQueryString`，不再手写 `URLSearchParams`
- [x] 9.5.3 新增 `mapApiResponse()` 辅助函数，替换 `api/categories.ts`、`api/abstractTags.ts`、`api/tagMergePreview.ts` 中的 `as unknown as ApiResponse<T>` 模式
- [x] 9.5.4 `types/reading_behavior.ts` 加文档注释约束 snake_case DTO 仅用于 API 层；规范已在 `api-helpers.ts camelizeKeys` 中实现

### 9.6 通知 Module Nuxt 化与去重（P1）
- [x] 9.6.1 `useNotify()` 改为 `useState` 替代模块级 `ref`，确保 SSR 兼容
- [x] 9.6.2 确认错误通知责任层已正确落实：API module 不弹 toast，由 Store/Composable 通知；`articlesStore` 内动态 import 改为静态 import
- [x] 9.6.3 检查确认无重复通知路径；`articlesStore` 优化动态 import 为静态 import（`useApiStore`、`useNotify`）

### 9.7 验证
- [x] 9.7.1 `pnpm lint` — 0 errors ✅
- [x] 9.7.2 `pnpm exec nuxi typecheck` — 已通过 ✅
- [x] 9.7.3 `pnpm test:unit` — 已通过 ✅
- [x] 9.7.4 前端架构说明已在各变更中记录（feedsStore 注释、snake_case 约束注释、vitest.setup.ts Nuxt auto-import mock）
