## 1. 修复 `next_run` 计算（scheduler-accuracy）

- [x] 1.1 在 `backend-go/internal/admin/scheduler/base.go` 的 `Start()` 中，将 `nextRun` 初始化改为真实首次触发时刻：有 `StartupDelay` 时 = `now + startupDelay`，否则 = `now + interval`（当前代码在 `StartupDelay=0` 时错误地设为 `now`）
- [x] 1.2 验证 `runJob()` 后的 `updateNextRun(time.Now().Add(s.cfg.Interval))` 在 interval 模式下已正确（无需改动）
- [x] 1.3 单元测试 `base_test.go`：`StartupDelay=0` 时 `nextRun == now+interval`（允许 ±1s 误差）；`StartupDelay>0` 时 `nextRun == now+startupDelay`

## 2. 可选墙钟调度 `Config.NextRun`（scheduler-accuracy）

- [x] 2.1 在 `backend-go/internal/admin/scheduler/base.go` 的 `Config` 新增字段 `NextRun func(now time.Time) time.Time`（可选；提供时忽略 `Interval`/`StartupDelay`）
- [x] 2.2 在 `Start()` 增加 `NextRun` 分支：循环「`next := cfg.NextRun(now)` → 同步 `s.nextRun` → `time.Until(next)` sleep → `runJob()`」，每轮重读 `NextRun` 以反映配置变更
- [x] 2.3 `Stop()` 与现有 ticker 分支一致（通过 `stopChan` 中断 sleep）
- [x] 2.4 单元测试 `base_test.go`：用 fake `NextRun`（返回 `now+短延迟`）验证触发时刻与 `nextRun` 字段准确

## 3. DailyReport 接入墙钟调度（scheduler-accuracy）

- [x] 3.1 在 `backend-go/internal/admin/scheduler/job_daily_report.go` 或新文件添加 `NextDailyReportTime(now time.Time) time.Time`：读取配置时刻 → 计算「今天该时刻」（若已过则明天）
- [x] 3.2 在 `backend-go/internal/app/runtime.go` 将 DailyReport 的 `Config` 从 `Interval: 86400s` 改为 `NextRun: scheduler.NextDailyReportTime`
- [x] 3.3 让 DailyReport 的 `Persistence.InitTask`/`UpdateTask` 在 `NextRun` 模式下用 `NextDailyReportTime(now)` 计算 `next_execution_time`（而非 `now+interval`）——可能需在 `TaskPersistence` 或 `persistence.go` 增加对 `NextRun` 的感知
- [x] 3.4 验证：服务在 21:00 前启动 → 21:00 触发；21:00 后启动 → 次日 21:00 触发（重启不丢调度）

## 4. 可配置日报时刻（scheduler-accuracy）

- [x] 4.1 在 `backend-go/internal/platform/aisettings/config_store.go` 新增 `LoadDailyReportTimeConfig() (string, error)`（读 `ai_settings` key=`daily_report_time`，不存在时返回默认 `"21:00"`）与 `SaveDailyReportTimeConfig(value string) error`
- [x] 4.2 校验 `HH:MM` 格式（`00:00`–`23:59`）：读取时非法值回退默认并 `logging.Warnf`；保存时非法返回 error
- [x] 4.3 `NextDailyReportTime` 调用 `LoadDailyReportTimeConfig` 获取时刻
- [x] 4.4 后端测试：默认值回退、合法值解析、非法值回退
- [x] 4.5 前端：在 `settings-workspace` 复用现有设置卡片框架，新增「日报生成时刻」字段（时间选择器或 `HH:MM` 输入），调用现有 `PUT /api/settings` 保存

## 5. 补齐 4 个调度器持久化（scheduler-observability）

- [x] 5.1 `backend-go/internal/app/runtime.go`：`log_cleanup` 注册时添加 `Persistence: admin.NewTaskPersistence("log_cleanup", "清理过期的 AI 调用日志和追踪数据")`
- [x] 5.2 同上为 `aux_label_cleanup` 添加 `Persistence`（name=`aux_label_cleanup`, description=`清理无活跃标签引用的辅助标签`）
- [x] 5.3 同上为 `blocked_article_recovery` 添加 `Persistence`（name=`blocked_article_recovery`, description=`恢复被阻塞的文章`）
- [x] 5.4 同上为 `firecrawl` 添加 `Persistence`（name=`firecrawl`, description=`自动爬取文章全文`）
- [x] 5.5 验证：手动触发后 `scheduler_tasks` 对应行写入 `last_execution_result`（含 `last_ai_call_logs_deleted`/`affected_count`/`recovered_count`/`completed` 等）

## 6. `preference_update` 补充数值（scheduler-observability）

