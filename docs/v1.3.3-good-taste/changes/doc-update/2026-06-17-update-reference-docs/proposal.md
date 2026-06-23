## Why

`docs/reference/` 下的参考文档存在多处与实际代码不一致的问题，会误导开发者。以代码为权威来源（`backend-go/go.mod`、`backend-go/internal/app/runtime.go`、`backend-go/cmd/`、`backend-go/internal/platform/database/migrator.go`、`backend-go/internal/`）对整个 `docs/reference/` 全量扫描核对，发现 11 类不一致：

1. **Go 版本错误**：`backend.md` 写 Go 1.21、`development.md` 写 Go 1.22+，实际 `go.mod` 是 `go 1.25.0`
2. **`cmd/` 幽灵命令**：`backend.md`、`overview.md`、`development.md`、`开发执行规范.md` 列出 `migrate-db`、`migrate-tags`、`migrate-embedding-queue`、`migrate-digest`、`test-digest`、`test-embedding`，但 `backend-go/cmd/` 实际只有 `server/`
3. **`internal/` 分层过时**：文档提到 `internal/domain/`、`internal/jobs/`、`internal/app/runtimeinfo/`，实际已重构为 `internal/reader/`、`internal/tagmanagement/`、`internal/topicgraph/`、`internal/admin/`、`internal/models/`
4. **调度器列表错误**：`runtime.go` 实际注册 9 个调度器（`auto_refresh`、`preference_update`、`content_completion`、`firecrawl`、`blocked_article_recovery`、`daily_report`、`tag_quality_score`、`log_cleanup`、`aux_label_cleanup`）；但 `api/schedulers.md` 列了 3 个幽灵调度器（`digest`、`narrative_summary`、`tag_hierarchy_cleanup`），`backend.md`/`runtime.md`/`overview.md`/`DATA_LIFECYCLE.md` 混入不存在的 `auto_tag_merge`、`NarrativeSummaryScheduler`
5. **已移除依赖仍列为技术栈**：`overview.md`、`backend.md` 把 `robfig/cron` 列为定时任务技术栈，但 `go.mod` 与源码均无该依赖（实际使用自研 `internal/admin/scheduler` 工厂 + Interval）
6. **不存在的 platform 子包**：`backend.md`、`overview.md`、`development.md` 的 platform 列表包含 `ai/`、`opennotebook/`，实际 `internal/platform/` 只有 `airouter`、`aisettings`、`config`、`database`、`jsonutil`、`logging`、`middleware`、`testutil`、`tracing`、`ws`
7. **runtime.md 描述过时的运行时模式**：通篇描述 `runtimeinfo/schedulers.go` 全局 Interface 共享模式（含 7 个已不存在的变量），还引用 `topicextraction.GetTagQueue()`、`topicanalysis.StartEmbeddingQueueWorker()` 等已删除的旧包函数；实际已是 `SchedulerRegistry` + `registry.Register` 模式，worker 由 `tagging.StartAllWorkers()` 统一启动
8. **tracing.md 路径失效**：自动注入方法表引用 `internal/domain/feeds/service.go`、`internal/domain/contentprocessing/*` 等已删除路径
9. **configuration.md 整段过时**："Topic Analysis 调优"段的 4 个 `TOPIC_ANALYSIS_*` 环境变量、`parseEnvInt`/`parseEnvFloat` 函数、`internal/domain/topicanalysis/ai_analysis.go` 文件在代码中**完全不存在**（grep 零命中）
10. **database 文档引用已删除调度器与旧包**：`DATA_LIFECYCLE.md` 的数据流图引用 `AutoTagMerge` 调度器、`narrative_summary`、`NarrativeSummaryScheduler`；`DATABASE_FIELDS.md` 主表区引用已废弃的 `topic_analysis_jobs`（模型 `topicanalysis.topicAnalysisJobRecord` 不在 migrator 注册表）与 `internal/domain/models/` 旧路径
11. **data-flow.md 引用幽灵调度器**：叙事数据流段以 `NarrativeSummaryScheduler 触发` 开头

