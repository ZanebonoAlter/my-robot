## Purpose

统一的调度器注册表，替代 `runtimeinfo/schedulers.go` 中的 9 个 `interface{}` 全局变量，提供类型安全的调度器访问、生命周期管理和 cron 表达式配置。

## Requirements

### Requirement: Unified Scheduler interface
系统 SHALL 定义 `Scheduler` 接口，包含以下方法：`Name()`, `DisplayName()`, `Description()`, `CronExpression()`, `SetCronExpression(string)`, `IsRunning()`, `IsExecuting()`, `Start(*cron.Cron)`, `Stop()`, `Trigger()`, `GetStatus() SchedulerStatusResponse`。所有定时任务 SHALL 实现此接口。

#### Scenario: All schedulers implement interface
- **WHEN** 系统启动并初始化所有 9 个调度器（auto_refresh, preference_update, content_completion, firecrawl, tag_quality_score, log_cleanup, daily_report, aux_label_cleanup, blocked_article_recovery）
- **THEN** 每个调度器都实现完整的 `Scheduler` 接口，可通过注册表类型安全地访问

#### Scenario: DisplayName returns Chinese name
- **WHEN** 调用 `auto_refresh` 调度器的 `DisplayName()`
- **THEN** 返回 "订阅源刷新"

#### Scenario: Description returns Chinese description
- **WHEN** 调用 `daily_report` 调度器的 `Description()`
- **THEN** 返回 "为所有活跃版块生成每日报告"

### Requirement: Scheduler registry
系统 SHALL 在 `jobs` 包中提供 `SchedulerRegistry`，支持 `Register(s Scheduler)`, `Get(name string) (Scheduler, bool)`, `All() []Scheduler` 操作。注册表 SHALL 替代 `runtimeinfo` 包中的所有全局变量。

#### Scenario: Register and retrieve scheduler
- **WHEN** `SchedulerRegistry.Register(autoRefresh)` 被调用，然后 `registry.Get("auto_refresh")`
- **THEN** 返回该调度器实例和 `true`

#### Scenario: Retrieve unknown scheduler
- **WHEN** `registry.Get("nonexistent")` 被调用
- **THEN** 返回 `nil, false`

#### Scenario: List all schedulers
- **WHEN** `registry.All()` 被调用
- **THEN** 返回所有已注册调度器的列表，包含 blocked_article_recovery

#### Scenario: runtimeinfo package removed
- **WHEN** 注册表实现完成
- **THEN** `runtimeinfo/schedulers.go` 文件被删除，所有 `runtimeinfo.SetXxx()` 和 `runtimeinfo.GetXxx()` 调用被替换为注册表访问

### Requirement: SchedulerStatusResponse extended
`SchedulerStatusResponse` 结构体 SHALL 新增 `display_name` (string), `description` (string), `cron_expression` (string) 字段。

#### Scenario: Status response includes new fields
- **WHEN** `GET /api/schedulers/auto_refresh` 被调用
- **THEN** 响应包含 `display_name: "订阅源刷新"`, `description: "定时刷新 RSS 订阅源，获取最新文章"`, `cron_expression: "* * * * *"`

#### Scenario: Backward compatibility for check_interval
- **WHEN** 调度器使用 cron 表达式 `*/5 * * * *`
- **THEN** `check_interval` 字段仍返回 `300`（从 cron 表达式推导的秒数），保持前端向后兼容

### Requirement: Cron expression update API
系统 SHALL 提供 `PUT /api/schedulers/:name/cron` 端点，接受 JSON body `{"expression": "0 */2 * * *"}`，在运行时更新调度器的 cron 表达式。

#### Scenario: Valid expression update
- **WHEN** `PUT /api/schedulers/auto_refresh/cron` body=`{"expression": "*/5 * * * *"}`
- **THEN** 调度器从 cron 实例中移除旧 job，注册新 job，更新 `scheduler_tasks` 表的 `cron_expression` 字段，返回 200 和新的调度器状态

#### Scenario: Invalid expression rejected
- **WHEN** `PUT /api/schedulers/auto_refresh/cron` body=`{"expression": "invalid"}`
- **THEN** 返回 400 错误，原表达式不变

#### Scenario: Update unknown scheduler
- **WHEN** `PUT /api/schedulers/nonexistent/cron` body=`{"expression": "*/5 * * * *"}`
- **THEN** 返回 404 错误

#### Scenario: Concurrent trigger during reschedule
- **WHEN** 调度器正在执行中，同时收到 cron 表达式更新请求
- **THEN** 等待当前执行完成后应用新表达式（或立即应用但不影响正在执行的 run），不产生竞态条件

### Requirement: BlockedArticleRecovery API registration
`blocked_article_recovery` 调度器 SHALL 注册到调度器注册表和 API 路由，支持状态查询和手动触发。

#### Scenario: Status query for blocked article recovery
- **WHEN** `GET /api/schedulers/blocked_article_recovery` 被调用
- **THEN** 返回该调度器的状态（name, status, is_executing 等），不再返回 404

#### Scenario: Manual trigger for blocked article recovery
- **WHEN** `POST /api/schedulers/blocked_article_recovery/trigger` 被调用
- **THEN** 执行一次阻止文章恢复并返回结果

### Requirement: Dependency injection for domain services
领域包（`content`, `preferences` 等）SHALL NOT 直接导入 `app/runtimeinfo`。需要调度器状态的领域服务通过构造函数注入 `SchedulerStatusChecker` 接口获取。

#### Scenario: Content service checks scheduler status via interface
- **WHEN** `content.ContentCompletionService` 需要检查 `content_completion` 调度器是否正在执行
- **THEN** 通过注入的 `SchedulerStatusChecker.IsSchedulerExecuting("content_completion")` 查询，而非直接访问全局变量

#### Scenario: No domain-to-app import
- **WHEN** 注册表实现完成
- **THEN** `domain/` 包下的所有文件都不包含对 `app/runtimeinfo` 的导入
