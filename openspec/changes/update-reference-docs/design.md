## Context

Syntopica 项目经历了多次架构重构，后端目录结构从 `internal/domain/*` 演变为按业务域划分的 `internal/reader/`、`internal/tagmanagement/`、`internal/topicgraph/`、`internal/admin/`。调度器系统从分散的 `internal/jobs/` + `runtimeinfo` 全局 Interface 迁移到统一的 `internal/admin/scheduler` 工厂模式 + `SchedulerRegistry`。旧的 `cmd/migrate-*`、`cmd/test-*` 辅助命令已全部移除，`cmd/` 仅保留 `server/`。`robfig/cron` 依赖已移除。旧的 `topicanalysis`/`topicextraction` 包已合并进 `tagmanagement`。`TOPIC_ANALYSIS_*` 环境变量与 `ai_analysis.go` 已删除。

这些架构变更已在代码中落地，但 `docs/reference/` 下的文档未同步更新，存在 11 类不一致（详见 proposal.md）。

**代码权威事实（实现时以此为准）：**

| 事实 | 权威来源 | 值 |
|------|----------|-----|
| Go 版本 | `backend-go/go.mod` | `go 1.25.0` |
| `cmd/` 子目录 | `ls backend-go/cmd/` | 仅 `server/` |
| `internal/` 业务域 | `ls backend-go/internal/` | `admin`、`app`、`models`、`platform`、`reader`、`tagmanagement`、`topicgraph` |
| `platform/` 子包 | `ls backend-go/internal/platform/` | `airouter`、`aisettings`、`config`、`database`、`jsonutil`、`logging`、`middleware`、`testutil`、`tracing`、`ws` |
| reader 服务文件 | `ls backend-go/internal/reader/service/` | `feed_service.go`、`firecrawl_service.go`、`content_completion_service.go`、`rss_parser.go`、`firecrawl_config.go` |
| 注册的调度器 | `runtime.go` 的 `registry.Register` 调用 | 9 个（见下） |
| `robfig/cron` | `go.mod` + 源码 import | 不存在 |
| `TOPIC_ANALYSIS_*` / `parseEnvInt` / `ai_analysis.go` | `grep -rn ... backend-go/internal/` | 全部零命中（已删除） |
| tagmanagement worker 入口 | `runtime.go` | `tagging.StartAllWorkers()` / `tagging.StopAllWorkers()`（根包无 `Start*Worker` 函数，细节在子包） |
| migrator 注册表 | `backend-go/internal/platform/database/migrator.go` | 见下（`topic_analysis_jobs` 不在其中） |

**`runtime.go` 实际注册的 9 个调度器：**

`auto_refresh`、`preference_update`、`content_completion`（持久化名 `ai_summary`）、`firecrawl`、`blocked_article_recovery`、`daily_report`、`tag_quality_score`、`log_cleanup`、`aux_label_cleanup`。

**`migrator.go` 实际 AutoMigrate 的表（节选，与文档相关）：**

`Category`、`Feed`、`Article`、`TopicTag`、`SemanticLabel`、`TopicTagSemanticLabel`、`TopicTagBoardLabel`、`BoardComposition`、`TopicTagEmbedding`、`TopicTagAnalysis`、`TopicAnalysisCursor`、`ArticleTopicTag`、`TagMergeSuggestion`、`TopicTagRelation`、`SchedulerTask`、`AISettings`、`EmbeddingConfig`、`EmbeddingQueue`、`MergeReembeddingQueue`、`AIProvider`、`AIRoute`、`AIRouteProvider`、`AICallLog`、`ReadingBehavior`、`UserPreference`、`FirecrawlJob`、`TagJob`、`NarrativeSummary`、`NarrativeBoard`。

注意：**无 `TopicAnalysisJob` / `topic_analysis_jobs`**——DATABASE_FIELDS.md 主表区引用的 `topicanalysis.topicAnalysisJobRecord` 已废弃。