- [x] 6.1 在 `backend-go/internal/admin/scheduler/job_preference_update.go` 让 `PreferenceUpdateJob` 返回实际更新的计数（需 `PreferenceUpdateService` 暴露本次更新项数）
- [x] 6.2 `JobResult.Data` 填入 `updated_count`（或同等字段），`Summary` 改为 `fmt.Sprintf("updated %d preferences", n)`

## 7. BlockedArticleRecovery 纳入 API（scheduler-observability）

- [x] 7.1 在 `backend-go/internal/admin/handler/scheduler_handler.go` 的 `schedulerDescriptors()` 添加 `blocked_article_recovery` 条目（含 `Name`/`DisplayName`/`Description`/`Get` 闭包）
- [x] 7.2 验证：`GET /api/schedulers` 返回该条目；`POST /api/schedulers/blocked_article_recovery/trigger` 可手动触发（依赖现有 `TriggerNow`，无需新代码）

## 8. 前端展示上次执行详情（scheduler-observability）

- [x] 8.1 在 `front/app/utils/schedulerMeta.ts` 新增 `formatLastRunSummary(name: string, summary: SchedulerLastRunSummary | null): string`：按 name 映射到中文结果文案（如 `log_cleanup` → `清理了 N 条日志、M 条追踪`，`aux_label_cleanup` → `清理了 N 个辅助标签`，`blocked_article_recovery` → `恢复了 N 篇文章`，未识别字段回退 `summary.reason`）
- [x] 8.2 在 `front/app/components/dialog/SchedulerStatusPanel.vue` 每个调度器卡片新增展示区：「上次执行时间 + 耗时」（`database_state.last_execution_time` + `last_execution_duration`）、「结果摘要」（`formatLastRunSummary`）、「失败次数 + 错误」（`failed_executions` / `consecutive_failures` / `last_error`，仅异常时显示）
- [x] 8.3 确保轮询（`useSchedulerStatus`）刷新后展示区更新；`is_executing=true` 时显示「执行中…」

## 10. 数据库迁移（daily_report_time 默认值 seed）

- [x] 10.1 在 `backend-go/internal/platform/database/postgres_migrations.go` 新增版本化迁移（如 `20260614_0001`），将 `daily_report_time` 登记进 `settingsDefaults` seed：`Key="daily_report_time"`, `Value="21:00"`, `Description="日报生成时刻（HH:MM）"`（复用 `postgres_migrations.go:263-274` 的 seed 模式）
- [x] 10.2 确认 `LoadDailyReportTimeConfig`（task 4.1）在 key 缺失时仍回退默认 `21:00`（迁移 seed + 代码回退双重保险）
- [x] 10.3 在 `backend-go/internal/platform/database/db_unit_test.go` 补断言：迁移后 `daily_report_time` 行存在且值为 `21:00`（参照现有 `deprecatedKey` 断言模式，约 db_unit_test.go:105）

## 11. 文档更新

- [x] 11.1 `docs/user-guide/schedulers/api.md`：解决遗留的 git merge conflict marker（`<<<<<<< Updated upstream` 开头）；更新「支持的调度器」列表为当前 9 个——新增 `daily_report`、`aux_label_cleanup`、`blocked_article_recovery`，删除已废弃的 `digest`、`narrative_summary`、`tag_hierarchy_cleanup`
- [x] 11.2 `docs/user-guide/schedulers/api.md`：补充各调度器 `last_execution_result` 计数字段说明（如 `log_cleanup` 的 `last_ai_call_logs_deleted`/`last_otel_spans_deleted`，`aux_label_cleanup` 的 `affected_count`，`blocked_article_recovery` 的 `recovered_count`）；新增 `daily_report_time` 配置项说明
- [x] 11.3 `docs/reference/configuration.md`：在配置项表（约 :174）新增 `daily_report_time` 行
- [x] 11.4 `docs/reference/development.md:233`：将 `internal/jobs/` 更正为 `internal/admin/scheduler/`（顺带检查该文件其它过时路径引用，如有一并修正）

## 9. Verify

- [x] 9.1 `cd backend-go && golangci-lint run ./internal/admin/... ./internal/platform/aisettings/...`
- [x] 9.2 `cd backend-go && go vet ./internal/admin/... ./internal/platform/aisettings/...`
- [x] 9.3 `cd backend-go && go test ./internal/admin/scheduler/... ./internal/platform/aisettings/... ./internal/platform/database/...`
- [x] 9.4 `cd backend-go && go build ./...`
- [x] 9.5 `cd front && pnpm lint`
- [x] 9.6 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`
- [x] 9.7 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"`
- [x] 9.8 手动验证：重启服务 → 观察日志中 DailyReport 的 `nextRun` 为「今天/明天 21:00」；手动触发 `log_cleanup` → 前端显示「清理了 N 条」
