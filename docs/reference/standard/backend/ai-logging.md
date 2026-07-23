# AI 调用记录规范（AI Call Logging）

<!--
doc-impact-applies: backend-go/internal/platform/airouter/, backend-go/internal/dataenrichment/
-->

> **权威源**：本文件是后端所有 AI/LLM 调用"必须记录什么、怎么记录"的唯一权威。
> 实现层在 `backend-go/internal/platform/airouter`（`Router.Chat` / `Router.Embed` → `AICallLog`）。

## 为什么必须有这条规范

每个新增 AI 功能（日报、摘要、标签聚类、未来的 agent 编排）都会产生 LLM 调用。
**若调用不被完整、统一地记录，调试时既看不见 prompt 也看不见响应**——出问题（幻觉、死循环、查错数据、token 超支）只能靠猜。
本规范把"AI 调用必须记什么"从"开发者自觉"提升为**硬约束**，让每一次 AI 调用都可观测、可回放、可审计。

## Requirements

### Requirement: 所有 AI 调用必须经 airouter

**级别**: MUST

业务代码 SHALL 通过 `airouter.Router.Chat` / `Router.Embed` 调用 LLM。airouter 是唯一入口：负责按 capability 路由、provider failover、限流、写 `AICallLog`。绕过 airouter = 绕过日志 = 不可观测，等同于不合规。

> 现状：日报管线（`ClusterTags`/`GenerateHighlights`/`GenerateClusterThreads`/`llmArbitrateMerges`/section embedding）已全部走 airouter，无绕过。新增功能沿用此路径。

#### Scenario: 新增 LLM 调用

- **WHEN** 业务代码需要调用 LLM（chat / embedding）
- **THEN** 走 `airouter.Router.Chat(req)` / `Router.Embed(req)`
- **AND NOT** 绕过 airouter 直接 `http.Post("/chat/completions")` 或 `/embeddings`

### Requirement: 必须记录完整调用字段

**级别**: MUST

每次 AI 调用的 `AICallLog` SHALL 至少包含下表字段；`messages`（prompt）与 `response` SHALL **完整记录**（超长按「截断与脱敏策略」处理），不得只存摘要。

| 字段 | 必填 | 说明 |
| ------ | ------ | ------ |
| `operation` | ✅ | 业务操作名（如 `daily_report.cluster_tags`、`daily_report.highlights`、`agent.tool_use`）。**必须语义化、可分组查询**，不能只填 capability |
| `capability` | ✅ | airouter capability 常量（`digest_polish`/`embedding`/未来的 `agent_tool_use` 等） |
| `provider_name` / `model` | ✅ | 实际调用的供应商 + 模型名 |
| `messages`（prompt） | ✅ | **完整 system+user prompt**，不再是只存 Metadata 摘要 |
| `response` | ✅ | 完整响应文本（超长可按「截断与脱敏策略」处理） |
| `token_usage` | ✅ | prompt_tokens / completion_tokens / total |
| `latency_ms` | ✅ | 单次调用耗时 |
| `success` / `error` | ✅ | 成功标志 + 失败时的错误码与信息 |
| `session_id` | ⚠️见下一 Requirement | 一次编排内多次调用的分组键 |

#### Scenario: 写 AICallLog

- **WHEN** 一次 AI 调用经 airouter 执行
- **THEN** `AICallLog` 写入上表全部 ✅ 字段（`messages` / `response` 完整）
- **AND NOT** 只存 capability + 响应摘要、不存 prompt（这是历史最大缺口）
- **AND NOT** `operation` 留空或填无意义值（如 `"chat"`）

### Requirement: 编排类调用必须带 session_id（串联同一次编排）

**级别**: MUST

agent loop / 多步管线（如日报 7 步管线、数据增强编排）一次产生 N 次 LLM 调用 + M 次工具调用。**同一次编排内的所有调用 SHALL 共享同一个 `session_id`**，否则事后无法 GROUP BY 重建"一次编排的全过程"。session_id 由编排入口生成（如 `daily_report_{report_id}_{uuid}`），通过 context 传给每一次 airouter 调用。

> 现状缺口：当前靠 OTel `trace_id`，但 goroutine fork 会断 span，且 trace_id 是链路追踪概念、不是业务编排分组键。**新增编排功能必须显式传 session_id**。

#### Scenario: 新增编排功能

- **WHEN** 新增多步编排功能（agent loop / 管线，一次产生多次 AI 调用）
- **THEN** 编排入口生成 `session_id`，通过 context 传给该次编排内每一次 airouter 调用
- **AND NOT** agent loop 的多次调用各传各的、事后无法重建编排过程

### Requirement: 工具调用也要记录（agent 编排专用）

**级别**: MUST

agent loop 里的**工具调用**（非 LLM 调用）SHALL 留痕，至少记：工具名、参数、返回结果摘要、耗时、成功失败。可单独存表，也可复用 `AICallLog` 加 `call_type='tool'` 区分——实现层自定，但**不能不记**。

#### Scenario: agent 工具调用留痕

- **WHEN** agent loop 执行工具调用（非 LLM 调用）
- **THEN** 记录工具名 / 参数 / 结果摘要 / 耗时 / 成功失败（单独表 或 `AICallLog` + `call_type='tool'`）

