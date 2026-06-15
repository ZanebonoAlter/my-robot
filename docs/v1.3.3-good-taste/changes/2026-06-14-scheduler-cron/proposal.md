## Why

定时任务系统当前有四个被证实的功能缺陷（重构后的残留问题）：

1. **`next_run` 字段谎报** — `BaseScheduler.Start()` 把内存里的 `nextRun` 设为「当前时刻」，但首次真实触发要等到 `now + interval`（或 `now + startupDelay`）。前端若展示「下次执行」永远显示「即将开始」，与实际严重不符。
2. **DailyReport 永远不会自动执行** — DailyReport 配置为 `Interval: 86400s`（24 小时）且 `StartupDelay: 0`。意味着只有在「服务连续运行满 24 小时」时才会触发一次。任何重启（更新、部署、崩溃）都会让计数清零。用户实际一直靠手动「执行」按钮出日报。且时间不可配置，无法实现「每天 21:00」。
3. **4 个调度器的执行结果不落库** — `log_cleanup`、`aux_label_cleanup`、`blocked_article_recovery`、`firecrawl` 在 `runtime.go` 注册时未传 `Persistence`。它们的 `JobResult`（如「清理了 N 条日志」「恢复 N 篇文章」）只活在内存，服务重启即丢失。
4. **前端调度器面板几乎不展示信息** — 当前 `SchedulerStatusPanel.vue` 仅渲染名称 + 状态点 + 执行按钮，完全不展示 `last_execution_time`、`last_error`、`failed_executions`、`last_run_summary`。后端已返回这些数据，前端未消费。此外 `BlockedArticleRecovery` 在 `runtime.go` 已注册运行，但 `schedulerDescriptors()` 漏了它，导致 API 不返回、前端不可见、无法手动触发。

## What Changes

- **修复 `next_run` 计算**：`BaseScheduler` 在 `Start()` 时将 `nextRun` 设为真实的首次触发时刻（`now + startupDelay`，或 `now + interval`），而非「现在」。
- **DailyReport 定点执行**：`BaseScheduler` 新增可选的 `NextRun func(now time.Time) time.Time` 配置；DailyReport 用它计算「下一个 21:00」并 sleep 到该时刻，每次执行后重新计算。服务重启后从当前时刻重新计算下一次，不再依赖「连续运行 24 小时」。
- **日报时间可配置**：日报的执行时刻（默认 21:00）存入 `AISettings`（key=`daily_report_time`，value=`"HH:MM"`），复用现有全局配置基础设施与设置 UI 框架。
- **补齐 4 个调度器的持久化**：`log_cleanup`、`aux_label_cleanup`、`blocked_article_recovery`、`firecrawl` 注册时传入 `Persistence`，使 `JobResult` 写入 `scheduler_tasks.last_execution_result`。
- **`preference_update` 补充数值**：当前 `JobResult.Data` 为空 `{}`，改为返回实际更新的偏好计数。
- **BlockedArticleRecovery 纳入 API**：在 `schedulerDescriptors()` 添加条目，使其出现在 `GET /api/schedulers` 列表、可手动触发。
- **前端展示上次执行详情**：`SchedulerStatusPanel.vue` 为每个调度器展示：上次执行时间、上次结果摘要（解析 `last_run_summary` 的计数，如「清理了 N 条」「恢复 N 篇」）、失败次数与错误信息。

## Capabilities

### New Capabilities

- `scheduler-accuracy`：调度器时间准确——`next_run` 反映真实下次触发时刻；DailyReport 在可配置的墙钟时刻执行（默认 21:00），且服务重启后仍能按时触发。
- `scheduler-observability`：调度器执行结果端到端可见——所有调度器持久化执行结果、`BlockedArticleRecovery` 进入 API、前端展示上次执行时间/结果摘要/失败信息。

## Impact

**后端**
- `backend-go/internal/admin/scheduler/base.go`：修复 `nextRun` 初始化；新增 `NextRun` 配置项与对应的 sleep-to-next 调度循环分支。
- `backend-go/internal/admin/scheduler/job_daily_report.go` / `backend-go/internal/app/runtime.go`：DailyReport 接入 `NextRun`，读取 `AISettings["daily_report_time"]`。
- `backend-go/internal/platform/database/postgres_migrations.go`：新增版本化迁移，seed `daily_report_time="21:00"` 默认值。
- `backend-go/internal/platform/aisettings/config_store.go`：新增 `daily_report_time` 的加载/保存函数（或复用通用 key-value 访问）。
- `backend-go/internal/admin/scheduler/job_preference_update.go`：`JobResult.Data` 返回实际计数。
- `backend-go/internal/app/runtime.go`：为 4 个缺持久化的调度器补 `Persistence`。
- `backend-go/internal/admin/handler/scheduler_handler.go`：`schedulerDescriptors()` 添加 `blocked_article_recovery`。

**前端**
- `front/app/components/dialog/SchedulerStatusPanel.vue`：新增上次执行时间、结果摘要、失败/错误展示区块。

**数据库**：无 schema 变更（复用 `scheduler_tasks` 与 `AISettings`，不新增表、不新增列）；新增一条版本化迁移用于 seed `daily_report_time` 默认值（与现有 `ai_settings` key 的约定一致）。

## Note on Change Name

本变更目录名为 `scheduler-cron`，源自最初设计（全部调度器迁移到 `robfig/cron/v3`）。经 6 月 11 日重构（工厂模式 + Registry），调度器基础设施已重构完毕，原提案的「cron 引擎」「统一大接口」「删 runtimeinfo」均已以不同方式完成或不再需要。本次重写将范围收窄到上述四个被证实的功能缺陷，**不再引入 cron 库**。目录名保留以避免破坏里程碑文档中的历史引用（见 `docs/v1.3.3-good-taste/.../2026-06-11-code-cleanup-dead/design.md`），但实质内容已与「cron」无关。
