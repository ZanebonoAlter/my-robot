## 1. Unified Scheduler Interface

- [ ] 1.1 Define `Scheduler` interface in `backend-go/internal/jobs/scheduler.go` with methods: `Name()`, `DisplayName()`, `Description()`, `CronExpression()`, `SetCronExpression(string) error`, `IsRunning()`, `IsExecuting()`, `Start(*cron.Cron)`, `Stop()`, `Trigger() error`, `GetStatus() SchedulerStatusResponse`
- [ ] 1.2 Update `SchedulerStatusResponse` struct in `backend-go/internal/jobs/handler.go` to add `display_name`, `description`, and `cron_expression` fields
- [ ] 1.3 Write unit tests for the interface contract in `backend-go/internal/jobs/scheduler_test.go`

## 2. SchedulerRegistry

- [ ] 2.1 Create `SchedulerRegistry` struct in `backend-go/internal/jobs/registry.go` with `Register(Scheduler)`, `Get(name string) (Scheduler, bool)`, `All() []Scheduler`, and `IsSchedulerExecuting(name string) bool`
- [ ] 2.2 Add `SchedulerStatusChecker` interface in `backend-go/internal/jobs/registry.go` (with `IsSchedulerExecuting(name string) bool`) for domain-layer DI
- [ ] 2.3 Write unit tests for registry in `backend-go/internal/jobs/registry_test.go`

## 3. Database Schema

- [ ] 3.1 Add `cron_expression` column to `SchedulerTask` model in `backend-go/internal/domain/models/ai_models.go` (`gorm:"size:100"`; nullable, empty means use code default)
- [ ] 3.2 Create `SchedulerExecutionLog` model in a new file `backend-go/internal/domain/models/scheduler_models.go` with columns: `id` (bigserial PK), `scheduler_name` (varchar(50)), `started_at`, `finished_at` (nullable), `status` (varchar(20): running/success/failed), `error_message` (text), `result_summary` (jsonb), `trigger_source` (varchar(20): cron/manual), `duration_ms` (integer)
- [ ] 3.3 Register `SchedulerExecutionLog` in `backend-go/internal/platform/database/migrator.go` `RunAutoMigrate` model list
- [ ] 3.4 Create versioned migration file in `backend-go/internal/platform/database/` adding indexes: `(scheduler_name, started_at DESC)`, `(scheduler_name, status)`, `(started_at)` on `scheduler_execution_logs`

## 4. Migrate Schedulers to Cron + Interface

- [ ] 4.1 Migrate `AutoRefreshScheduler` (`backend-go/internal/jobs/auto_refresh.go`) — implement `Scheduler` interface, replace `@every 60s` with default cron `* * * * *`
- [ ] 4.2 Migrate `PreferenceUpdateScheduler` (`backend-go/internal/jobs/preference_update.go`) — convert from `time.NewTicker` loop to cron with default `*/30 * * * *`
- [ ] 4.3 Migrate `ContentCompletionScheduler` (`backend-go/internal/jobs/content_completion.go`) — implement `Scheduler` interface, replace `@every 60s` with default `* * * * *`
- [ ] 4.4 Migrate `FirecrawlScheduler` (`backend-go/internal/jobs/firecrawl.go`) — implement `Scheduler` interface, replace `@every 300s` with default `*/5 * * * *`
- [ ] 4.5 Migrate `TagQualityScoreScheduler` (`backend-go/internal/jobs/tag_quality_score.go`) — convert from `time.NewTicker` loop to cron with default `0 * * * *`
- [ ] 4.6 Migrate `LogCleanupScheduler` (`backend-go/internal/jobs/log_cleanup.go`) — implement `Scheduler` interface, keep `0 3 * * *`; add cleanup of `scheduler_execution_logs` older than 30 days
- [ ] 4.7 Migrate `DailyReportScheduler` (`backend-go/internal/jobs/daily_report.go`) — implement `Scheduler` interface, replace `@every` with default `0 21 * * *`
- [ ] 4.8 Migrate `AuxLabelCleanupScheduler` (`backend-go/internal/jobs/aux_label_cleanup.go`) — convert from `time.NewTicker` loop to cron with default `0 * * * *`
- [ ] 4.9 Migrate `BlockedArticleRecovery` (`backend-go/internal/jobs/blocked_article_recovery.go`) — convert from `time.NewTicker` loop to cron with default `*/30 * * * *`
- [ ] 4.10 Add `initSchedulerTask()` to each scheduler: on startup, persist default cron expression to DB if `cron_expression` column is empty; populate `check_interval` from `secondsFromCron(expr)` for backward compat

