## Purpose

调度器的执行结果必须端到端可见：每个调度器的执行产物（如「清理了 N 条日志」「恢复 N 篇文章」）SHALL 持久化到数据库、SHALL 通过 API 可读、SHALL 在前端展示。本能力修复当前「结果只在内存、重启即丢」「前端不展示详情」「部分调度器在 API 不可见」的问题。
## Requirements
### Requirement: 所有调度器持久化执行结果
全部 9 个调度器（auto_refresh, preference_update, content_completion, firecrawl, tag_quality_score, log_cleanup, daily_report, aux_label_cleanup, blocked_article_recovery）SHALL 在执行后将 `JobResult`（含 `Data` 计数与 `Summary`）写入 `scheduler_tasks.last_execution_result` 字段。SHALL NOT 有任何调度器在注册时缺少持久化配置。

#### Scenario: log_cleanup 结果持久化
- **WHEN** `log_cleanup` 调度器执行完成，删除了 42 条 `ai_call_logs` 和 10 条 `otel_spans`
- **THEN** `scheduler_tasks` 中 `log_cleanup` 行的 `last_execution_result` 包含 `last_ai_call_logs_deleted=42`、`last_otel_spans_deleted=10`

#### Scenario: aux_label_cleanup 结果持久化
- **WHEN** `aux_label_cleanup` 执行完成，禁用了 5 个辅助标签
- **THEN** 对应 `scheduler_tasks` 行的 `last_execution_result` 包含 `affected_count=5`

#### Scenario: blocked_article_recovery 结果持久化
- **WHEN** `blocked_article_recovery` 执行完成，恢复了 3 篇文章
- **THEN** 对应 `scheduler_tasks` 行的 `last_execution_result` 包含 `recovered_count=3`

#### Scenario: firecrawl 结果持久化
- **WHEN** `firecrawl` 执行完成，处理了 8 个任务（6 成功 2 失败）
- **THEN** 对应 `scheduler_tasks` 行的 `last_execution_result` 包含 `completed=6, failed=2, total=8`

#### Scenario: 重启后仍可见上次结果
- **WHEN** `log_cleanup` 执行后服务重启，前端再次加载调度器状态
- **THEN** `last_execution_result` 仍显示上次清理计数（SHALL NOT 因重启丢失）

### Requirement: preference_update 返回实际计数
`preference_update` 调度器的 `JobResult.Data` SHALL 包含本次实际更新的偏好项数（如 `updated_count`），SHALL NOT 返回空对象。

#### Scenario: 偏好更新报告计数
- **WHEN** `preference_update` 执行，更新了 7 项偏好
- **THEN** `JobResult.Data` 包含 `updated_count=7`，`Summary` 包含该数值，并持久化到 `last_execution_result`

### Requirement: BlockedArticleRecovery 纳入 API
`blocked_article_recovery` SHALL 出现在 `GET /api/schedulers` 的返回列表中，并支持通过 `POST /api/schedulers/blocked_article_recovery/trigger` 手动触发。

#### Scenario: 列表包含 blocked_article_recovery
- **WHEN** 调用 `GET /api/schedulers`
- **THEN** 返回数据包含 `name="blocked_article_recovery"` 的条目，含状态与上次执行信息

#### Scenario: 手动触发 blocked_article_recovery
- **WHEN** 调用 `POST /api/schedulers/blocked_article_recovery/trigger`
- **THEN** 执行一次阻塞文章恢复并返回触发结果（accepted/started）

### Requirement: 前端展示上次执行详情
调度器状态面板 SHALL 为每个调度器展示：上次执行时间、执行耗时、结果摘要（人类可读的中文计数，如「清理了 N 条日志」）、失败次数与最近错误信息（仅异常时）。

#### Scenario: 展示上次执行时间与耗时
- **WHEN** 调度器卡片渲染，且 `database_state.last_execution_time` 存在
- **THEN** 显示「上次执行：YYYY-MM-DD HH:MM:SS  耗时 N.Ns」

#### Scenario: 展示结果摘要计数
- **WHEN** 调度器卡片渲染，且 `last_run_summary` 含计数字段
- **THEN** 按 `name` 映射为中文结果文案（如 `log_cleanup` → 「清理了 N 条 AI 日志、M 条追踪」，`aux_label_cleanup` → 「清理了 N 个辅助标签」，`blocked_article_recovery` → 「恢复了 N 篇文章」）

#### Scenario: 展示失败与错误（仅异常时）
- **WHEN** 调度器 `failed_executions > 0` 或 `consecutive_failures > 0`
- **THEN** 显示「失败：N 次」与最近错误信息（`last_error`）；无失败时 SHALL NOT 显示该区块

#### Scenario: 执行中状态
- **WHEN** 调度器 `is_executing=true`
- **THEN** 状态区显示「执行中…」

#### Scenario: 未识别的结果字段回退
- **WHEN** `last_run_summary` 含未识别的字段
- **THEN** 回退显示原始 `Summary` 文案（SHALL NOT 显示空白）

### Requirement: 调度器状态体现分析暂停语义
当 analysis_paused 为 true 时，受影响的分析类调度器在 GET /api/schedulers 返回中 SHALL 体现"暂停"状态语义（如 status 含 paused 标记或暂停说明），使暂停态在调度器可观测面板端到端可见。

#### Scenario: 暂停时调度器状态可见
- **WHEN** analysis_paused 为 true 且调用 GET /api/schedulers
- **THEN** content_completion 等受影响调度器的状态条目体现暂停语义（如显示 paused 标记）

#### Scenario: 恢复后调度器状态复原
- **WHEN** analysis_paused 切回 false
- **THEN** 受影响调度器状态恢复正常 idle/running 语义

