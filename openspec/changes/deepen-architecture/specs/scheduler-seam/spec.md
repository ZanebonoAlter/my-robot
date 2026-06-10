## ADDED Requirements

### Requirement: Scheduler 接口定义
系统 SHALL 在 `internal/jobs/` 中定义一个 `Scheduler` Go 接口，包含 `Start() error`、`Stop()`、`GetStatus() SchedulerStatusResponse`、`TriggerNow() map[string]interface{}`、`UpdateInterval(int) error`、`ResetStats() error` 六个方法。

#### Scenario: 所有调度器实现统一接口
- **WHEN** 新增一个调度器类型
- **THEN** 它 MUST 实现完整的 `Scheduler` 接口，编译器检查通过后方可编译

### Requirement: Scheduler 注册中心
系统 SHALL 提供一个 `Registry` 结构体，允许按名称注册和查找 `Scheduler` 实例。

#### Scenario: 通过名称查找调度器
- **WHEN** 调用 `registry.Get("auto_refresh")`
- **THEN** 返回 `*AutoRefreshScheduler` 和 `true`（若已注册），否则返回 `nil` 和 `false`

#### Scenario: 注册同名调度器
- **WHEN** 以相同名称注册两次
- **THEN** 第二次注册 MUST 被忽略或触发 panic

### Requirement: 运行时统一启停
系统 SHALL 支持遍历 `Registry` 中所有调度器统一执行 `Start()` 和 `Stop()`。

#### Scenario: 启动所有调度器
- **WHEN** 调用 `registry.StartAll()`
- **THEN** 按注册顺序依次调用每个调度器的 `Start()`，任一失败记录日志但不中断后续启动

#### Scenario: 关闭所有调度器（含超时）
- **WHEN** 调用 `registry.StopAll(30 * time.Second)`
- **THEN** 在超时时间内依次 stop 所有调度器，超时后强制终止

### Requirement: 删除 runtimeinfo 包
系统 SHALL 删除 `internal/app/runtimeinfo/` 包及其 8 个 `interface{}` 全局变量。

#### Scenario: 删除后编译通过
- **WHEN** 删除 `runtimeinfo/schedulers.go`
- **THEN** `go build ./...` MUST 成功，无任何包引用 `runtimeinfo`

### Requirement: Handler 通过 Registry 访问调度器
`jobs/handler.go` SHALL 通过 `Registry.Get(name)` 获取 `Scheduler` 接口值操作调度器，不再使用类型断言。

#### Scenario: 获取调度器状态
- **WHEN** 请求 `GET /api/schedulers/auto_refresh/status`
- **THEN** handler 从 Registry 获取 scheduler，调用 `scheduler.GetStatus()` 返回状态，无需类型断言

#### Scenario: 触发 DailyReport 带日期参数
- **WHEN** 请求 `POST /api/schedulers/daily_report/trigger?date=2026-06-01`
- **THEN** handler 获取 scheduler 后通过 `scheduler.(*DailyReportScheduler)` 断言调用 `TriggerNowWithDate`——此方法不属于 `Scheduler` 接口，仅该具体类型支持
