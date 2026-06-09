## Why

定时任务系统存在四个问题：(1) 全部使用 `@every Ns` 间隔表达式而非标准 cron 表达式，无法精确控制执行时刻（如"每天 21:00"）；(2) 两种调度模式（cron-based vs ticker-based）混杂，接口不统一；(3) 任务执行结果只记录最后一次，无历史日志；(4) 前端展示的任务描述全为英文技术术语，用户无法理解每个任务的功能。

## What Changes

- 将所有定时任务从 `@every Ns` 迁移到标准 cron 表达式（如 `0 21 * * *`）
- 统一调度接口：消除 ticker-based 模式，全部使用 `robfig/cron/v3`
- 定义统一的 `Scheduler` 接口替代当前 9 个 `interface{}` 全局变量
- 新增 `scheduler_execution_logs` 表，记录每次任务执行的历史结果（启动时间、完成时间、状态、错误信息、结果摘要）
- `scheduler_tasks` 表新增 `cron_expression` 字段，替代 `check_interval`
- 前端调度器列表展示中文功能描述（"做了什么 + 为什么 + 执行频率"）
- 标记已废弃任务（如有残留）
- `BlockedArticleRecovery` 注册到 API（当前运行但不可查看/触发）
- 支持通过 API 更新 cron 表达式

## Capabilities

### New Capabilities

- `scheduler-execution-logging`: 定时任务执行历史记录，持久化每次执行的起止时间、状态、错误和结果摘要
- `scheduler-registry`: 统一的调度器注册表，替代 `runtimeinfo` 的 `interface{}` 服务定位，提供类型安全的调度器访问

### Modified Capabilities

- `narrative-board-generation`: （如 code-cleanup-dead 未完成则需处理 narrative_summary 相关清理）
- `log-cleanup`: 新增清理 `scheduler_execution_logs` 超过 N 天的记录

## Impact

- `backend-go/internal/jobs/`：所有调度器文件重写为 cron 表达式 + 统一接口
- `backend-go/internal/app/runtime.go`：简化启动流程，使用 scheduler registry
- `backend-go/internal/app/runtimeinfo/schedulers.go`：替换为 registry 模式
- `backend-go/internal/jobs/handler.go`：API 支持中文描述、cron 表达式更新
- `backend-go/internal/domain/models/`：新增 `SchedulerExecutionLog` 模型
- 数据库迁移：`scheduler_tasks` 增加 `cron_expression` 列；新增 `scheduler_execution_logs` 表
- 前端调度器管理页面：展示中文描述、cron 表达式、执行历史
