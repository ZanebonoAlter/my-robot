## Context

Syntopica runs 9 background schedulers that keep feeds fresh, articles complete, tags scored, and reports generated. The current implementation has accumulated significant technical debt:

1. **Two scheduling modes** — 5 schedulers use `robfig/cron/v3` with `@every Ns` expressions; 4 use raw `time.NewTicker` + goroutine loops. They share no common interface.
2. **Service locator via `interface{}`** — `runtimeinfo/schedulers.go` exports 9 package-level `interface{}` vars. Domain packages (e.g. `content`, `preferences`) import `app/runtimeinfo` to read scheduler state — a layer inversion.
3. **No execution history** — `SchedulerTask` stores aggregate counters (total/success/fail) and only the last execution result. There is no per-execution log, making debugging and trend analysis impossible.
4. **Imprecise scheduling** — `@every Ns` means "run N seconds after the last run ended", not "run at wall-clock time X". Users cannot set "daily report at 21:00".
5. **Unregistered scheduler** — `BlockedArticleRecovery` runs in code but has no API registration, so it's invisible to operators.
6. **English-only descriptions** — Frontend displays raw Go struct field names; Chinese-speaking users cannot understand what each task does.

## Goals

- All schedulers use standard cron expressions via `robfig/cron/v3`.
- Unified `Scheduler` interface replaces all `runtimeinfo` `interface{}` vars.
- Scheduler registry lives in the `jobs` package (not `app` layer).
- Domain handlers access scheduler status via constructor injection, not global vars.
- New `scheduler_execution_logs` table records every execution.
- `scheduler_tasks` gains `cron_expression` column (replaces `check_interval` for scheduling logic).
- Chinese descriptions for all user-facing schedulers.
- `BlockedArticleRecovery` registered in API.
- API supports updating cron expressions at runtime.

## Non-Goals

- Distributed locking or multi-instance coordination (single-user deployment).
- Web UI for cron expression editing (API support only; frontend follows later).
- Migrating existing aggregate counters in `scheduler_tasks` (they continue to work).
- Removing `SchedulerTask` model (it stays for aggregate stats and DB-driven status).

## Decisions

### 1. Unified `Scheduler` Interface

All schedulers implement a single interface defined in `jobs`:

```go
type Scheduler interface {
    Name() string              // e.g. "auto_refresh"
    DisplayName() string       // Chinese: "订阅源刷新"
    Description() string       // Chinese: "定时刷新 RSS 订阅源，获取最新文章"
    CronExpression() string    // current cron expression, e.g. "*/1 * * * *"
    SetCronExpression(string)  // update at runtime (validates before applying)
    IsRunning() bool
    IsExecuting() bool
    Start(*cron.Cron)          // register job with shared cron instance
    Stop()
    Trigger() error            // manual trigger
    GetStatus() SchedulerStatusResponse
}
```

This replaces the two existing patterns:
- **Current cron-based**: `GetStatus() SchedulerStatusResponse` — kept, but struct embeds `Scheduler` interface compliance.
- **Current ticker-based**: `GetStatus() map[string]interface{}` — migrated to return `SchedulerStatusResponse` directly.

The `SchedulerStatusResponse` struct gains `display_name`, `description`, and `cron_expression` fields. The `check_interval` field is retained for backward compatibility but populated from cron expression parsing.

### 2. Registry Pattern in `jobs` Package

A `SchedulerRegistry` struct replaces the 9 `interface{}` vars:

```go
// jobs/registry.go
type SchedulerRegistry struct {
    schedulers map[string]Scheduler
    mu         sync.RWMutex
}

func (r *SchedulerRegistry) Register(s Scheduler)
func (r *SchedulerRegistry) Get(name string) (Scheduler, bool)
func (r *SchedulerRegistry) All() []Scheduler
```

The registry is created in `app/runtime.go` and passed to:
- **Router** — handler receives `*SchedulerRegistry` (not individual schedulers).
- **Domain services** — receive `*SchedulerRegistry` or specific `Scheduler` via constructor injection.

`runtimeinfo/schedulers.go` is deleted entirely. All `runtimeinfo.SetXxx()` / `runtimeinfo.GetXxx()` calls are replaced with registry access.

### 3. Cron Expression Defaults

Each scheduler has a default cron expression stored in code:

