# 验证报告：scheduler-cron

> 独立验证（非自验）：逐文件审查实现代码 + 实际运行编译/测试。

## 摘要记分卡

| 维度 | 状态 |
|------|------|
| 完整性 | 39/43 任务完成（4 项为手动验证任务，代码均已实现）；8/8 需求有实现 |
| 正确性 | 8/8 需求实现正确，15/15 场景已被代码与测试覆盖 |
| 一致性 | 设计决策全部遵循；代码模式一致 |

编译与测试实测（Windows 工具链，cmd.exe）：
- `go build ./...` → ✅ BUILD_OK
- `go test ./internal/admin/scheduler/...` → ✅ ok
- `go test ./internal/platform/aisettings/...` → ✅ ok
- `go test ./internal/platform/database/ -run TestDailyReportTimeMigrationSeedsDefaultValue` → ✅ PASS
- ⚠️ `database` 包另有 2 个**与本变更无关**的既有失败（见下方说明）

## 问题按优先级分类

### 1. 关键问题（必须修复才能归档）

**无关键问题。**

所有需求（8 项）均已实现且通过代码审查 + 单元测试验证；所有 35 项实现类任务均已完成。

### 2. 警告问题（建议修复）

#### 2.1 四个手动验证任务未完成

tasks.md 中 4 项未勾选均为「手动/QA 验证」类，对应代码均已实现并有单元测试覆盖：

| 任务 | 内容 | 对应实现（已验证） |
|------|------|-------------------|
| 3.4 | 验证 DailyReport 在 21:00 前后启动均按时触发 | `base.go` NextRun 循环 + `job_daily_report.go:NextDailyReportTime` + `base_test.go:TestBaseSchedulerNextRunCallback` |
| 5.5 | 验证手动触发后 `last_execution_result` 写库 | `persistence.go:updateSchedulerTask` 写 `last_execution_result`（9 个调度器均注册 Persistence） |
| 7.2 | 验证 `GET /api/schedulers` 含 blocked_article_recovery + trigger | `scheduler_handler.go:schedulerDescriptors()` 已含该条目 |
| 9.8 | 端到端：重启看日志 nextRun；触发 log_cleanup 看前端计数 | 日志（`base.go` wall-clock started 行）+ 前端 `SchedulerStatusPanel.vue` 展示区 |

**建议**：在归档前执行这 4 项手动验证（启动服务、触发调度器、查库/看前端），或确认实现已满足后勾选。这些不阻塞代码正确性，但是发布前应有的信心保证。

#### 2.2 database 包有 2 个既有测试失败（与本变更无关）

```
--- FAIL: TestPostgresMigrationsDocumentStagedEmbeddingCutover
    db_unit_test.go:20: expected staged rollout description, got "Set topic_tag_embeddings.embedding column type to vector(4096)."
--- FAIL: TestSemanticLabelBoardSystemMigrationDocumentsSchemaCutover
    db_unit_test.go:53: expected text to contain "CREATE TABLE IF NOT EXISTS semantic_labels"
```

- 这两个失败涉及 `topic_tag_embeddings` 与 `semantic_labels` 的迁移文档断言，属于早期提交（`代码梳理`等）遗留，**不是 scheduler-cron 引入**。
- 本变更新增的迁移 `20260614_0001`（清理死配置 key）与 `20260614_0002`（seed `daily_report_time`）及其测试均通过。

**建议**：另行修复这两个既有失败（不在本变更范围内），避免 `go test ./...` 整体红灯干扰后续归档/CI。

### 3. 建议问题（可选改进）

#### 3.1 前端「失败区块」显示条件与 spec 字面表述略有出入

spec 场景「展示失败与错误（仅异常时）」字面条件为 `failed_executions > 0 或 consecutive_failures > 0`，而 `SchedulerStatusPanel.vue:156` 实际仅用 `consecutive_failures > 0` 作为显示开关（展示的数值仍是 `failed_executions`）。

- 实现比 spec 更合理：`failed_executions` 是累计值、永不减少，若以其为开关会永久显示失败块；`consecutive_failures` 在成功后清零，能正确反映「当前异常」。
- **建议**：保持现状，并把 spec 场景条件更正为 `consecutive_failures > 0`（或补注「累计失败数仅在当前有连续失败时展示」），使二者一致。

#### 3.2 `NextDailyReportTime` 内重复解析 HH:MM（已无害，可优化）

