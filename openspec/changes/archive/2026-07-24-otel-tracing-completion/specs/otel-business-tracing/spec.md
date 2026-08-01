## ADDED Requirements

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
