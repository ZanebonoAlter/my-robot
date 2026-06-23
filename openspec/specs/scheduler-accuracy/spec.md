## Purpose

调度器的时间表达必须真实且可控。本能力确保：(1) 调度器对外的 `next_run` 始终等于真实的下次触发时刻，而非误导性的「当前时刻」；(2) DailyReport 在可配置的墙钟时刻（默认 21:00）执行，且服务重启后仍能按时触发（不依赖「连续运行满周期」）。

## Requirements

### Requirement: next_run 反映真实下次触发时刻
所有基于固定间隔（`Interval`）的调度器，其 `next_run` 字段 SHALL 等于真实的下次触发时刻：有 `StartupDelay` 时为 `启动时刻 + startupDelay`，否则为 `启动时刻 + interval`。SHALL NOT 在 `StartupDelay=0` 时将 `next_run` 设为「当前时刻」。

#### Scenario: 无启动延迟时 next_run 等于启动时刻加间隔
- **WHEN** `auto_refresh` 调度器（`Interval=60s`，`StartupDelay=0`）在 t=0 启动
- **THEN** 其 `next_run` 等于 `t+60s`（而非 `t=0`）

#### Scenario: 有启动延迟时 next_run 等于启动时刻加延迟
- **WHEN** `log_cleanup` 调度器（`Interval=86400s`，`StartupDelay=5min`）在 t=0 启动
- **THEN** 其 `next_run` 等于 `t+5min`（首次触发时刻），执行后变为 `执行时刻 + interval`

#### Scenario: 前端展示的下次执行时间准确
- **WHEN** 前端读取调度器状态的 `next_run` 字段
- **THEN** 显示的剩余时间与实际下次触发时刻一致，SHALL NOT 永远显示「即将开始」

### Requirement: DailyReport 在可配置墙钟时刻执行
DailyReport 调度器 SHALL 在每天固定的墙钟时刻执行，该时刻通过 `AISettings` 配置（key=`daily_report_time`，值格式 `HH:MM`，默认 `21:00`）。SHALL NOT 使用「从启动时刻起每 24 小时」的固定间隔模式。

#### Scenario: 服务在目标时刻前启动
- **WHEN** 日报时刻配置为 `21:00`，服务在当日 18:00 启动
- **THEN** DailyReport 在当日 21:00 触发执行

#### Scenario: 服务在目标时刻后启动
- **WHEN** 日报时刻配置为 `21:00`，服务在当日 22:00 启动（当日 21:00 已过）
- **THEN** DailyReport 在次日 21:00 触发执行（SHALL NOT 在启动后立即触发，也 SHALL NOT 等待 24 小时）

#### Scenario: 重启不丢失调度
- **WHEN** 服务在 22:00 重启，日报时刻为 `21:00`
- **THEN** 重启后 DailyReport 仍将在次日 21:00 触发（SHALL NOT 因重启清零计数而延迟到「重启后 24 小时」）

#### Scenario: 默认时刻
- **WHEN** `AISettings` 中不存在 `daily_report_time` 键
- **THEN** DailyReport 使用默认时刻 `21:00`

#### Scenario: 非法配置值回退默认
- **WHEN** `AISettings["daily_report_time"]` 为非法格式（如 `"25:99"`、`"abc"`）
- **THEN** DailyReport 回退使用默认 `21:00`，并记录一条警告日志

### Requirement: 日报时刻可配置且可保存
系统 SHALL 通过 `AISettings` 存储 `daily_report_time`，复用现有 `GET/PUT /api/settings` 接口。保存时 SHALL 校验 `HH:MM` 格式（`00:00`–`23:59`），非法值 SHALL 被拒绝（返回错误，不写入）。

#### Scenario: 保存合法时刻
- **WHEN** 通过设置接口保存 `daily_report_time` = `"09:30"`
- **THEN** 值被写入 `ai_settings`，后续 DailyReport 调度使用 09:30

#### Scenario: 保存非法时刻被拒绝
- **WHEN** 通过设置接口保存 `daily_report_time` = `"25:00"`
- **THEN** 返回校验错误，原值不变

### Requirement: 配置变更在下次执行后生效
DailyReport 的墙钟计算 SHALL 在每次执行后重新读取配置时刻。配置变更 SHALL 在当前周期结束后生效（最迟一个执行周期）。

#### Scenario: 修改时刻后下次执行按新时刻
- **WHEN** 当前日报时刻为 `21:00`，在某次执行后将其改为 `09:00`
- **THEN** 下次 DailyReport 在次日 09:00 触发（重新读取配置）
