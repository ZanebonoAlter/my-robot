## Purpose

扩展 log-cleanup 调度器，新增清理 `scheduler_execution_logs` 过期记录的职责。基于现有 `log-cleanup` spec（`openspec/specs/log-cleanup/spec.md`）的增量修改。

## Requirements

### Requirement: Scheduler execution log cleanup
`LogCleanupScheduler` SHALL 在每次执行时额外清理 `scheduler_execution_logs` 表中 started_at 超过 30 天的记录。

#### Scenario: Execution logs older than 30 days are deleted
- **WHEN** LogCleanupScheduler 执行清理
- **THEN** 删除 `scheduler_execution_logs` 中 `started_at < NOW() - 30 days` 的所有记录，并记录删除行数

#### Scenario: Execution logs within retention are preserved
- **WHEN** LogCleanupScheduler 执行清理，且某条 execution log 的 started_at 在 30 天内
- **THEN** 该记录不被删除

#### Scenario: Cleanup reports execution log deletion count
- **WHEN** LogCleanupScheduler 执行完成
- **THEN** `GetStatus()` 返回的 `last_execution_result` 包含 `scheduler_execution_logs_deleted` 计数，与现有 `ai_call_logs_deleted` 和 `otel_spans_deleted` 并列

### Requirement: Execution log started_at index
`scheduler_execution_logs` 表 SHALL 在 `started_at` 列上建立 btree 索引以支持高效范围删除。

#### Scenario: Index used for cleanup query
- **WHEN** cleanup DELETE 执行在 `scheduler_execution_logs` 上
- **THEN** 查询使用 `started_at` 索引（非顺序扫描）

### Requirement: Unified scheduler interface compliance
`LogCleanupScheduler` SHALL 实现统一的 `Scheduler` 接口（含 Name, DisplayName, Description, CronExpression, Start, Stop, Trigger, GetStatus），替代当前 ticker-based 模式。

#### Scenario: Migrated from ticker to cron
- **WHEN** 系统启动
- **THEN** LogCleanupScheduler 使用 cron 表达式 `0 3 * * *`（每天凌晨 3 点）注册到共享 cron 实例，而非独立的 `time.NewTicker`

#### Scenario: DisplayName and Description in Chinese
- **WHEN** 调用 `DisplayName()` 和 `Description()`
- **THEN** 分别返回 "日志清理" 和 "清理过期的 AI 调用日志和追踪数据"

#### Scenario: Startup delay removed
- **WHEN** LogCleanupScheduler 启动并注册到 cron
- **THEN** 不再有 5 分钟延迟（原 ticker 模式的 initial delay），首次执行由 cron 表达式决定（凌晨 3:00）
