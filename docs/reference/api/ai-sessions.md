# AI Session 聚合端点

> `GET /api/ai/sessions/:session_id` — 按编排 session 一次性返回「业务调用日志 + 链路时间线」，供观测台一次请求拿全貌。
>
> 由 `otel-tracing-completion` change 引入。依赖前置 `ai-call-logging-schema`（session_id / trace_id 桥梁）。

## 路径

```
GET /api/ai/sessions/:session_id
```

归属 `/api/ai` group（与 `/api/ai/call-logs` 同 group）。handler：`internal/admin/handler/session_handler.go`。

## 作用

一次日报编排（cluster_tags / highlights / cluster_threads / merge / embedding 多次 LLM 调用）在观测层是「一个 session、一条时间线」。本端点把分散两处的数据聚合：

- **业务调用日志**：`ai_call_logs`（按 `session_id`，主键）
- **链路时间线**：`otel_spans`（通过 call_logs 的 `trace_id` 反查关联）

避免前端自己拼两次请求、自己对齐 session_id / trace_id。

## 聚合策略：trace_id 反查（非 attribute jsonb 查）

**不**直接按 `otel_spans.attributes` 里的 `ai.session_id` 做 jsonb 查询（数组 JSON，性能与可读性差）。改用 trace_id 反查：

1. `SELECT * FROM ai_call_logs WHERE session_id = ? ORDER BY created_at` → call_logs + trace_id 去重集合
2. `SELECT * FROM otel_spans WHERE trace_id IN (?) ORDER BY start_time_unix_nano` → 完整链路（`QuerySpansByTraceIDs`）
3. `BuildSpanTree` 组装 timeline 树

## 响应

```json
{
  "success": true,
  "data": {
    "session_id": "daily_report_42_ab12cd34",
    "summary": {
      "call_count": 6,
      "span_count": 14,
      "started_at": "2026-07-04T...",
      "ended_at": "2026-07-04T...",
      "total_tokens": { "prompt": 12340, "completion": 890, "total": 13230 },
      "error_count": 0
    },
    "call_logs": [ { "operation": "...", "prompt": "...", "token_usage": {...}, "trace_id": "...", ... } ],
    "timeline": [ { "name": "workflow.daily_report.generate", "children": [...] } ]
  }
}
```

- `summary.total_tokens`：该 session 下所有 call_logs 的 `token_usage` 之和（prompt / completion / total 分别聚合；malformed 行跳过，不让单行失败拖垮整次聚合）
- `summary.span_count`：含编排 span + LLM / embed span
- `summary.started_at` / `ended_at`：取 call_logs 与 otel_spans 的最早 / 最晚时间
- `timeline`：`BuildSpanTree` 组装的 span 树（根节点 `workflow.daily_report.generate`，children = 各编排步骤；`BuildSpanTree` 递归构建，保留多层嵌套）

## 空态

session 不存在时返回 `success=true` + 空 `call_logs`（`[]`）+ 空 `timeline`（`[]`）+ 零值 summary，**不报 404**，便于前端空态统一处理。

## 关联

- 编排业务 span（`workflow.daily_report.*`）由同 change 的 task 2 在日报管线埋点，通过 trace_id 自动串进 timeline——埋点越多 timeline 越完整，本端点自动受益。
- 业务日志字段详见 [`ai-call-logs.md`](ai-call-logs.md)；链路 span 结构详见 [`traces.md`](traces.md) + [`architecture/tracing.md`](../architecture/tracing.md)。
