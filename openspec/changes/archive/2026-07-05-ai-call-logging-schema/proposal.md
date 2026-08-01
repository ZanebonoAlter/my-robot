## Why

`AICallLog` 是 airouter 写的 AI 调用日志表，但当前是「**有实现、无规范、看不见**」状态——三个缺口：

1. **prompt 完全不记**：`router.go:151` 的 `LogCall` 只把 `req.Metadata`（统计字段 map）入库，真正的 `messages`（system+user prompt）从不落库。这是用户「日报 prompts 看不见」的直接根因。调试幻觉/死循环/查错数据时，无从回放"当时喂给 LLM 的是什么"。
2. **只写不读**：全仓 `ai_call_logs` 的唯一消费者是 7 天清理 job（`job_log_cleanup.go:16`），**没有任何查询 API、没有前端视图**。即使记了也看不到。
3. **编排无法重建**：现有 schema 靠 OTel `trace_id` 串联，但 goroutine fork 会断 span；且 trace_id 是链路追踪概念，不是「业务编排会话」。日报 7 步管线的多次调用、未来 agent loop 的 N 次调用，散落在多行里无法 GROUP BY 重建一次编排。

规范文档 [`standard/backend/ai-logging.md`](../../../docs/reference/standard/backend/ai-logging.md) 已立（R1-R4 红线 + 补齐跟踪表），本 change 把规范**落地为 schema + 代码 + 查询入口**，让 AI 调用真正可观测、可回放、可审计。

## What Changes

### A. schema 补字段（ai-logging）

`AICallLog` 新增 4 列，把「规范要求必记」的字段补齐：

- `operation`（业务操作名，语义化、可分组，如 `daily_report.cluster_tags`）—— 当前只塞在 Metadata blob 里，无独立列
- `prompt`（完整 messages 文本，超长按 20000 runes 截断并标注）—— **当前完全不记**
- `token_usage`（prompt/completion/total 的 JSON）—— 当前不记
- `session_id`（编排分组键，一次 agent loop / 一次日报生成的多次调用共享）—— 当前只有 trace_id

### B. airouter 改造（ai-logging）

- `ChatRequest` 新增 `Operation string`、`SessionID string` 字段；调用方必填 Operation（airouter 校验，空则 panic-on-init 或 return error）。
- `Router.Chat` / `Router.Embed` 把 `req.Messages`、token 用量、operation、session_id 写入 `AICallLog`。
- 日报管线（`ClusterTags`/`GenerateHighlights`/`GenerateClusterThreads`/`llmArbitrateMerges`/section embedding）+ `topic_watch.evaluate` 全部补齐 `Operation` + `SessionID` 传参——这是规范「补齐跟踪表」的执行。
- **span 桥梁（连接 OTel）**：`Router.Chat`/`Router.Embed` 的 span attribute 改为从 `req.Operation`/`req.SessionID` 一等字段注入（现状从 `Metadata["operation"]` 弱读取，见 `router.go:111-113`），并新增 `ai.session_id` attribute、给 `Router.Embed` 补 `ai.operation`。这让 `otel_spans` 与 `ai_call_logs` 能用**同一个 session_id/operation**互查，是后续「按 session 聚合统一端点」change 的前置依赖。

### C. 查询入口（ai-logging）

新增 `GET /api/ai/call-logs`，支持按 `operation` / `session_id` / `capability` / 时间范围过滤，让记录「看得见」。前端本 change **不做查看页面**（前端最小），仅留 API 供后续 change 接。

## Capabilities

### Modified Capabilities

- `ai-logging`：`AICallLog` schema 补字段、airouter 记全 prompt/token/operation/session_id、现有日报+topic_watch 调用补齐传参、新增查询 API
- `otel-business-tracing`：`Router.Chat`/`Router.Embed` span 的业务 attribute 来源从 `Metadata` 弱读取升级为 `req.Operation`/`req.SessionID` 一等字段强注入，新增 `ai.session_id` attribute、补齐 `Router.Embed` 的 `ai.operation`

## Impact

- **后端**
  - 新增列：`ai_call_logs.operation`、`prompt`、`token_usage`、`session_id`（显式迁移，§10）。
  - `internal/platform/airouter`：`ChatRequest` 加字段、`LogCall` 落 prompt/token、新增 `Operation` 必填校验。
  - `internal/topicgraph/service/`：日报管线 5 处 + `daily_report_watch.go` 补齐 Operation/SessionID。
  - `internal/admin/`：新增 `GET /api/ai/call-logs` handler + 路由。
- **前端**：无（本 change 仅留 API）。
- **数据兼容**：4 列新增，历史行新列为 NULL，不影响清理 job 与现有读取。
- **OTel span attribute**：来源从 `Metadata["operation"]` 改为 `req.Operation`。现状业务侧本就在 Metadata 传 operation（日报 7 处、tagmanagement 多处），升级后 span 行为向后兼容（值不变），只是更可靠 + 多了 session_id。
- **AI 成本**：零额外 AI 调用——本 change 只改日志记录，不改任何 LLM 调用逻辑。
- **依赖关系**：`data-enrichment-orchestration` change 依赖本 change 的 session_id（编排可观测基础），**须先归档本 change**。
