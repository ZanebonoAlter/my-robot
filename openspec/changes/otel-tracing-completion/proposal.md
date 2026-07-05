## Why

`ai-call-logging-schema`（前置 change）让 `AICallLog` 有了 operation/session_id/prompt/token，并打通了 span attribute 桥梁（session_id 进 `otel_spans`）。但「调用链路可观测」还差三块，本 change 补齐：

1. **日报编排中间步骤无业务 span**：trace 里只有最外层 HTTP span + 内层 `Router.Chat` span，中间 7 步编排逻辑（ClusterTags/Highlights/Threads/Merge/embedding）是黑洞——看得到每次 LLM 调用，但看不到"哪步编排慢、哪步错、步骤间如何衔接"。[`architecture/tracing.md`](../../../docs/reference/architecture/tracing.md)「当前问题与下一步建议」第 1 条已点名此缺口。
2. **goroutine fork 断链**：`topic_watch` 在日报事务后异步跑，脱离 trace ctx。`GoWithTrace`/`TraceAsyncOp`（`internal/platform/tracing/helpers.go`）工具已就绪但**全仓零业务调用**（tracing.md §5 自陈）。
3. **查询割裂**：`/api/ai/call-logs`（业务日志）和 `/api/traces`（链路 span）两套 API，前端要看"一次日报编排的全貌"得自己拼两次请求、自己对齐 session_id/trace_id。缺一个按 session 一次性返回「业务日志 + 链路时间线」的聚合端点。

## What Changes

### A. 聚合统一端点（otel-business-tracing）

新增 `GET /api/ai/sessions/:session_id`：按 session_id 一次性返回该编排的「业务调用日志（AICallLog）+ 链路时间线（otel_spans）」。以 `ai_call_logs.session_id` 为主键反查 trace_id 集合，再 `otel_spans WHERE trace_id IN (...)` 拿完整链路（含编排 span，若 B 部分已埋点）。前端一个请求拿全貌。

### B. 日报编排埋点（otel-business-tracing）

在日报管线关键编排节点补业务 span（`workflow.daily_report.*`），形成父子拓扑，让 trace 能重建 7 步编排。**具体埋点函数待 apply 阶段对着 `internal/topicgraph/service/daily_report_*.go` 实际代码决策**（函数签名、ctx 传递路径需逐个确认），本 change 在 proposal/design 层定策略与命名规范，task 层留框架。

### C. topic_watch 异步接 GoWithTrace（otel-business-tracing）

把 `daily_report_watch.go` 的 goroutine fork 改用 `tracing.GoWithTrace`，让异步执行的 watch 评估通过 `parent_trace_id` attribute 关联日报 trace，不再完全断链。

## Capabilities

### Modified Capabilities

- `otel-business-tracing`：新增 session 聚合端点、日报编排业务 span、goroutine 异步 trace 接入

## Impact

- **后端**：
  - `internal/admin/`（HTTP handler）+ 复用 `internal/platform/tracing` 的 `BuildSpanTree`/`QueryByTraceID`：新增 `GET /api/ai/sessions/:session_id`。
  - `internal/topicgraph/service/daily_report_*.go`：编排节点加 span（函数签名/ctx 待 apply 决策）。
  - `internal/topicgraph/service/daily_report_watch.go`：goroutine 改用 `GoWithTrace`。
- **前端**：无（本 change 仅留 API，前端查看页后续 change 接）。
- **数据库**：无 schema 变更（聚合是查询层；session_id 列与 span attribute 已由前置 change 落地）。
- **依赖**：**`ai-call-logging-schema` 须先归档**——聚合端点依赖 session_id 进 `otel_spans`（span 桥梁），编排埋点也依赖 Operation/SessionID 一等字段已就位。
- **AI 成本**：零。
