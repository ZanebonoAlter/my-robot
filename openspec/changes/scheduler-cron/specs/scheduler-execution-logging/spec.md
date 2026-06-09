## Purpose

记录定时任务每次执行的完整历史，包括起止时间、状态、错误信息和结果摘要，替代当前只保留最后一次执行结果的 `SchedulerTask` 聚合统计。

## Requirements

### Requirement: Execution log creation
系统 SHALL 在每次定时任务执行开始时创建 `scheduler_execution_logs` 记录（status=`running`），执行结束时更新该记录的 finished_at、status、error_message、result_summary 和 duration_ms。

#### Scenario: Successful execution creates completed log
- **WHEN** `auto_refresh` 调度器执行一次成功刷新
- **THEN** 系统在执行开始时插入一条 `scheduler_execution_logs` 记录（scheduler_name=`auto_refresh`, status=`running`, trigger_source=`cron`），执行结束时更新该记录 status=`success`, finished_at=当前时间, duration_ms=计算值, result_summary 包含执行统计

#### Scenario: Failed execution creates failed log
- **WHEN** `daily_report` 调度器执行失败并返回错误
- **THEN** 系统更新对应 execution log 的 status=`failed`, error_message=错误信息, finished_at=当前时间, duration_ms=计算值

#### Scenario: Manual trigger records trigger_source
- **WHEN** 用户通过 `POST /api/schedulers/firecrawl/trigger` 手动触发执行
- **THEN** 创建的 execution log 记录 trigger_source=`manual`

### Requirement: Execution log schema
`scheduler_execution_logs` 表 SHALL 包含以下列：id (bigserial PK), scheduler_name (varchar(50)), started_at (timestamptz, NOT NULL), finished_at (timestamptz), status (varchar(20): running/success/failed), error_message (text), result_summary (jsonb), trigger_source (varchar(20): cron/manual), duration_ms (integer)。

#### Scenario: Table created with required columns
- **WHEN** 数据库迁移运行
- **THEN** `scheduler_execution_logs` 表包含 id, scheduler_name, started_at, finished_at, status, error_message, result_summary, trigger_source, duration_ms 列，且 scheduler_name 列有外键约束到 scheduler_tasks(name)

#### Scenario: Indexes for query performance
- **WHEN** 数据库迁移运行
- **THEN** 表上存在索引 (scheduler_name, started_at DESC)、(scheduler_name, status) 和 (started_at)

### Requirement: Execution history API
系统 SHALL 提供 `GET /api/schedulers/:name/executions` 端点，返回指定调度器的最近执行历史，默认按 started_at DESC 排序，支持 `limit` 参数（默认 20，最大 100）。

#### Scenario: Get recent executions
- **WHEN** `GET /api/schedulers/auto_refresh/executions?limit=10`
- **THEN** 返回最近 10 条 execution log 记录，按 started_at 降序排列，包含 id, started_at, finished_at, status, duration_ms, trigger_source, error_message（如有）, result_summary

#### Scenario: Default limit applied
- **WHEN** `GET /api/schedulers/daily_report/executions`（无 limit 参数）
- **THEN** 返回最近 20 条记录

#### Scenario: Limit capped at maximum
- **WHEN** `GET /api/schedulers/auto_refresh/executions?limit=200`
- **THEN** 返回最多 100 条记录，不报错

#### Scenario: Unknown scheduler name
- **WHEN** `GET /api/schedulers/nonexistent/executions`
- **THEN** 返回 404 错误

### Requirement: Execution log retention
`scheduler_execution_logs` 中超过 30 天的记录 SHALL 由 log_cleanup 调度器定期清理。

#### Scenario: Old logs cleaned by log_cleanup
- **WHEN** log_cleanup 调度器执行清理
- **THEN** 删除 `scheduler_execution_logs` 中 started_at < NOW() - 30 天的所有记录

#### Scenario: Recent logs preserved
- **WHEN** log_cleanup 调度器执行清理，且某条记录 started_at 在 30 天内
- **THEN** 该记录不被删除
