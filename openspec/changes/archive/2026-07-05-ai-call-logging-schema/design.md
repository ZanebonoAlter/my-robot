# Design — ai-call-logging-schema

> 把 [`standard/backend/ai-logging.md`](../../../docs/reference/standard/backend/ai-logging.md) R1-R4 红线落地为 schema + 代码 + 查询入口。

## 设计目标

让每一次 AI 调用**可观测、可回放、可审计**。可观测 = 字段齐全；可回放 = prompt/response 完整；可审计 = 按编排（session）重建。

## 1. schema 补字段

`ai_call_logs` 现有 12 列（`ai_models.go:122-140`），本 change 新增 4 列：

| 新列 | 类型 | 约束 | 说明 |
|------|------|------|------|
| `operation` | varchar(80) | NOT NULL（迁移后；历史行回填 `unknown`） | 业务操作名，如 `daily_report.cluster_tags` |
| `prompt` | text | nullable | 完整 messages 文本（拼接 system+user），超 20000 runes 截断标 `[truncated]` |
| `token_usage` | jsonb | nullable | `{"prompt":n,"completion":n,"total":n}` |
| `session_id` | varchar(120) | nullable | 编排分组键；非编排的单次调用可为 NULL |

**为什么不单独建 prompt 表**：单用户系统，调用量不大（日均百级），text 列直接挂主表够用，避免 join 复杂度。若将来 prompt 体量爆炸再拆。

**迁移**（§10 显式迁移，非 AutoMigrate）：
- `ALTER TABLE ai_call_logs ADD COLUMN ...`（4 列）。
- `UPDATE ai_call_logs SET operation='unknown' WHERE operation IS NULL`（回填历史，再 NOT NULL）。
- 索引：`CREATE INDEX idx_call_logs_session ON ai_call_logs(session_id)`、`CREATE INDEX idx_call_logs_op_time ON ai_call_logs(operation, created_at)`。

## 2. airouter 改造

### 2.1 ChatRequest 加字段

`router.go` 的 `ChatRequest`（或等价入参）新增：
```go
Operation string   // 必填，业务操作名
SessionID string   // 可选，编排分组键
```

**Operation 必填校验**：`Router.Chat` 入口检查 `req.Operation != ""`，空则返回 error（防"忘记填"导致日志无法分组）。这是把规范从"自觉"变"强制"的关键。

### 2.2 LogCall 落全量

`Router.Chat` 现有 `LogCall`（`router.go:143`/`:164`）补写：
- `Operation` = `req.Operation`
- `SessionID` = `req.SessionID`
- `Prompt` = `formatMessages(req.Messages)`（拼接为文本，截断）
- `TokenUsage` = 从 provider 响应的 `usage` 字段取（OpenAI 兼容接口都返回）

`Router.Embed` 路径同样补 Operation/SessionID（Embed 不记 prompt，但记 operation/token）。

### 2.3 span attribute 桥梁（连接 OTel）

现状 `router.go:111-113` Chat span 已打 `ai.capability` + `ai.operation`，但 operation 是从 `req.Metadata["operation"]` 弱读取（调用方漏填则为空）；`Router.Embed`（`router.go:191`）只打了 capability 没打 operation；两边都没有 session_id。

本 change 把 attribute 来源切到一等字段，三处统一：

| span | attribute | 来源 |
|------|-----------|------|
| `Router.Chat` | `ai.capability` | `req.Capability`（不变） |
| `Router.Chat` | `ai.operation` | `req.Operation`（**改为**一等字段，不再读 Metadata） |
| `Router.Chat` | `ai.session_id` | `req.SessionID`（**新增**） |
| `Router.Embed` | `ai.capability` | `capability` 参数（不变） |
| `Router.Embed` | `ai.operation` | `req.Operation`（**新增**） |
| `Router.Embed` | `ai.session_id` | `req.SessionID`（**新增**） |

