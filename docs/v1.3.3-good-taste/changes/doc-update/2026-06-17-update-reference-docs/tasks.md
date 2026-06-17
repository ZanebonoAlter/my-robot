## 1. 更新 backend.md

- [x] 1.1 修正"技术栈":Go 版本改为 `1.25`(对齐 `go.mod` 的 `go 1.25.0`)
- [x] 1.2 修正"技术栈":移除 `robfig/cron`(`go.mod` 与源码均无;定时任务由 `internal/admin/scheduler` 工厂 + Interval 实现)
- [x] 1.3 修正"当前目录现实"中的 `cmd/` 树:仅保留 `server/`,移除 `migrate-db/`、`migrate-embedding-queue/`、`migrate-tags/`、`test-embedding/`
- [x] 1.4 修正"当前目录现实"中的 `internal/` 树:移除 `internal/domain/`、`internal/jobs/`、`internal/app/runtimeinfo/`,使用 `admin/`、`reader/`、`tagmanagement/`、`topicgraph/`、`models/`、`app/`、`platform/`
- [x] 1.5 修正"分层职责 > `internal/platform/`":移除不存在的 `ai/`、`opennotebook/`(实际子包以 `ls backend-go/internal/platform/` 为准)
- [x] 1.6 移除"当前真实入口"中的 `runtimeinfo/schedulers.go` 引用,以及正文中所有 `runtimeinfo` 描述(改为 `SchedulerRegistry`)
- [x] 1.7 修正调度器列表为 `runtime.go` 实际注册的 9 个(移除不存在的 `auto_tag_merge`、`narrative_summary`)
- [x] 1.8 移除"Tag 合并"段中 `AutoTagMerge 调度器基于 pgvector 余弦相似度 > 0.97 自动触发`(该调度器不存在;当前只有手动 `HardMergeTags` + `tag_quality_score` 重算)

## 2. 更新 overview.md

- [x] 2.1 修正目录树注释中的 `cmd/`:仅 `server/`
- [x] 2.2 修正目录树:移除 `internal/domain/`、`internal/jobs/`、`internal/app/runtimeinfo/`,使用正确包路径
- [x] 2.3 修正"核心子系统"段:移除 `internal/domain/feed/`、`internal/domain/article/`、`internal/domain/content/`、`internal/domain/preferences/`、`internal/domain/narrative/` 引用(叙事归入 `tagmanagement` 域,无独立 `narrative` 包)
- [x] 2.4 移除 `NarrativeSummaryScheduler` 引用(行 81、89)
- [x] 2.5 修正"后台调度器一览"与"统一调度器管理":从"8 类"改为 9,移除 `AutoTagMerge` 行(行 192),对齐 `runtime.go`
- [x] 2.6 修正技术栈表"定时任务"行:移除 `robfig/cron`,改为 `internal/admin/scheduler`;移除 platform 注释中的 `ai`、`opennotebook`(行 152)

## 3. 更新 runtime.md

- [x] 3.1 重写"运行时共享状态怎么暴露"段:从 `runtimeinfo/schedulers.go` 全局 Interface 模式改为 `SchedulerRegistry`(`registry.Register` / `StartAll` / `StopAll`)模式
- [x] 3.2 移除所有已删除的全局 Interface 变量名(`AutoRefreshSchedulerInterface` 等 7 个)
- [x] 3.3 修正 worker 启动描述:移除 `topicextraction.GetTagQueue().Start()`、`topicanalysis.StartEmbeddingQueueWorker()`、`topicanalysis.StartMergeReembeddingQueueWorker()`(行 36-38),改为 `tagging.StartAllWorkers()` 统一入口
- [x] 3.4 调度器列表与默认间隔对齐 `runtime.go`(9 个,移除 `AutoTagMerge`、`narrative_summary`、`tag_hierarchy_cleanup`)
- [x] 3.5 修正"推荐阅读顺序"中 `internal/jobs/handler.go`、`internal/jobs/content_completion.go`、`internal/jobs/narrative_summary.go`、`internal/jobs/tag_hierarchy_cleanup.go`(行 238-241)为真实路径

## 4. 更新 tracing.md

- [x] 4.1 修正"已落地的自动注入方法"表文件路径:`internal/domain/feeds/service.go` → `internal/reader/service/feed_service.go`;`internal/domain/contentprocessing/firecrawl_service.go` → `internal/reader/service/firecrawl_service.go`;`internal/domain/contentprocessing/content_completion_service.go` → `internal/reader/service/content_completion_service.go`
- [x] 4.2 移除 `narrative_summary` 调度器引用(行 98)

## 5. 更新 data-flow.md

- [x] 5.1 修正"每日叙事生成"段:移除 `NarrativeSummaryScheduler 触发`(行 241),对齐当前真实触发链路(实现时以代码为准确认 narrative_boards 的触发方式)

