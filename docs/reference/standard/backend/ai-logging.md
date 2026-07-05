# AI 调用记录规范（AI Call Logging）

> **权威源**：本文件是后端所有 AI/LLM 调用"必须记录什么、怎么记录"的唯一权威。
> 实现层在 `internal/platform/airouter`（`Router.Chat` / `Router.Embed` → `AICallLog`）。

## 为什么必须有这条规范

每个新增 AI 功能（日报、摘要、标签聚类、未来的 agent 编排）都会产生 LLM 调用。
**若调用不被完整、统一地记录，调试时既看不见 prompt 也看不见响应**——出问题（幻觉、死循环、查错数据、token 超支）只能靠猜。
本规范把"AI 调用必须记什么"从"开发者自觉"提升为**硬约束**，让每一次 AI 调用都可观测、可回放、可审计。

## 红线（硬约束）

### R1. 所有 AI 调用必须经 airouter

- 业务代码**禁止**绕过 `airouter.Router.Chat` / `Router.Embed` 直接打 `/chat/completions` 或 `/embeddings`。
- airouter 是唯一入口：负责按 capability 路由、provider failover、限流、**写 AICallLog**。
- 绕过 airouter = 绕过日志 = 不可观测，**等同于不合规**。

> 现状确认：日报管线（`ClusterTags`/`GenerateHighlights`/`GenerateClusterThreads`/`llmArbitrateMerges`/section embedding）**已全部走 airouter**，无绕过。新增功能沿用此路径。

### R2. 必须记录的字段

每次 AI 调用的 `AICallLog` 至少包含：

| 字段 | 必填 | 说明 |
|------|------|------|
| `operation` | ✅ | 业务操作名（如 `daily_report.cluster_tags`、`daily_report.highlights`、`agent.tool_use`）。**必须语义化、可分组查询**，不能只填 capability |
| `capability` | ✅ | airouter capability 常量（`digest_polish`/`embedding`/未来的 `agent_tool_use` 等） |
| `provider_name` / `model` | ✅ | 实际调用的供应商 + 模型名 |
| `messages`（prompt） | ✅ | **完整 system+user prompt**，不再是只存 Metadata 摘要 |
| `response` | ✅ | 完整响应文本（超长可按 §截断策略处理） |
| `token_usage` | ✅ | prompt_tokens / completion_tokens / total |
| `latency_ms` | ✅ | 单次调用耗时 |
| `success` / `error` | ✅ | 成功标志 + 失败时的错误码与信息 |
| `session_id` | ⚠️见 R4 | 一次编排内多次调用的分组键 |

### R3. 编排类调用必须带 session_id（串联同一次编排）

agent loop / 多步管线（如日报 7 步管线、未来的数据增强编排）会一次产生 N 次 LLM 调用 + M 次工具调用。
**同一次编排内的所有调用，必须共享同一个 `session_id`**，否则事后无法 GROUP BY 重建"一次编排的全过程"。

- session_id 由编排入口生成（如 `daily_report_{report_id}_{uuid}`），通过 context 传给每一次 airouter 调用。
- 现状缺口：当前靠 OTel `trace_id`，但 goroutine fork 会断 span，且 trace_id 是链路追踪概念、不是业务编排分组键。**新增编排功能必须显式传 session_id**。

### R4. 工具调用也要记录（agent 编排专用）

agent loop 里的**工具调用**（非 LLM 调用）也必须留痕，至少记：工具名、参数、返回结果摘要、耗时、成功失败。
这可以单独存表，也可以复用 AICallLog 加 `call_type='tool'` 区分——实现层自定，但**不能不记**。

## 截断与脱敏策略

- **prompt / response 完整记录**，但超长（默认 > 20000 runes）时截断并标注 `[truncated]`，保留头尾。不能因为"可能很长"就一律只存摘要。
- **敏感信息脱敏**：API key、用户私密字段在入库前脱敏。Prompt 正文属于调试必需信息，**不脱敏**（这是本地单用户系统，不存在跨用户泄露）。
- 保留期：当前由 `job_log_cleanup.go` 7 天后清理。如需更长留存，在配置里调，不在此规范约束。

## Anti-Patterns（硬禁）

- ❌ 绕过 airouter 直接 HTTP 调 LLM
- ❌ `AICallLog` 只存 capability + 响应摘要，不存 prompt（**这是当前最大缺口**）
- ❌ `operation` 字段留空或填无意义值（如 `"chat"`）
- ❌ agent loop 的多次调用不传 session_id，事后无法重建编排过程
- ❌ 新增 AI 功能不更新本规范的"已接入功能清单"

## 已接入 AI 调用的功能清单（含缺口）

> 本节是"补齐跟踪表"。新增 AI 功能必须在此登记。✅=符合本规范，⚠️=待补齐。

| 功能 | operation 名 | 经 airouter | 记 prompt | 记 token | 带 session_id | 状态 |
|------|-------------|:---:|:---:|:---:|:---:|:---:|
| 日报-标签聚类 | `daily_report.cluster_tags` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-高亮生成 | `daily_report.highlights` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-线索生成 | `daily_report.threads` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-合并仲裁 | `daily_report.merge_arbitration` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| section 向量化 | `section.embedding` | ✅ | N/A | ✅ | ✅ | 已补齐 |
| 话题关注评估 | `topic_watch.evaluate` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 数据增强编排（规划中） | `data_enrichment.*` | 待接入 | 待接入 | 待接入 | 必须带 | 未实现 |

**补齐状态**：以上日报管线 6 处调用已由 [`ai-call-logging-schema`](../../../../openspec/changes/ai-call-logging-schema) change 全部补齐（prompt/token/session_id 落地，operation 升级为一等字段且 airouter 强制校验非空），从"待补齐"转为 ✅。其他调用方（reader/tagmanagement）也已补 operation 字段。

## 查询入口（规范要求，实现可排期）

记录是为了能看。规范要求至少提供：
- `GET /api/ai/call-logs`（支持按 operation / session_id / capability / 时间范围过滤）
- 前端查看视图（按 session 分组回放一次编排）

> 现状：`GET /api/ai/call-logs` 已由 [`ai-call-logging-schema`](../../../../openspec/changes/ai-call-logging-schema) change 实现（支持 operation/session_id/capability/时间范围过滤）。前端查看视图（按 session 分组回放）待后续 change。

## 资料来源

基于 `internal/platform/airouter/router.go`（`Chat`:102 / `Embed`:188）、`internal/models/ai_models.go`（`AICallLog`:122）现状调研，
以及 2026-07-04 关于"日报 AI 调用 prompts 看不见 / agent 编排记录缺口"的评审讨论。