| Name | Default Cron | Human-Readable |
|------|-------------|----------------|
| auto_refresh | `* * * * *` | Every minute |
| preference_update | `*/30 * * * *` | Every 30 minutes |
| content_completion | `* * * * *` | Every minute |
| firecrawl | `*/5 * * * *` | Every 5 minutes |
| tag_quality_score | `0 * * * *` | Hourly |
| log_cleanup | `0 3 * * *` | Daily at 03:00 |
| daily_report | `0 21 * * *` | Daily at 21:00 |
| aux_label_cleanup | `0 * * * *` | Hourly |
| blocked_article_recovery | `*/30 * * * *` | Every 30 minutes |

The `scheduler_tasks` table stores the active cron expression (may differ from default if user changed it). On startup, if DB has no expression, the code default is used and persisted.

The `check_interval` column is retained but deprecated — populated as `secondsFromCron(expr)` for backward compatibility with existing frontend code.

### 4. Execution Log Schema

New table `scheduler_execution_logs`:

| Column | Type | Notes |
|--------|------|-------|
| id | bigserial PK | |
| scheduler_name | varchar(50) | FK to scheduler_tasks.name |
| started_at | timestamptz | Job start time |
| finished_at | timestamptz | Job end time (null if running) |
| status | varchar(20) | `running` / `success` / `failed` |
| error_message | text | Error details (null on success) |
| result_summary | jsonb | Structured summary (counts, etc.) |
| trigger_source | varchar(20) | `cron` / `manual` |
| duration_ms | integer | Computed: finished_at - started_at |

Indexes:
- `(scheduler_name, started_at DESC)` — per-scheduler recent history
- `(scheduler_name, status)` — failed execution queries
- `(started_at)` — log cleanup range delete

Retention: `log_cleanup` scheduler deletes rows older than 30 days (configurable).

### 5. Dependency Injection Approach

Current layer inversion:
```
domain/content → app/runtimeinfo (to check if ContentCompletion is running)
domain/preferences → app/runtimeinfo (to check PreferenceUpdate state)
```

Fix:
- `SchedulerRegistry` is created in `app/runtime.go`.
- Domain services that need scheduler state receive a function closure or interface:
  ```go
  type SchedulerStatusChecker interface {
      IsSchedulerExecuting(name string) bool
  }
  ```
- This interface is satisfied by `SchedulerRegistry`. Domain services depend on the interface, not the concrete registry.
- `runtimeinfo` package is removed; all references rewritten.

## Risks and Trade-offs

| Risk | Mitigation |
|------|-----------|
| **Ticker→cron migration changes timing** | Ticker-based schedulers ran on fixed intervals; cron `*/N` runs on wall-clock boundaries. Accept this — it's the desired behavior. |
| **Cron expression validation** | `robfig/cron/v3` parser validates on `AddFunc`. Wrap `SetCronExpression` with parse-before-apply; return error on invalid expressions. |
| **Execution log volume** | At 9 schedulers running minutely, worst case ~13K rows/day. `log_cleanup` with 30-day retention caps at ~390K rows — acceptable for Postgres. |
| **Race during cron reschedule** | When user updates expression via API, remove old job from cron instance, add new one. Hold mutex to prevent concurrent trigger during swap. |
| **Backward compatibility** | `check_interval` field retained in API responses. Frontend migration can happen independently. |

## Migration Plan

1. **Phase 1: Unified interface + registry**
   - Define `Scheduler` interface and `SchedulerRegistry` in `jobs/`.
   - Migrate all 9 schedulers to implement `Scheduler`.
   - Replace `runtimeinfo` vars with registry.
   - Wire registry through DI in `runtime.go`.
   - Delete `runtimeinfo/schedulers.go`.

2. **Phase 2: Cron expressions**
   - Add `cron_expression` column to `scheduler_tasks`.
   - Convert all schedulers from `@every Ns` / ticker to standard cron.
   - Persist default expression on first startup.

3. **Phase 3: Execution logging**
   - Create `scheduler_execution_logs` table.
   - Instrument all schedulers to write log on start/end.
   - Extend `log_cleanup` to include this table.

4. **Phase 4: Chinese descriptions + API**
   - Add `DisplayName()` and `Description()` to all schedulers.
   - Add `blocked_article_recovery` to API registry.
   - Add `PUT /api/schedulers/:name/cron` endpoint.
   - Update `SchedulerStatusResponse` with new fields.

Each phase is independently deployable and testable.
