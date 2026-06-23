## Context

本变更最初设计于 2026-06-08，设想「全部调度器迁移到 `robfig/cron/v3` + 统一大接口 + 删 `runtimeinfo`」。2026-06-11 的后端重构（`工厂模式后端改造`、`架构架构`、`代码梳理`）已完成：

- `runtimeinfo/schedulers.go` 包已删除（9 个 `interface{}` 全局变量消除）。
- `internal/jobs/` 不复存在，调度器迁至 `internal/admin/scheduler/`。
- 统一的 `BaseScheduler` 工厂 + 极简 `Scheduler` 接口（`Start/Stop/TriggerNow/UpdateInterval/ResetStats`）+ `Registry` 已建立。
- 中文显示名由前端 `schedulerMeta.ts` 维护（已是中文），无需后端 `DisplayName()`。

因此原提案的架构性目标**已达成或不再适用**。重写后的范围聚焦四个被代码证据证实的功能缺陷（详见 `proposal.md`）。重构采用的调度模型是 `time.NewTicker` + 固定 `Interval`，本变更**保持该模型**，只对其做两处修复：`next_run` 计算与可选的墙钟调度。

## 证据基线

| 问题 | 代码位置 | 证据 |
|------|---------|------|
| `next_run` 谎报 | `scheduler/base.go:109-113` | `Start()` 设 `nextRun = time.Now()`，但首个 ticker 触发在 `now + interval` |
| DailyReport 永不自动跑 | `app/runtime.go:148`（`Interval: 86400s`，无 `StartupDelay`） | 24h ticker，重启即清零 |
| 4 调度器无持久化 | `app/runtime.go:92,99,106,160` | 注册时未传 `Persistence` |
| `preference_update` 无数值 | `scheduler/job_preference_update.go:17-19` | `JobResult.Data` 为空 `{}` |
| BlockedArticleRecovery 不可见 | `handler/scheduler_handler.go` `schedulerDescriptors()` | 列表中无此条目（runtime 已注册） |
| 前端不展示详情 | `components/dialog/SchedulerStatusPanel.vue` | 仅渲染 name + status + 执行按钮 |

## Goals

- `next_run` 字段在任何时刻都等于「下次真实触发时刻」。
- DailyReport 在可配置的墙钟时刻（默认 21:00）执行；服务重启后仍按时触发。
- 所有 9 个调度器的 `JobResult` 持久化到 `scheduler_tasks.last_execution_result`。
- `BlockedArticleRecovery` 出现在 `GET /api/schedulers` 并可手动触发。
- 前端调度器面板展示每个调度器的上次执行时间、结果摘要计数、失败次数与错误信息。

## Non-Goals

- **不引入** `robfig/cron/v3` 或任何第三方 cron 库——只有 DailyReport 需要定点，用轻量的 `NextRun` 回调即可。
- **不新增** `scheduler_execution_logs` 历史表——只要「上次执行」，`scheduler_tasks` 现有字段足够。
- **不新增** `cron_expression` 列或 `PUT /:name/cron` 接口——只有日报需要可配置，用 `AISettings` key-value 即可。
- **不重构** `BaseScheduler` 的核心调度模型（保持 ticker-based）。
- **不改动** `Scheduler` 接口形状（保持重构后的小接口）。
- **不做** 多实例分布式锁（单用户部署）。
- 不迁移现有 `scheduler_tasks` 的聚合计数（继续工作）。

## Decisions

### 1. 修复 `next_run` 计算（适用于所有 interval 调度器）

`base.go` 的 `Start()` 中：

```go
// 现状（谎报）
nextRun := time.Now()
if s.cfg.StartupDelay > 0 {
    nextRun = nextRun.Add(s.cfg.StartupDelay)
}
s.nextRun = &nextRun   // 但 StartupDelay=0 时 = now，而真实触发在 now+interval
```

修复为反映真实首次触发时刻：

```go
firstDelay := s.cfg.Interval
if s.cfg.StartupDelay > 0 {
    firstDelay = s.cfg.StartupDelay   // 有启动延迟时，首次触发 = now + startupDelay
}
nextRun := time.Now().Add(firstDelay)
s.nextRun = &nextRun
```

`runJob()` 后的 `updateNextRun` 已正确（`time.Now().Add(s.cfg.Interval)`），无需改动。

### 2. 可选墙钟调度：`Config.NextRun`

`BaseScheduler.Config` 新增可选字段：

