## ADDED Requirements

### Requirement: Database operations are auto-traced

系统 SHALL 通过自写 GORM trace 插件（`internal/platform/tracing/gorm_plugin.go` 的 `NewGORMPlugin()`，实现 GORM `Plugin` 接口 + callback）在 db 初始化处挂载，使所有 GORM 数据库操作自动产生 `SpanKind=Client` 的 span，并携带 `db.system`、`db.operation`、`db.statement` 等标准 DB semantic attributes。该 span SHALL 挂在当前 trace 的父 span 下（当存在活跃 trace 时），不要求业务代码做任何改动。

> 注：自写而非用 `gorm.io/plugin/opentelemetry`，因其最新版强制 gorm v1.30 升级（超 change 范围），见 design 决策 1。

当 `InstrumentGORM` 配置为 `false` 时，系统 SHALL 不挂载该插件（退化为现状，仅 SlowLogger）。

#### Scenario: 请求链路含 GORM 子 span

- **WHEN** 一个带活跃 trace 的 HTTP 请求执行若干 GORM 查询
- **THEN** 该 trace 的 span 树 SHALL 包含对应数量的 `SpanKind=Client` DB span，作为 HTTP / 业务 span 的子节点

#### Scenario: DB span 携带语句与操作类型

- **WHEN** 一条 `SELECT * FROM feeds WHERE id = ?` 执行
- **THEN** 对应 DB span 的 attributes SHALL 包含 `db.operation = SELECT` 与 `db.statement`（参数占位符化）

#### Scenario: 可按配置关闭

- **WHEN** `InstrumentGORM = false`
- **THEN** db 初始化 SHALL 不调用 `db.Use(tracing.NewGORMPlugin())`，GORM 操作不产生 span

### Requirement: Outbound HTTP calls are auto-traced

系统 SHALL 提供统一的出站 HTTP 客户端构造入口（`internal/platform/httpclient`），其产出的 `*http.Client` SHALL 通过 `otelhttp.NewTransport` 包装 transport，使所有经该 client 发出的出站请求自动产生 `SpanKind=Client` span，并透传 `traceparent` / `tracestate`（W3C Trace Context）到下游服务。

`airouter`（LLM / Embedding 调用）、`dataenrichment`、`admin` 等现有裸 `&http.Client{}` 调用点 SHALL 改为使用该统一入口。改动 SHALL 限定在客户端构造层，不进入 domain 业务逻辑。

当 `InstrumentHTTP = false` 时，工厂 SHALL 返回未包装 otelhttp 的普通 client（退化为现状行为）。

#### Scenario: LLM 调用出现在 trace

- **WHEN** `airouter` 经统一 client 发起对 LLM provider 的 HTTP 调用（在活跃 trace 内）
- **THEN** 该 trace 的 span 树 SHALL 包含一个 `SpanKind=Client` 的出站 HTTP span，作为调用方 span 的子节点

#### Scenario: traceparent 透传到下游

- **WHEN** 经统一 client 发起出站请求时存在活跃 trace
- **THEN** 请求 header SHALL 携带 `traceparent`，使下游服务（若接入 trace）能延续同一 trace

#### Scenario: 保留各调用点自定义

- **WHEN** 某调用点原本设置 `Timeout: 120s`
- **THEN** 改用统一 client 后 SHALL 仍能设置同等 Timeout（通过 functional options）

#### Scenario: 可按配置关闭

- **WHEN** `InstrumentHTTP = false`
- **THEN** 工厂产出的 client transport SHALL 不含 otelhttp 包装

### Requirement: Sampling strategy and trace configuration externalization

系统 SHALL 在 `TracerProvider` 初始化时配置 `ParentBased(TraceIDRatioBased(ratio))` 采样器：root span（无父 span，如 HTTP 入站、scheduler tick）按 `ratio` 决定是否采样；一旦 root span 被采样，其下所有子 span（DB / 出站 HTTP / 业务）SHALL 全部被采样，保证整条链路完整、不在中间断链。

`SampleRatio`、`InstrumentGORM`、`InstrumentHTTP` SHALL 可从环境变量（`TRACE_SAMPLE_RATIO` 等）读取。未配置任何环境变量时，系统 SHALL 默认 `SampleRatio = 1.0`（全采）、两个 instrumentation 开关为 `true`，行为等同变更前的全量入库。

#### Scenario: 全采下链路完整

- **WHEN** `SampleRatio = 1.0`，一个 HTTP 请求被处理（含 DB 与出站 HTTP 调用）
- **THEN** 该请求的整条 trace（HTTP root + 业务 span + DB span + 出站 HTTP span）SHALL 全部记录到 `otel_spans`

#### Scenario: 降采样仍保持链路完整

- **WHEN** `SampleRatio = 0.5`，某 HTTP 请求的 root span 被采样命中
- **THEN** 该 root 下的所有子 span SHALL 全部记录（不在子节点层做二次丢弃）

#### Scenario: 不配环境变量默认全采

- **WHEN** 未设置任何 trace 相关环境变量
- **THEN** 系统 SHALL 以 `SampleRatio = 1.0`、`InstrumentGORM = true`、`InstrumentHTTP = true` 运行，与变更前行为一致
