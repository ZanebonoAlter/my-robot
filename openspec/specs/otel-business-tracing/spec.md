## Purpose

在 OpenTelemetry trace 中注入业务上下文（capability、operation），使 LLM 调用可按业务流程区分；将 AICallLog 与 trace 链路关联；为关键 tag-processing 流程建立端到端 span 父子拓扑。
## Requirements
### Requirement: LLM span carries business attributes

Every `Router.Chat` and `Router.Embed` span SHALL include the following business attributes, sourced from the request's first-class fields (**NOT** from `ChatRequest.Metadata`):

- `ai.capability`: from `ChatRequest.Capability` (Chat) / the `capability` parameter (Embed)
- `ai.operation`: from `ChatRequest.Operation` (mandatory, non-empty — enforced by the `ai-logging` capability's forced-Operation requirement)
- `ai.session_id`: from `ChatRequest.SessionID` (set when non-empty; omitted only for non-orchestrated single calls)

`Router.Embed` SHALL be brought to parity with `Router.Chat` — previously (`router.go:191`) Embed spans only carried `ai.capability` and lacked `ai.operation`.

airouter SHALL NOT source these attributes from `ChatRequest.Metadata["operation"]` anymore (deprecates the old `router.go:111-113` weak-read path). Callers transition to passing `Operation`/`SessionID` as first-class fields; this is exercised by the `ai-logging` capability's "现有 AI 调用补齐 Operation 与 SessionID" requirement.

> **为什么 MODIFY 而非 ADD**：现行 spec（`openspec/specs/otel-business-tracing/spec.md`）已要求 Chat span 带 `ai.operation`，只是来源是 `Metadata["operation"]` 且 Operation 非强制。本 delta 把来源改为一等字段、补 session_id、补 Embed parity、并把"漏填静默通过"升级为"airouter 强制拒绝"。属于同一 requirement 的收紧，不新增并列 requirement。

#### Scenario: Chat span carries first-class operation and session_id

- **WHEN** `Router.Chat` is called with `Operation: "daily_report.cluster_tags"` and `SessionID: "daily_report_42_ab12cd34"`
- **THEN** the resulting span has attributes `ai.operation=daily_report.cluster_tags` and `ai.session_id=daily_report_42_ab12cd34`, sourced from the fields, not from Metadata

#### Scenario: Embed span parity with Chat

- **WHEN** `Router.Embed` is called with `Operation: "section.embedding"` and `SessionID` set
- **THEN** the resulting span has `ai.operation` and `ai.session_id` set (previously Embed only carried `ai.capability`)

#### Scenario: Empty operation no longer silently produces untagged span

- **WHEN** `Router.Chat` is called with an empty `Operation`
- **THEN** the call is rejected by airouter (per the `ai-logging` forced-Operation requirement) and no span/LogCall is created — supersedes the prior "LLM call without operation metadata still works" scenario

### Requirement: AICallLog records trace context
The `AICallLog` model SHALL include a `TraceID` field of type `string` (NULLABLE), and when writing a log entry, the system SHALL populate it from the current span context's trace ID. The `AICallLog` model SHALL also have a btree index on `created_at` for efficient retention-based cleanup.

#### Scenario: Successful LLM call records trace_id
- **WHEN** `Router.Chat` successfully calls an LLM provider within an active trace
- **THEN** the `AICallLog` row written has `trace_id` set to the current span's trace ID (32-char hex string)

#### Scenario: Failed LLM call still records trace_id
- **WHEN** `Router.Chat` fails on all provider attempts within an active trace
- **THEN** the `AICallLog` row for each failed attempt has `trace_id` set

#### Scenario: created_at index supports cleanup
- **WHEN** a DELETE query filters on `ai_call_logs.created_at`
- **THEN** the database uses the btree index on `created_at` (no sequential scan)

### Requirement: Context propagated through key tag-processing call chains
The following function call chains SHALL pass `context.Context` from entry point to LLM invocation:
- `runCleanupCycle` → `ExecuteFlatMerge` → `callFlatMergeLLM` → `Router.Chat`
- `processJob` → `TagArticle` / `tagArticle` → `findOrCreateTag` → `callLLMForTagJudgment` → `Router.Chat`

#### Scenario: Hierarchy cleanup trace has connected span tree
- **WHEN** the `TagHierarchyCleanupScheduler` triggers a cleanup cycle
- **THEN** the `workflow.hierarchy_cleanup.cycle` span is the parent of all `Router.Chat` spans created during that cycle

#### Scenario: Article tagging trace has connected span tree
- **WHEN** a tag job is processed by the TagQueue worker
- **THEN** the `workflow.article_tagging` span is the parent of all `Router.Chat` spans created during that job

### Requirement: Workflow entry points create parent spans with baggage
The following workflow entry points SHALL create a parent span and set OTel baggage:
- `runCleanupCycle`: span name `workflow.hierarchy_cleanup.cycle`, baggage `workflow.name=hierarchy_cleanup`, `workflow.domain=tag_management`
- TagJob worker (`processJob`): span name `workflow.article_tagging`, baggage `workflow.name=article_tagging`, `workflow.domain=tag_management`

The `Router.Chat` method SHALL propagate baggage values from the current context into span attributes with the `baggage.` prefix.

#### Scenario: Baggage propagated to LLM spans
- **WHEN** a hierarchy cleanup cycle runs with baggage `workflow.name=hierarchy_cleanup`
- **THEN** each `Router.Chat` span within that cycle has attribute `baggage.workflow.name=hierarchy_cleanup`

#### Scenario: Separate workflows produce distinct trace trees
- **WHEN** a hierarchy cleanup runs concurrently with an article tagging job
- **THEN** their respective `Router.Chat` spans are grouped under different parent spans (different trace IDs or parent span IDs)

### Requirement: Session-level aggregation endpoint

系统 SHALL 暴露 `GET /api/ai/sessions/:session_id`，按 session_id 一次性返回该编排的业务调用日志与链路时间线。

实现 SHALL 通过 trace_id 反查聚合（非 `otel_spans.attributes` 的 jsonb 查询）：

1. 先 `ai_call_logs WHERE session_id=?` 得调用记录与 trace_id 集合
2. 再 `otel_spans WHERE trace_id IN (...)` 得完整链路
3. 用 `BuildSpanTree` 组装 timeline 树

#### Scenario: 按编排 session 返回全貌

- **WHEN** `GET /api/ai/sessions/daily_report_42_ab12cd34`
- **THEN** 响应 SHALL 包含该 session 下全部 AICallLog（`call_logs`，按 created_at 正序）+ 通过 trace_id 关联的全部 otel_spans 组装的 `timeline`（span 树）

#### Scenario: 聚合 token 统计

- **WHEN** 响应返回
- **THEN** `summary.total_tokens` SHALL 为该 session 下所有 call_logs 的 token_usage 之和（prompt/completion/total 分别聚合）

#### Scenario: session 不存在

- **WHEN** `GET /api/ai/sessions/<不存在的id>`
- **THEN** SHALL 返回 `success=true` 但 `call_logs` 与 `timeline` 均为空数组（不报 404，便于前端空态处理）

### Requirement: Daily report orchestration carries business spans

日报管线的编排步骤 SHALL 创建 `workflow.daily_report.*` 业务 span，与 `Router.Chat`/`Router.Embed` span 形成父子拓扑，使一次日报生成的 trace 能重建编排步骤顺序。

span 名 SHALL 与 AICallLog.operation 对齐（`workflow.` 前缀 + operation 值；本地无 LLM 步骤用语义名）：

- LLM/embedding 节点：`workflow.daily_report.cluster_tags` / `.highlights` / `.cluster_threads` / `.merge_arbitration` / `.section_embedding`
- 本地节点（无 LLM，记耗时/拓扑）：`workflow.daily_report.dedup` / `.thread_fit`
- 编排入口 root span：`workflow.daily_report.generate`

**Step5 并发约束**：`GenerateHighlights` + K 个 `GenerateClusterThreads` 在 goroutine 并发，parent span context SHALL 在 goroutine 启动前取好再传入，确保并发 span 共享正确 parent。

#### Scenario: 日报 trace 含编排步骤 span

- **WHEN** 一次日报生成完成
- **THEN** 其 trace 的 timeline SHALL 包含 `workflow.daily_report.*` span 作为 `Router.Chat`/`Router.Embed` span 的父节点（或兄弟编排节点），非仅有最外层 HTTP span + LLM span

