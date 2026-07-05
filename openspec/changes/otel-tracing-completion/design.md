# Design — otel-tracing-completion

> 补齐 `ai-call-logging-schema`（前置）之后的调用链路缺口：聚合端点 + 日报编排埋点 + goroutine 异步接入。

## 设计目标

让一次编排（如日报生成）在观测层是「一个 session、一条时间线、一个请求拿全貌」。session_id 是业务编排分组键（AICallLog 主），trace_id 是链路追踪键（otel_spans 主），两者通过 `LogCall`（`store.go:178`）写入的 trace_id 串联。

## 1. 聚合统一端点

### 1.1 路径与归属

`GET /api/ai/sessions/:session_id`，归 `/api/ai` group（与前置 change 的 `/api/ai/call-logs` 同 group）。理由：session_id 的「家」是 `ai_call_logs`，otel_spans 只是带了同名 attribute；聚合以业务日志为主、链路为辅。HTTP handler 放 `internal/admin/`，链路组装复用 `internal/platform/tracing` 的 `BuildSpanTree`（`query.go:209`）。

### 1.2 查询策略（trace_id 反查，非 attribute 查）

**核心决策：不直接按 `otel_spans.attributes` 里的 `ai.session_id` 做 jsonb 查询，而是用 trace_id 反查。**

理由：
- `otel_spans.attributes` 存的是 `[{"key":"ai.session_id","value":{"type":"STRING","value":"xxx"}}]` 数组 JSON（见 `model.go` MarshalAttributes），按属性值过滤需 jsonb path 查询，性能与可读性差。
- `LogCall` 已把 trace_id 写进 `ai_call_logs.trace_id`，天然串联。

流程：
1. `SELECT * FROM ai_call_logs WHERE session_id = ? ORDER BY created_at` → 得 call_logs + trace_id 集合（去重）。
2. `SELECT * FROM otel_spans WHERE trace_id IN (?) ORDER BY start_time_unix_nano` → 得完整链路（含编排 span，若 B 部分已埋点；若未埋点，只有 Router.Chat/Embed span + 外层 HTTP span）。
3. 用现有 `BuildSpanTree` 组装 timeline 树。

**协同效应**：编排埋点（B 部分）加的 `workflow.daily_report.*` span 会自动通过 trace_id 串进 timeline，无需额外查询逻辑。埋点越多 timeline 越完整，聚合端点自动受益——这把 A、B 两部分设计成互相增益。

### 1.3 响应 schema

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
      "total_tokens": {"prompt": 12340, "completion": 890, "total": 13230},
      "error_count": 0
    },
    "call_logs": [ { "operation":"...","prompt":"...","token_usage":{...},"trace_id":"...", ... } ],
    "timeline": [ { "name":"workflow.daily_report.cluster_tags","children":[...], ... } ]
  }
}
```

`summary.total_tokens` 从 call_logs 的 token_usage 聚合；`span_count` 含编排 span + LLM span；`started_at`/`ended_at` 取 call_logs 与 otel_spans 的最早/最晚时间。

### 1.4 空态处理

session 不存在时返回 `success=true` + 空 call_logs + 空 timeline（不 404），便于前端空态统一处理。

## 2. 日报编排埋点

### 2.1 命名规范

编排节点 span 名用 `workflow.daily_report.<step>`：

| AICallLog.operation | 编排 span 名 |
|---------------------|-------------|
| `daily_report.cluster_tags` | `workflow.daily_report.cluster_tags` |
| `daily_report.highlights` | `workflow.daily_report.highlights` |
| `daily_report.threads` | `workflow.daily_report.threads` |
| `daily_report.merge_arbitration` | `workflow.daily_report.merge_arbitration` |
| `section.embedding` | `workflow.daily_report.section_embedding` |
| `topic_watch.evaluate` | `workflow.daily_report.watch_evaluate` |

与 operation 值刻意只差 `workflow.` 前缀，便于人眼对齐两套视图（业务日志 vs 链路）。

### 2.2 ctx 传递与 trace 继承

日报编排入口 `GenerateAndSaveReport` 已（由前置 change）生成 SessionID 并经 context 传递。本 change 在每个步骤函数入口 `tracer.Start(ctx, "workflow.daily_report.<step>")` 开 span，自动继承同一 trace_id（前提是入口 ctx 在同一 trace 内）。

### 2.3 待 apply 决策（task 留框架）

具体埋点要对着 `daily_report_cluster.go`/`daily_report_llm.go`/`daily_report_orchestrator.go`/`daily_report_watch.go` 实际函数签名与 ctx 传递路径逐个确认：
- 哪些函数已 exported 且第一参 `ctx`（可直接 `tracer.Start`）
- 哪些函数缺 ctx 参数需补签名（参照前置 otel change `2026-04-30-otel-business-tracing-improvements` 的 ctx 补齐模式）
- 是否用 `go-instrument` 自动注入 vs 手动 span（参照 `architecture/tracing.md`「go-instrument 第一版的真实约束」）

**apply 阶段先 `codegraph` + `read` 摸清各步骤函数签名，再定 task 细节**。本 change 不在 proposal 层锁死函数清单，避免与实际代码脱节（这正是"task 留待代码决策"的来由）。

## 3. topic_watch 异步接入 GoWithTrace

`daily_report_watch.go` 在日报事务后异步跑（goroutine），现状脱离 trace ctx。改用 `tracing.GoWithTrace`（`helpers.go`）：
- 异步任务开 `WithNewRoot` root span，用 `parent_trace_id` attribute 记录来源日报 trace（`architecture/tracing.md` §5 描述的现有 helper 行为）。
- **不**把异步 span 挂成日报 trace 的子节点（跨 goroutine 不安全），用 parent_trace_id 软关联。

聚合端点在组装 timeline 时，把 `parent_trace_id` 匹配本 session trace_id 的 watch span 作为关联段呈现（实现细节 apply 定）。

## 4. 不做什么

- 不改 `/api/traces` 现有端点（recent/search/stats/timeline/otlp 保持）。
- 不引入 OTLP collector / Jaeger（继续用自建 DB exporter）。
- 不做前端查看页（聚合端点供后续 change 接前端）。
- 不改 otel_spans schema（attributes 已能承载 session_id attribute）。
- 不动采样率（单用户系统全量，`tracing/config.go` 不变）。

## 5. 风险与取舍

- **编排埋点的函数签名改动风险**：补 ctx 参数可能影响调用方。参照前置 otel change 用 `grep` + `go build ./...` 兜底。缓解：apply 阶段逐函数确认，task 细化到函数级。
- **聚合端点 trace_id 反查性能**：日报一次编排 trace_id 通常 1-3 个，otel_spans 按 trace_id 索引（`idx_otel_spans_trace_id` 已建），IN 查询快。无性能风险。
- **编排埋点增量完成时 timeline 不完整**：B 部分若只埋部分步骤，timeline 只有部分编排 span。可接受——先有端点，埋点增量补全，timeline 自动变完整（§1.2 的协同效应）。
- **parent_trace_id 软关联 vs 硬挂子 span**：取软关联，牺牲"watch span 直接挂在日报 trace 树里"的视觉完整性，换跨 goroutine 安全。
