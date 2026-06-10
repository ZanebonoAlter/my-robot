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
- [ ] 1.7.1.1 创建 `service/tagging/` 子包，迁移：article_tagger.go, tagger.go, tag_queue.go, workers.go, helpers.go, services.go, types.go, tag_cache.go, config_service.go 及对应测试
- [ ] 1.7.1.2 创建 `service/board/` 子包，迁移：semantic_board_matching.go, semantic_board_upgrade.go, semantic_board_backfill.go, tag_clustering.go 及对应测试
- [ ] 1.7.1.3 创建 `service/embedding/` 子包，迁移：embedding.go, embedding_queue.go, merge_reembedding_queue.go, description_backfill.go, person_metadata_backfill.go, cotag_expansion.go 及对应测试
- [ ] 1.7.1.4 创建 `service/merge/` 子包，迁移：tag_merge_suggest.go, hard_merge.go 及对应测试
- [ ] 1.7.1.5 创建 `service/auxlabel/` 子包，迁移：auxiliary_label_service.go 及对应测试
- [ ] 1.7.1.6 将 service/ 下 extractor_enhanced.go, extractor_heuristic.go 及测试移入已有的 `extraction/` 一级子包
- [ ] 1.7.1.7 更新 `wire.go` facade 的 import 路径
- [ ] 1.7.1.8 `go build ./...` + `go test ./internal/tagmanagement/...` 验证

### 1.7.2 tagmanagement 组织范式统一
- [ ] 1.7.2.1 删除 `analysis/` 整个目录（已确认为废弃代码：后端有路由但前端零调用）
- [ ] 1.7.2.2 从 `router.go` 移除 `taganalysis` import 和路由注册
- [ ] 1.7.2.3 `watched/watched_tags_handler.go` → `handler/watched_tags_handler.go`
- [ ] 1.7.2.4 `watched/watched_tags_service.go` → `service/watched/` 子包（或合并到已有子包）
- [ ] 1.7.2.5 删除空的 `watched/` 目录
- [ ] 1.7.2.6 更新 `wire.go` facade
- [ ] 1.7.2.7 `go build ./...` + `go test ./internal/tagmanagement/...` 验证

### 1.7.3 narrative 废弃代码删除
- [ ] 1.7.3.1 删除 `admin/handler/narrative_handler.go`（12 个 endpoint，前端零调用）
- [ ] 1.7.3.2 删除 `admin/service/narrative_*.go`（narrative_service.go, narrative_collector.go, narrative_generator.go, narrative_board_creation.go, narrative_board_narrative_generator.go, narrative_board_postprocess.go 及测试）
- [ ] 1.7.3.3 清理 `admin/repository/repository.go` 中 narrative 相关 DB 方法
- [ ] 1.7.3.4 检查 `models/narrative.go` + `models/narrative_board.go` 是否还被其他代码引用，无则删除
- [ ] 1.7.3.5 从 `admin/wire.go` 移除 `RegisterNarrativeRoutes` re-export
- [ ] 1.7.3.6 从 `router.go` 移除 `admin.RegisterNarrativeRoutes(api)` 调用
- [ ] 1.7.3.7 前端 `NarrativeGenerateDialog.vue` → `DailyReportGenerateDialog.vue`，更新 TagsPage.vue 中的 import
- [ ] 1.7.3.8 `go build ./...` + `go test ./internal/admin/...` 验证

### 1.7.4 platform/ai 合并到 platform/airouter
- [ ] 1.7.4.1 将 `platform/ai/service.go` 功能合并到 `platform/airouter/fallback.go`（或适合的文件）
- [ ] 1.7.4.2 更新所有 `import "syntopica-backend/internal/platform/ai"` → `import "syntopica-backend/internal/platform/airouter"`
- [ ] 1.7.4.3 删除 `platform/ai/` 目录
- [ ] 1.7.4.4 `go build ./...` 验证

### 1.7.5 巨型文件拆分
- [ ] 1.7.5.1 `semantic_board_handler.go` (1923行) → 按操作拆为 `board_crud_handler.go` + `board_match_handler.go` + `board_upgrade_handler.go`
- [ ] 1.7.5.2 `topicgraph/repository/repository.go` (1114行) → 确认职责，按实体拆分
- [ ] 1.7.5.3 `daily_report_generator.go` (1031行) → 按阶段拆分（初始化/收集/聚类/生成/后处理）
- [ ] 1.7.5.4 `go build ./...` + 受影响包测试验证

## Phase 2: 路由自注册 + 调度器接口化

- [ ] 2.1 reader 包添加 `routes.go`：`RegisterRoutes(rg)` 注册 feeds/articles/categories/content-completion/firecrawl/opml 路由
- [ ] 2.2 topicgraph 包添加 `routes.go`：`RegisterRoutes(rg)` 注册 topic-graph/daily-reports 路由
- [ ] 2.3 tagmanagement 包添加 `routes.go`：`RegisterRoutes(rg)` 注册 tag-queue/embedding/semantic-boards/watched-tags 路由
- [ ] 2.4 admin 包添加 `routes.go`：`RegisterRoutes(rg)` 注册 ai/schedulers/reading-behavior/user-preferences/narrative 路由
- [ ] 2.5 重构 `router.go`：`SetupRoutes()` 变为 4 个 `RegisterRoutes` 委托调用 + health/ws/tasks/status 路由
- [ ] 2.6 在 admin 包定义 `Scheduler` 接口 + `Registry` 结构体
- [ ] 2.7 各 scheduler 显式实现 `Scheduler` 接口
- [ ] 2.8 改造 `app/runtime.go`：创建 Registry，各 scheduler 注册，`StartAll`/`StopAll`
- [ ] 2.9 改造 `admin/scheduler_handler.go`：通过 Registry 获取 scheduler
- [ ] 2.10 删除 `internal/app/runtimeinfo/` 包
- [ ] 2.11 `golangci-lint run ./...` + `go build ./...` + `go test ./...` 验证