```go
type Config struct {
    // ... 现有字段 ...
    Interval     time.Duration
    StartupDelay time.Duration
    // NextRun 可选。若提供，调度器忽略 Interval/StartupDelay，
    // 改为「计算下次触发时刻 → sleep 到该时刻 → 执行 → 重新计算」循环。
    // 用于 DailyReport 等需要墙钟定点的场景。
    NextRun func(now time.Time) time.Time
}
```

`Start()` 的循环分支：

```
if cfg.NextRun != nil:
    循环 {
        next := cfg.NextRun(time.Now())      // 每次重新读，可反映配置变更
        sleepUntil(next)                      // nextRun 字段同步更新
        runJob()
    }
else:
    // 现有 ticker + interval 逻辑（已修复 nextRun）
```

`NextRun` 每次循环重新调用，因此**配置变更在下次执行后生效**（最迟一个周期，对日报为 24h）。重启服务可立即生效。这是可接受的取舍——不做 sleep 中断以保持简单。

### 3. DailyReport 墙钟计算

```go
func nextDailyReportTime(now time.Time) time.Time {
    hhmm := loadDailyReportTimeSetting()   // 读 AISettings["daily_report_time"]，默认 "21:00"
    h, m := parseHHMM(hhmm)                // "21:00" → 21, 0
    today := time.Date(now.Year(), now.Month(), now.Day(), h, m, 0, 0, now.Location())
    if now.Before(today) {
        return today                        // 今天的 21:00 还没到
    }
    return today.Add(24 * time.Hour)        // 已过，用明天的
}
```

DailyReport 注册改为：

```go
// runtime.go
dailyReportBase := scheduler.New(scheduler.Config{
    Name:     "Daily Report",
    NextRun:  nextDailyReportTime,          // 替代 Interval: 86400s
    Job:      admin.DailyReportJob(),
    Persistence: admin.NewTaskPersistence("daily_report", "..."),
})
```

注意：`Persistence.InitTask` 当前用 `interval time.Duration` 计算 `next_execution_time`。DailyReport 改用 `NextRun` 后，DB 的 `next_execution_time` 应改由 `NextRun` 计算（`InitTask`/`UpdateTask` 需兼容 `NextRun` 模式，或 DailyReport 的 persistence 用专门逻辑）。

### 4. 可配置时间存储：`AISettings` key-value

```
表: ai_settings (已存在)
key: "daily_report_time"
value: "21:00"   (HH:MM 格式)
description: "日报生成时刻"
```

- 复用现有 `AISettings` 模型与 `GET/PUT /api/settings` 接口（`config_store.go` 已有 `LoadFirecrawlConfig` 等同类函数）。
- 新增 `LoadDailyReportTimeConfig()` / `SaveDailyReportTimeConfig()` 到 `config_store.go`（或用通用 key-value 读写）。
- **默认值通过版本化迁移 seed**（与现有所有 `ai_settings` key 的约定一致，见 `postgres_migrations.go` 的 `settingsDefaults`）：新增迁移将 `daily_report_time="21:00"` 写入 DB，用户与运维可直接查询到默认值。
- **代码读取时仍保留回退逻辑**（双重保险）：若 key 因任何原因缺失，`LoadDailyReportTimeConfig` 回退 `"21:00"`。
- 校验：`HH:MM` 格式，`00:00`–`23:59`，非法值回退默认并记日志；保存时非法值返回错误。
- 前端复用 `settings-workspace` 的设置卡片框架，新增一个「日报时刻」字段（输入或时间选择器）。

### 5. 补齐 4 个调度器的持久化

`runtime.go` 为以下 4 个调度器添加 `Persistence`（与现有 `auto_refresh`/`daily_report` 等同款 `NewTaskPersistence`）：

| 调度器 | `NewTaskPersistence` name | description |
|--------|---------------------------|-------------|
| `log_cleanup` | `log_cleanup` | 清理过期的 AI 调用日志和追踪数据 |
| `aux_label_cleanup` | `aux_label_cleanup` | 清理无活跃标签引用的辅助标签 |
| `blocked_article_recovery` | `blocked_article_recovery` | 恢复被阻塞的文章 |
| `firecrawl` | `firecrawl` | 自动爬取文章全文 |

补齐后，它们的 `JobResult.Data`（如 `last_ai_call_logs_deleted`、`affected_count`、`recovered_count`、`completed/failed`）会通过现有 `updateSchedulerTask` 写入 `last_execution_result` JSON，前端可读取。

### 6. `preference_update` 补充数值

`job_preference_update.go` 当前：