## What Changes

- 更新 `architecture/backend.md`：修正 Go 版本、`cmd/` 目录（仅 `server/`）、`internal/` 分层、`platform/` 子包、调度器列表（9 个）、移除 `robfig/cron`、移除 `platform/ai/` 与 `opennotebook/`、移除 `runtimeinfo`、移除 `AutoTagMerge` 调度器描述
- 更新 `architecture/overview.md`：修正目录结构（含 `cmd/`）、移除所有 `internal/domain/*` 引用（含 `narrative`）、调度器数量（9）、移除 `robfig/cron`、移除 `AutoTagMerge` 调度器
- 重写 `architecture/runtime.md`：从 `runtimeinfo` 全局 Interface 模式改为 `SchedulerRegistry` 模式、移除 `topicanalysis`/`topicextraction` 旧函数名、调度器列表对齐 `runtime.go`
- 更新 `architecture/tracing.md`：修正自动注入方法表文件路径为 `internal/reader/service/` 下真实路径
- 更新 `architecture/data-flow.md`：移除 `NarrativeSummaryScheduler` 引用，对齐当前真实触发链路
- 更新 `development.md`：修正 Go 版本、移除"辅助工具命令"表中不存在的 `cmd/` 命令、修正后端目录约定、修正测试示例路径
- 更新 `开发执行规范.md`：修正测试规范引用路径、修正后端分层约定、移除 `cmd/migrate-db` 引用
- 更新 `api/schedulers.md`：删除 3 个幽灵调度器（`digest`、`narrative_summary`、`tag_hierarchy_cleanup`），补齐 4 个缺失调度器（`daily_report`、`log_cleanup`、`aux_label_cleanup`、`blocked_article_recovery`）
- 更新 `configuration.md`：删除整段"Topic Analysis 调优"（环境变量、函数、文件均不存在）
- 更新 `database/DATA_LIFECYCLE.md`：移除 `AutoTagMerge` 调度器、`narrative_summary`、`NarrativeSummaryScheduler` 引用，对齐当前真实调度器与数据流
- 更新 `database/DATABASE_FIELDS.md`：将 `topic_analysis_jobs` 标注为废弃（无 migrator 注册、无 Go 模型），修正 `internal/domain/models/` → `internal/models/`

## Capabilities

### New Capabilities

- `configuration-docs`：`configuration.md` 配置文档与代码环境变量一致
- `database-docs`：`database/` 下文档与代码模型/调度器一致

### Modified Capabilities

- `architecture-docs`：修正 `backend.md`、`overview.md`、`runtime.md`、`tracing.md`、`data-flow.md`
- `development-docs`：修正 `development.md`、`开发执行规范.md`
- `api-docs`：修正 `api/schedulers.md`

## Impact

- **受影响的文件**（11 个）：
  - `docs/reference/architecture/backend.md`
  - `docs/reference/architecture/overview.md`
  - `docs/reference/architecture/runtime.md`
  - `docs/reference/architecture/tracing.md`
  - `docs/reference/architecture/data-flow.md`
  - `docs/reference/development.md`
  - `docs/reference/开发执行规范.md`
  - `docs/reference/api/schedulers.md`
  - `docs/reference/configuration.md`
  - `docs/reference/database/DATA_LIFECYCLE.md`
  - `docs/reference/database/DATABASE_FIELDS.md`

- **已核查无过时引用，本次不改**：`architecture/frontend.md`、`architecture/data-flow.md`（仅 NarrativeSummaryScheduler 一处，已纳入）、`deployment.md`、`testing.md`、`database/ER_DIAGRAM.md`、`database/_index.md`、`api/_conventions.md`、`api/_index.md`、`api/` 其余 14 个端点文档
- **无 API 变更**：本次只更新文档，不涉及代码修改
- **无依赖变更**：不影响任何功能或构建流程