## Goals / Non-Goals

**Goals:**

- 修正 `architecture/` 下 5 个文档（backend、overview、runtime、tracing、data-flow）
- 修正 `development.md` 与 `开发执行规范.md`
- 修正 `api/schedulers.md`
- 修正 `configuration.md`（删除整段 TOPIC_ANALYSIS 调优）
- 修正 `database/DATA_LIFECYCLE.md` 与 `database/DATABASE_FIELDS.md`

**Non-Goals:**

- 不修改任何代码或 API
- 不创建新的文档文件
- 不更新 `docs/` 下非 `reference/` 的其他文档（如 `experience/`、`plans/`）
- 不修改已核查无过时引用的文档：`architecture/frontend.md`、`deployment.md`、`testing.md`、`database/ER_DIAGRAM.md`、`database/_index.md`、`api/_conventions.md`、`api/_index.md`、`api/` 其余 14 个端点文档

## Decisions

### 1. 以代码为准，不以旧文档为准

**决策**：当文档与代码冲突时，以代码为准更新文档。

**理由**：代码是可运行的真实系统。`backend-go/internal/` 的实际目录结构、`runtime.go` 的 `registry.Register` 调用、`migrator.go` 的 AutoMigrate 列表、`go.mod` 的 Go 版本声明都是权威来源。

### 2. 调度器列表以 `runtime.go` 的 `registry.Register` 为唯一权威

**决策**：`backend.md`、`runtime.md`、`api/schedulers.md`、`DATA_LIFECYCLE.md` 四处调度器引用必须与 `runtime.go` 逐一对应，不参考旧文档列表。

**理由**：原文档列表恰好混入了不存在的 `auto_tag_merge`、`narrative_summary`、`tag_hierarchy_cleanup`、`digest`。直接抄旧文档会继承错误。

### 3. configuration.md 删除而非保留 TOPIC_ANALYSIS 段

**决策**：删除整段"Topic Analysis 调优"（含 `REDIS_URL` 描述中"Topic 分析任务队列"表述一并核对），不保留"已废弃"说明。

**理由**：4 个环境变量、`parseEnvInt`/`parseEnvFloat` 函数、`ai_analysis.go` 文件在代码中完全不存在（grep 零命中），保留只会误导。实现时需核实 `REDIS_URL` 当前是否仍被代码读取及其真实用途。

### 4. database 文档聚焦"代码引用准确性"，不重写表结构

**决策**：`DATABASE_FIELDS.md` / `DATA_LIFECYCLE.md` 只修正"引用已删除调度器/Go 包/模型"的部分，不重新核对全部字段定义。

**理由**：表结构以数据库实际 schema 为准，本次扫描未发现字段级错误，只发现调度器/包路径引用错误。全量字段核对属另一范畴。

## Risks / Trade-offs

- **[风险] 更新可能遗漏细节** → 缓解：tasks.md 第 11 节提供覆盖全部 11 个文档的 grep 零命中验证命令
- **[风险] 调度器列表可能再次过时** → 缓解：在文档中注明"以 `runtime.go` 为准"
- **[风险] configuration.md 的 `REDIS_URL` 真实用途需实现时核实** → 缓解：tasks 中标注为"实现时确认"，不臆断
- **[风险] DATA_LIFECYCLE.md 数据流图修正涉及业务理解** → 缓解：以 `runtime.go` 真实注册的调度器为边界，移除幽灵调度器引用，不臆造新流程

## Migration Plan

1. 按文件组顺序更新：architecture（backend → overview → runtime → tracing → data-flow）→ development → 开发执行规范 → api/schedulers → configuration → database
2. 每个文件更新后，检查内部链接是否有效
3. 全部更新后，执行 tasks.md 第 11 节的 grep 验证，确认零残留
4. 调度器列表与 `runtime.go` 逐一对照

## Open Questions

（无）