```go
return &JobResult{Data: map[string]interface{}{}, Summary: "preferences updated successfully"}, nil
```

改为返回实际计数（需 `PreferenceUpdateService` 暴露更新的用户/偏好数量）：

```go
return &JobResult{
    Data: map[string]interface{}{
        "updated_count": n,        // 本次更新的偏好项数（具体字段名取决于 service 能提供什么）
    },
    Summary: fmt.Sprintf("updated %d preferences", n),
}, nil
```

### 7. BlockedArticleRecovery 纳入 API

`scheduler_handler.go` 的 `schedulerDescriptors()` 添加：

```go
{
    Name:        "blocked_article_recovery",
    DisplayName: "Blocked Article Recovery",
    Description: "Recover articles stuck in blocked state",
    Get: func() interface{} {
        s, _ := Reg.Get("blocked_article_recovery")
        return s
    },
},
```

无需其它改动——`BaseScheduler` 已实现 `TriggerNow()`，`ResolveScheduler` 会自动支持手动触发。

### 8. 前端展示上次执行详情

`SchedulerStatusPanel.vue` 在每个调度器卡片的状态行下方，新增展示区（数据均来自现有 `database_state` / `last_run_summary`，后端已返回）：

```
┌─────────────────────────────────────────────┐
│ ● 后台刷新                      [空闲] [执行]│
│   auto_refresh                              │
│─────────────────────────────────────────────│
│ 上次执行：2026-06-14 21:03:12  耗时 1.2s    │
│ 结果：刷新 12 个订阅源（扫描 48 / 到期 12） │ ← 解析 last_run_summary
│ 失败：0 次                                  │ ← failed_executions (仅 >0 时显示)
└─────────────────────────────────────────────┘
```

实现要点：
- 「上次执行」：`database_state.last_execution_time`（已是 CST 格式化字符串）+ `last_execution_duration`。
- 「结果」：解析 `last_run_summary`（JSON）。不同调度器的字段不同（`triggered_feeds` / `affected_count` / `recovered_count` / `ai_call_logs_deleted` / `completed` / `report_count` …），用 `schedulerMeta.ts` 新增一个 `formatLastRunSummary(name, summary)` 函数按 name 映射到中文文案。
- 「失败」：`database_state.failed_executions`，仅 >0 时显示；`consecutive_failures` >0 时额外显示最近错误 `last_error`。
- 不轮询新接口，复用现有 `useSchedulerStatus` 的轮询。

## Risks and Trade-offs

| 风险 | 缓解 |
|------|------|
| **DailyReport 配置变更有一周期延迟** | `NextRun` 每次循环重读，最迟 24h 生效；重启立即生效。文档注明。如需即时，可后续加 sleep 中断，本次不做。 |
| **`NextRun` 模式与 interval 模式分支增加 BaseScheduler 复杂度** | 分支清晰隔离；interval 路径完全不变。DailyReport 是唯一使用 NextRun 的调度器。 |
| **`AISettings` 时刻被改成非法值** | 读取时校验 `HH:MM`，非法回退默认 `21:00` 并记日志；前端输入用时间选择器限值。 |
| **DailyReport 的 persistence `InitTask`/`UpdateTask` 用 interval 算 next_execution_time** | 需让 persistence 在 `NextRun` 模式下用 `NextRun(now)` 算 `next_execution_time`；或 DailyReport 用独立 persistence 逻辑。见 Decision 3。 |
| **前端解析 `last_run_summary` 的字段分散** | `formatLastRunSummary` 集中映射，未识别字段回退显示原始 Summary 文案。 |

## Migration Plan

本变更无破坏性 schema 变更，分四组独立可部署：

0. **数据库 seed**：版本化迁移登记 `daily_report_time="21:00"` 默认值（应最先执行，使后续组的配置读取有值可读）。
1. **时间修复**（`scheduler-accuracy`）：修 `next_run` 计算 + 加 `NextRun` + DailyReport 接入 + `AISettings` 配置。
2. **结果持久化**（`scheduler-observability` 后端部分）：4 调度器补 Persistence + `preference_update` 计数 + BlockedArticleRecovery 进 descriptor。
3. **前端展示**（`scheduler-observability` 前端部分）：`SchedulerStatusPanel.vue` 新增展示区 + `formatLastRunSummary`。

文档更新可与任意组并行。

无表/列变更（新增一条 seed 默认值的版本化迁移）；无 API 破坏性变更（`SchedulerStatusResponse` 字段不变，只是前端开始消费已有字段）。