### Requirement: 规范清单维护

**级别**: SHOULD

新增 AI/LLM 调用功能 SHALL 在本规范「已接入 AI 调用的功能清单」登记 operation 名 + 补齐状态，保持补齐跟踪表最新。

#### Scenario: 新增 AI 功能登记

- **WHEN** 新增 AI/LLM 调用功能并接入 airouter
- **THEN** 在「已接入 AI 调用的功能清单」表新增一行（operation 名 + 经 airouter / 记 prompt / 记 token / 带 session_id / 状态）

## 截断与脱敏策略

- **prompt / response 完整记录**，但超长（默认 > 20000 runes）时截断并标注 `[truncated]`，保留头尾。不能因为"可能很长"就一律只存摘要。
- **敏感信息脱敏**：API key、用户私密字段在入库前脱敏。Prompt 正文属于调试必需信息，**不脱敏**（这是本地单用户系统，不存在跨用户泄露）。
- 保留期：当前由 `job_log_cleanup.go` 7 天后清理。如需更长留存，在配置里调，不在此规范约束。

## 已接入 AI 调用的功能清单（含缺口）

> 本节是"补齐跟踪表"。新增 AI 功能必须在此登记。✅=符合本规范，⚠️=待补齐。

| 功能 | operation 名 | 经 airouter | 记 prompt | 记 token | 带 session_id | 状态 |
| ------ | ------------- | :---: | :---: | :---: | :---: | :---: |
| 日报-标签聚类 | `daily_report.cluster_tags` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-高亮生成 | `daily_report.highlights` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-线索生成 | `daily_report.threads` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 日报-合并仲裁 | `daily_report.merge_arbitration` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| section 向量化 | `section.embedding` | ✅ | N/A | ✅ | ✅ | 已补齐 |
| 话题关注评估 | `topic_watch.evaluate` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 循环A-新闻汇总 | `data_enrichment.summarize_context` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 循环B-解读员 | `data_enrichment.interpret` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 循环B-查询员每轮 | `data_enrichment.tool_use` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 循环B-分析员 | `data_enrichment.analyze` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 循环B-review对比 | `data_enrichment.review_judge` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 个股辩论-结果提炼 | `data_enrichment.debate_distill` | ✅ | ✅ | ✅ | ✅ | 已补齐 |
| 报告追问-查询员每轮 | `data_enrichment.qa_tool_use` | ✅ | ✅ | ✅ | ✅ | 已补齐 |

**补齐状态**：以上日报管线 6 处调用已由 [`ai-call-logging-schema`](../../../../openspec/changes/ai-call-logging-schema) change 全部补齐（prompt/token/session_id 落地，operation 升级为一等字段且 airouter 强制校验非空），从"待补齐"转为 ✅。其他调用方（reader/tagmanagement）也已补 operation 字段。

**数据增强 SessionID 规则**：

- 循环B（`data_enrichment.interpret` / `tool_use` / `analyze` / `review_judge`）同一次增强内所有 LLM 调用共享 `data_enrichment_{topic_id}_{uuid8}`
- 循环A（`data_enrichment.summarize_context`）一次汇总共享 `lifeline_context_{topic_id}_{granularity}_{uuid8}`
- 个股辩论提炼（`data_enrichment.debate_distill`）共享 `data_enrichment_debate_{topic_id}_{result_id}`
- 报告追问（`data_enrichment.qa_tool_use`）每次询问唯一，共享 `data_enrichment_qa_{result_id}_{uuid8}`（基于 result 而非 topic，同一报告多轮追问各自独立 session）

**数据增强 Capability 映射**：

| Capability | 归属的 Operation |
| --- | --- |
| `data_enrichment_news` | `data_enrichment.summarize_context`（循环A 纯新闻汇总） |
| `data_enrichment_analysis` | `data_enrichment.interpret`（解读员，含形态判断+视角候选）/ `data_enrichment.tool_use`（查询员每轮）/ `data_enrichment.analyze`（分析员，分层见解）/ `data_enrichment.review_judge`（兑现度复盘）/ `data_enrichment.debate_distill`（FinGenius 提炼）/ `data_enrichment.qa_tool_use`（报告追问每轮，复用工具循环） |

## 查询入口（规范要求，实现可排期）

记录是为了能看。规范要求至少提供：

- `GET /api/ai/call-logs`（支持按 operation / session_id / capability / 时间范围过滤）
- 前端查看视图（按 session 分组回放一次编排）

> 现状：`GET /api/ai/call-logs` 已由 [`ai-call-logging-schema`](../../../../openspec/changes/ai-call-logging-schema) change 实现（支持 operation/session_id/capability/时间范围过滤）。前端查看视图（按 session 分组回放）待后续 change。

## 资料来源

基于 `backend-go/internal/platform/airouter/router.go`（`Chat`:102 / `Embed`:188）、`backend-go/internal/models/ai_models.go`（`AICallLog`:122）现状调研，
以及 2026-07-04 关于"日报 AI 调用 prompts 看不见 / agent 编排记录缺口"的评审讨论；
2026-07-07 补 `data_enrichment.debate_distill`（FinGenius 辩论结果提炼）及 Capability 映射表。