## 5. Execution Logging

- [ ] 5.1 Create `ExecutionLogger` in `backend-go/internal/jobs/execution_logger.go` — wraps writing `SchedulerExecutionLog` rows on job start/finish
- [ ] 5.2 Instrument each scheduler's `Trigger()` to call `ExecutionLogger.Start()` / `ExecutionLogger.Finish()` around the core work
- [ ] 5.3 Write unit tests for `ExecutionLogger` in `backend-go/internal/jobs/execution_logger_test.go`

## 6. Fix Layer Inversion — Remove runtimeinfo

- [ ] 6.1 Add `SchedulerStatusChecker` parameter to `content` domain handlers in `backend-go/internal/domain/content/content_completion_handler.go` — replace `runtimeinfo.ContentCompletionSchedulerInterface` type assertion with injected `SchedulerStatusChecker`
- [ ] 6.2 Add `SchedulerStatusChecker` parameter to `preferences` domain handlers in `backend-go/internal/domain/preferences/handler.go` — replace `runtimeinfo.PreferenceUpdateSchedulerInterface` with injected checker
- [ ] 6.3 Update `backend-go/internal/app/runtime.go` — create `SchedulerRegistry`, register all schedulers, pass registry (or `SchedulerStatusChecker` interface) to domain handler constructors
- [ ] 6.4 Delete `backend-go/internal/app/runtimeinfo/schedulers.go` and remove the `runtimeinfo` package
- [ ] 6.5 Update `backend-go/internal/jobs/handler.go` — replace `schedulerDescriptor.Get()` closures (which read `runtimeinfo.XxxInterface`) with `SchedulerRegistry.Get(name)` lookups
- [ ] 6.6 Update tests: `backend-go/internal/jobs/handler_test.go`, `backend-go/internal/domain/preferences/handler_test.go`, `backend-go/internal/domain/content/content_completion_handler_test.go` — inject `SchedulerRegistry` or mocks instead of setting `runtimeinfo` vars

## 7. Chinese Descriptions

- [ ] 7.1 Add `DisplayName()` and `Description()` to all 9 schedulers, returning Chinese strings (e.g., `AutoRefresh` → DisplayName: "订阅源刷新", Description: "定时刷新 RSS 订阅源，获取最新文章")
- [ ] 7.2 Update `schedulerDescriptors()` in `backend-go/internal/jobs/handler.go` to include `DisplayName` and `Description` from each scheduler's interface methods (or remove static descriptors in favor of registry)

## 8. Register BlockedArticleRecovery

- [ ] 8.1 Add `BlockedArticleRecovery` entry to `schedulerDescriptors()` (or equivalent registry-based listing) in `backend-go/internal/jobs/handler.go` so it appears in `GET /api/schedulers` status and is triggerable via API

## 9. API Updates

- [ ] 9.1 Add `PUT /api/schedulers/:name/cron` handler in `backend-go/internal/jobs/handler.go` — validates cron expression via `robfig/cron/v3` parser, calls `scheduler.SetCronExpression()`, re-registers job with shared cron instance, persists to DB
- [ ] 9.2 Add `GET /api/schedulers/:name/executions` handler in `backend-go/internal/jobs/handler.go` — returns paginated execution history from `scheduler_execution_logs`
- [ ] 9.3 Register new routes in `backend-go/internal/app/router.go`
- [ ] 9.4 Update `GetSchedulersStatus` response to include `display_name`, `description`, `cron_expression` fields

## 10. Verify

- [ ] 10.1 `cd backend-go && golangci-lint run ./...`
- [ ] 10.2 `cd backend-go && go vet ./...`
- [ ] 10.3 `cd backend-go && go test ./internal/jobs/... ./internal/domain/preferences/... ./internal/domain/content/... ./internal/app/...`
- [ ] 10.4 `cd backend-go && go build ./...`