**为什么放进本 change**：session_id 是本 change 引入的一等字段，attribute 桥梁是它的自然延伸，零额外风险；且它是后续「按 session 聚合统一端点」change 的前置——不把 session_id 打进 `otel_spans`，聚合端点的 OTel 侧就查不到。

**向后兼容**：业务侧现状已在 Metadata 传 operation（日报 7 处、tagmanagement 多处），attribute 来源切换后 span 值不变（同一份数据从字段读而非 map 读），只是更可靠。Metadata["operation"] 在本 change 后可保留亦可移除（airouter 不再读它，旧调用方过渡期保留无害）。

### 2.4 formatMessages 截断策略

```
[system]\n{system_content}\n\n[user]\n{user_content}\n...
```
超 20000 runes 时：保留前 18000 + `\n...[truncated N runes]...\n` + 后 2000。头尾都保留，便于排查。

## 3. 现有调用补齐清单

规范「补齐跟踪表」的执行。每处补 `Operation` + `SessionID`：

| 文件 | 函数 | Operation 值 |
|------|------|-------------|
| `daily_report_cluster.go:133` | `ClusterTags` | `daily_report.cluster_tags` |
| `daily_report_llm.go:43` | `GenerateHighlights` | `daily_report.highlights` |
| `daily_report_llm.go:158` | `GenerateClusterThreads` | `daily_report.threads` |
| `daily_report_llm.go:287` | `llmArbitrateMerges` | `daily_report.merge_arbitration` |
| `daily_report_orchestrator.go:256` | section `.Embed` | `section.embedding` |
| `daily_report_watch.go` | `EvaluateWatchHits` | `topic_watch.evaluate` |

**SessionID 统一生成**：日报编排入口 `GenerateAndSaveReport` 生成 `daily_report_{board_id}_{uuid8}`（用 board_id 而非 report_id——report.ID 由 GORM 在 SaveReport 后才填充，而 SessionID 必须在 LLM 调用前注入；board_id+uuid8 已保证唯一可分组），通过 context 传给上述所有调用。`topic_watch.evaluate` 用同一个 session_id（它在日报事务后跑）。

## 4. 查询 API

`GET /api/ai/call-logs`（`internal/admin/routes.go` 的 `/ai` group）：

查询参数：
- `operation`（精确匹配，如 `daily_report.cluster_tags`）
- `session_id`（精确匹配——按编排回放）
- `capability`（精确匹配）
- `from` / `to`（时间范围，ISO8601）
- `limit`（默认 50，上限 200）/ `offset` 分页

响应：`gin.H{"success":true,"data":[{...}]}`，每条含 id/operation/session_id/capability/provider/model/success/latency_ms/token_usage/prompt(截断预览)/response_snippet/created_at。

**按 session 回放**：前端按 `session_id` GROUP BY 展开时间线（本 change 前端不做，仅 API 支撑）。

## 5. 风险与取舍

- **prompt 落库的体积**：日报单次 prompt 约 2-5k tokens，日均百级调用 → 日增 ~1MB text。7 天清理 job 覆盖，可接受。
- **Operation 必填的回归风险**：现有调用若漏填会被 airouter 拒绝。缓解：补齐清单逐个改 + 单测覆盖，迁移期可选"空则记 unknown 不报错"过渡一轮后再收紧。本 change 采用**直接收紧**（少一次过渡债务）。
- **trace_id 保留**：不删 trace_id 列，它与 session_id 正交（OTel 链路 vs 业务编排），共存。
- **span attribute 来源切换**：把 `ai.operation` 从读 Metadata 改为读一等字段，是 MODIFY `otel-business-tracing` 现有 requirement（`LLM span carries business attributes`）。现状业务侧已习惯在 Metadata 传 operation，切换后过渡期 Metadata 可保留，待后续 change 收敛调用方后移除，避免一次性改太多处。

## 6. 不做什么

- 不做前端查看页面（留 API，前端后续 change 接）。
- 不改清理保留期（7 天不变，配置层调）。
- 不脱敏 prompt（单用户系统，调试需要完整 prompt）。
