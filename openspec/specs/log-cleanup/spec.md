## Purpose

统一管理数据库日志表的保留与清理，通过 scheduler 系统定时删除过期数据，替代散落在各模块内的手动清理逻辑。

## Requirements

### Requirement: Scheduled log table cleanup
The system SHALL run a `LogCleanupScheduler` that executes every 24 hours and deletes rows older than 7 days from both `ai_call_logs` and `otel_spans` tables, in that order.

#### Scenario: Cron cleanup removes expired rows from both tables
- **WHEN** the scheduler ticker fires (every 24h)
- **THEN** the system deletes all rows from `ai_call_logs` where `created_at < NOW() - 7 days`, followed by all rows from `otel_spans` where `start_time_unix_nano < (NOW() - 7 days).UnixNano()`

#### Scenario: Startup delay before first cleanup
- **WHEN** the scheduler starts
- **THEN** the first execution SHALL occur after a 5-minute delay, not immediately

#### Scenario: No rows to clean
- **WHEN** the cleanup runs and no rows are older than 7 days
- **THEN** the scheduler logs "no rows to clean" and reports zero rows affected

### Requirement: Manual trigger support
The `LogCleanupScheduler` SHALL support manual triggering via the scheduler API (`POST /api/schedulers/log_cleanup/trigger`).

#### Scenario: Manual trigger from API
- **WHEN** a POST request is sent to `/api/schedulers/log_cleanup/trigger`
- **THEN** the cleanup executes immediately and returns the number of rows deleted per table

#### Scenario: Manual trigger while already running
- **WHEN** a manual trigger is requested while a cleanup cycle is executing
- **THEN** the request returns 409 with `accepted: false` and reason `already_running`

### Requirement: ai_call_logs created_at index
The `ai_call_logs` table SHALL have a btree index on the `created_at` column to support efficient range deletes.

#### Scenario: Index exists for cleanup query
- **WHEN** the cleanup DELETE executes on `ai_call_logs`
- **THEN** the query uses the `created_at` index (no sequential scan)

### Requirement: Scheduler status observable via API
The `LogCleanupScheduler` SHALL report its status through `GetStatus()` following the existing scheduler interface, including: status, check_interval, is_executing, last execution time, rows cleaned per table, and any errors.

#### Scenario: Status response includes cleanup metrics
- **WHEN** `GET /api/schedulers/log_cleanup` is called
- **THEN** the response includes `last_ai_call_logs_deleted` and `last_otel_spans_deleted` counts

### Requirement: embedding_queues completed 保留策略

既有 scheduled log cleanup 作业 SHALL 同时清理 `embedding_queues` 中 status='completed' 且创建时间早于保留期（默认 30 天）的行，防止 completed 历史行无限累积。清理查询 MUST 有 created_at（或等价时间列）索引支撑。

#### Scenario: 定时清理移除过期 completed 行
- **WHEN** scheduled cleanup 运行且存在 status='completed'、创建时间 > 30 天的行
- **THEN** 这些行被删除，30 天内的 completed 行保留

#### Scenario: 无过期行时正常空跑
- **WHEN** scheduled cleanup 运行且无过期行
- **THEN** 作业正常完成，不报错