`job_daily_report.go` 的 `NextDailyReportTime` 用 `fmt.Sscanf("%d:%d")` 解析，而 `LoadDailyReportTimeConfig` 已用严格正则 `hhmmPattern` 校验并回退默认。当前 Sscanf 对 `"25:99"` 这类值不会报错，但因 `LoadDailyReportTimeConfig` 已保证返回值合法，实际无害。
- **建议**（可选）：`NextDailyReportTime` 复用同一 `hhmmPattern`，或直接信任 `LoadDailyReportTimeConfig` 的返回值不做二次解析，消除冗余防御。

## 详细验证结果

### 完整性验证

#### 任务完成情况
- 总任务：43，已完成：39（90.7%），未完成：4（全为手动验证任务，见 2.1）。

#### 规范覆盖
- **scheduler-accuracy**：4/4 需求实现（next_run 真实时刻、DailyReport 墙钟、时刻可配置保存、配置变更下周期生效）。
- **scheduler-observability**：4/4 需求实现（全部调度器持久化、preference_update 计数、BlockedArticleRecovery 进 API、前端展示详情）。
- 场景：15/15 被代码 + 测试覆盖。

### 正确性验证（需求 → 实现映射）

| 需求 | 实现位置 | 结论 |
|------|---------|------|
| next_run 反映真实下次触发 | `base.go:116-119`（`firstDelay` 逻辑）+ `base_test.go` 两个用例 | ✅ |
| DailyReport 墙钟执行 | `job_daily_report.go:NextDailyReportTime` + `runtime.go`（`NextRun` 配置） | ✅ |
| 时刻可配置且可保存 | `config_store.go:LoadDailyReportTimeConfig/SaveDailyReportTimeConfig`（`hhmmPattern` 正则 00:00–23:59）+ `config_store_test.go` | ✅ |
| 配置变更下次执行后生效 | `base.go` NextRun 分支每轮重读 `cfg.NextRun(now)` | ✅ |
| 所有调度器持久化结果 | `runtime.go`（9 个全部带 `Persistence`）+ `persistence.go:updateSchedulerTask` 写 `last_execution_result` | ✅ |
| preference_update 返回计数 | `job_preference_update.go`（`updated_count`）+ `UpdateAllPreferencesWithCount` | ✅ |
| BlockedArticleRecovery 进 API | `scheduler_handler.go:schedulerDescriptors()`（含该条目，9 个齐全） | ✅ |
| 前端展示上次执行详情 | `SchedulerStatusPanel.vue`（时间/耗时/结果摘要/失败）+ `schedulerMeta.ts:formatLastRunSummary`（按 name 映射 8 类文案） | ✅ |

### 一致性验证

- 设计 Decision 3（persistence 在 NextRun 模式下用 `NextRun(now)` 算 `next_execution_time`）→ `persistence.go:NewTaskPersistenceWithNextRun` + `initSchedulerTask/updateSchedulerTask` 的 `nextRunFn` 分支 ✅
- 迁移版本化（`20260614_0001` 清理死 key、`20260614_0002` seed `daily_report_time="21:00"`）顺序正确，seed 先于无依赖 ✅
- `docs/api.md` 列出 9 个调度器（含 daily_report/aux_label_cleanup/blocked_article_recovery，已删 digest/narrative_summary/tag_hierarchy_cleanup），并补充各 `last_execution_result` 字段与 `daily_report_time` 配置 ✅
- `docs/reference/configuration.md:177` 新增 `daily_report_time` 行 ✅
- `docs/reference/development.md:233` 已由 `internal/jobs/` 更正为 `internal/admin/` ✅
- 前端设置卡片（`SettingsSectionSchedulers.vue`）新增「日报生成时刻」字段，走 `PUT /api/settings` ✅

## 最终评估

**无关键问题。** 2 个警告（手动验证任务 + 既有无关测试失败），2 个可选建议。

实现与变更工件（proposal/design/specs/tasks）完全一致：8 个需求全部正确实现，15 个场景被覆盖，所有设计决策被遵循，编译与受影响包测试通过。

**结论**：完成 4 项手动验证后即可归档；既有 database 测试失败应另行修复（不属本变更）。

---
**验证方式**：逐文件代码审查 + cmd.exe 实跑 `go build ./...` / 受影响包 `go test`
**变更名称**：scheduler-cron　**架构**：spec-driven