## Phase 3: 前端 SSE 事件流统一

- [ ] 3.1 创建 `app/composables/useEventStream.ts`：单例 SSE 连接 + 类型化事件订阅 + 自动重连
- [ ] 3.2 创建 `app/utils/eventTypes.ts`：集中所有事件类型字符串常量
- [ ] 3.3 确认/补充后端 SSE 端点覆盖所有当前 WS 消息类型
- [ ] 3.4 迁移 `useTagWebSocket.ts` → 使用 `useEventStream()` 订阅 `tag_completed` / `tag_failed`
- [ ] 3.5 迁移 `useWebSocketRebuild.ts` → 使用 `useEventStream()` 订阅 `hierarchy_rebuild`（如无消费者则删除）
- [ ] 3.6 迁移 `useDailyReportProgress.ts` → 使用 `useEventStream()` 订阅 `daily_report_progress`
- [ ] 3.7 迁移 `useOrganizeWebSocket.ts` → 使用 `useEventStream()` 订阅 `organize_progress`
- [ ] 3.8 删除旧的独立 WebSocket 连接代码
- [ ] 3.9 运行 `pnpm lint` + typecheck 验证

## Phase 4: 前端 Store 数据一致性

- [ ] 4.1 `useArticlesStore.markAsRead` 改为调用 `apiStore.markAsRead` + 乐观更新回滚
- [ ] 4.2 `useArticlesStore.markAllAsRead` 改为调用 `apiStore.markAllAsRead` + 乐观更新回滚
- [ ] 4.3 `useArticlesStore.toggleFavorite` 改为调用 `apiStore.toggleFavorite` + 乐观更新回滚
- [ ] 4.4 `useArticlesStore.setCurrentArticle` 确保 markAsRead 调用经过 API
- [ ] 4.5 消除 `apiStore.allFeeds`：合并到 `feeds`，消费方改用 computed 过滤
- [ ] 4.6 运行 `pnpm test:unit` 验证 Store 测试通过

## Phase 5: 前端统一错误通知

- [ ] 5.1 创建 `app/composables/useNotify.ts`：全局 toast 队列 + success/error/warn 方法
- [ ] 5.2 创建 `<NotifyContainer>` 组件渲染 toast 列表
- [ ] 5.3 在 `app.vue` 中挂载 `<NotifyContainer>`
- [ ] 5.4 API store 方法失败时自动调用 `notify.error()`
- [ ] 5.5 TopicGraphPage 替换 `notice.value` 为 `notify.error/warn`
- [ ] 5.6 其他组件逐步替换独立的 `error.value` → `notify.error()`
- [ ] 5.7 运行 `pnpm lint` + typecheck 验证

## Phase 6: 前端 API 层归一化

- [ ] 6.1 创建 `app/utils/api-helpers.ts`：`camelizeKeys`、`buildQueryString`（自动拼 `?`）
- [ ] 6.2 `apiClient.buildQueryParams` 改为自动拼接 `?`
- [ ] 6.3 创建 `createQueueApi<T>()` 泛型工厂，替代 tagQueue 与 embeddingQueue 的重复代码
- [ ] 6.4 创建 `unwrapResponse<T>()` 统一处理 API 响应，替代各模块的 `as unknown as` 模式
- [ ] 6.5 逐步替换 6 处独立 snake_case→camelCase normalizer
- [ ] 6.6 运行 `pnpm lint` + typecheck + `pnpm test:unit` 验证

## Phase 7: 代码卫生

- [ ] 7.1 删除 `app/composables/useWebSocketRebuild.ts`（如确认无消费者）
- [ ] 7.2 删除未使用依赖：`d3-dag`、`p5`、`katex`（`pnpm remove`）
- [ ] 7.3 替换魔法字符串：分类标识符、路由路径、事件类型 → 常量
- [ ] 7.4 `FeedLayoutShell.vue` 消除 7 处不必要的 `any`
- [ ] 7.5 确认 `narrative` 包废弃路线（标记 `// Deprecated` 或保留并行）
- [ ] 7.6 `ArticleListPanelShell.vue` 改用 `v-bind="$attrs"` 与其他 Shell 对齐
- [ ] 7.7 将 `platform/ai/` 的 prompt/parser 工具函数迁移到 `platform/airouter/`

## Phase 8: 前端大组件拆分

- [ ] 8.1 `TopicGraphPage.vue` 抽取 `useTopicGraph()` composable 管理共享状态
- [ ] 8.2 子视图从 composable 读取而非 props drilling
- [ ] 8.3 验证 TopicGraphPage 从 2286 行缩减至 <300 行
- [ ] 8.4 运行 `pnpm lint` + typecheck + `pnpm test:unit` 验证