## 6. 更新 development.md

- [x] 6.1 修正"前置条件":Go 版本改为 `1.25+`
- [x] 6.2 删除或重写"辅助工具命令"表(`migrate-digest`、`test-digest`、`migrate-tags`、`migrate-db` 均不存在;`backend-go/cmd/` 只有 `server`)
- [x] 6.3 修正"后端目录约定"表:移除 `internal/domain/`、`internal/domain/models/`,使用 `internal/reader/`、`internal/tagmanagement/`、`internal/topicgraph/`、`internal/admin/`、`internal/models/`;移除 platform 列表中的 `ai`、`opennotebook`(行 236)
- [x] 6.4 修正测试示例路径:`go test ./internal/domain/feeds` → `go test ./internal/reader/service`(行 117、123、258、285)

## 7. 更新 开发执行规范.md

- [x] 7.1 修正测试规范引用:`internal/domain/feeds/service_test.go` → `internal/reader/service/feed_service_test.go`
- [x] 7.2 修正后端分层约定:"业务逻辑在 `internal/domain/*`" → 按业务域(`internal/reader/`、`internal/tagmanagement/`、`internal/topicgraph/`、`internal/admin/`)
- [x] 7.3 修正架构审查项中的 `internal/domain/*` + `internal/platform/*` 表述
- [x] 7.4 移除 `cmd/migrate-db/` 引用(行 321)

## 8. 更新 api/schedulers.md

- [x] 8.1 删除 3 个幽灵调度器:`digest`、`narrative_summary`、`tag_hierarchy_cleanup`
- [x] 8.2 补齐 4 个缺失调度器:`daily_report`、`log_cleanup`、`aux_label_cleanup`、`blocked_article_recovery`
- [x] 8.3 核对每个调度器的别名(`content_completion` 别名 `ai_summary`)与说明对齐 `runtime.go`

## 9. 更新 configuration.md

- [x] 9.1 删除整段"Topic Analysis 调优"(4 个 `TOPIC_ANALYSIS_*` 环境变量表 + `parseEnvInt`/`parseEnvFloat` + `internal/domain/topicanalysis/ai_analysis.go` 引用,代码中全部不存在)
- [x] 9.2 核实 `REDIS_URL` 描述:实现时确认 Redis 是否仍被代码读取及其真实用途,修正"Topic 分析任务队列"表述

## 10. 更新 database 文档

- [x] 10.1 `DATA_LIFECYCLE.md`:移除数据流图中的 `AutoTagMerge 调度器 (3600s)`(行 134)、`capability='narrative_summary'`(行 275)、`NarrativeSummaryScheduler 调度器需启用`(行 308),对齐当前真实调度器
- [x] 10.2 `DATABASE_FIELDS.md`:将 `topic_analysis_jobs` 主表行(行 29)标注为废弃(无 migrator 注册、模型 `topicanalysis.topicAnalysisJobRecord` 不存在),或移至"无 Go 代码引用"区
- [x] 10.3 `DATABASE_FIELDS.md`:修正 `internal/domain/models/` → `internal/models/`(行 712)

## 11. 验证

- [x] 11.1 路径零残留：`grep -rn 'internal/domain\|internal/jobs\|runtimeinfo' docs/reference/` → 零命中
- [x] 11.2 幽灵依赖/包零残留：`grep -rn 'robfig\|opennotebook' docs/reference/` → 零命中；`grep -rn 'platform/ai/' docs/reference/` → 零命中
- [x] 11.3 幽灵命令零残留：`grep -rn 'migrate-db\|migrate-tags\|migrate-embedding-queue\|migrate-digest\|test-digest\|test-embedding' docs/reference/` → 零命中
- [x] 11.4 Go 版本一致：`grep -rn 'Go 1.2[0-4]' docs/reference/` → 零命中
- [x] 11.5 幽灵调度器零残留：`grep -rn 'auto_tag_merge\|narrative_summary\|tag_hierarchy_cleanup\|NarrativeSummaryScheduler\|AutoTagMerge' docs/reference/` → 零命中
- [x] 11.6 旧包名零残留：`grep -rn 'contentprocessing\|topicanalysis\|topicextraction\|topictypes\|aiadmin' docs/reference/` → 零命中（仅保留废弃标注）
- [x] 11.7 `backend.md` 的 `internal/platform/` 子包列表与 `ls backend-go/internal/platform/` 一致
- [x] 11.8 调度器列表一致：`api/schedulers.md`、`architecture/backend.md`、`architecture/runtime.md` 的调度器列表均与 `runtime.go` 的 9 个 `registry.Register` 调用逐一对应
- [x] 11.9 `configuration.md` 不再出现 `TOPIC_ANALYSIS` 或 `ai_analysis.go`
- [x] 11.10 文档内部链接有效
