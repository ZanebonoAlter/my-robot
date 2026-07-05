# Tasks — otel-tracing-completion

> 依赖 `ai-call-logging-schema` 先归档。

## 1. 聚合统一端点

- [ ] 1.1 新增 `GET /api/ai/sessions/:session_id` handler（`internal/admin/`）
- [ ] 1.2 实现聚合查询：`ai_call_logs WHERE session_id=?` → trace_id 集合（去重）→ `otel_spans WHERE trace_id IN (...)` → 复用 `tracing.BuildSpanTree`
- [ ] 1.3 组装响应 schema：`summary`（call_count/span_count/started_at/ended_at/total_tokens/error_count）+ `call_logs` + `timeline`
- [ ] 1.4 路由注册到 `/api/ai` group；空态返回空数组非 404

## 2. 日报编排埋点（待 apply 阶段对着实际代码决策）

> **apply 前先 `codegraph`/`read` 摸清** `daily_report_cluster.go`/`daily_report_llm.go`/`daily_report_orchestrator.go` 的函数签名与 ctx 传递路径，再细化本节 task。本节为框架，函数级细节 apply 时补。

- [ ] 2.1 编排入口 `GenerateAndSaveReport` 确保 trace ctx 启动（或在入口开 `workflow.daily_report.generate` root span）
- [ ] 2.2 各步骤函数补 span：`workflow.daily_report.cluster_tags` / `.highlights` / `.threads` / `.merge_arbitration` / `.section_embedding`
- [ ] 2.3 确认 ctx 传递路径连续（参照前置 otel change `2026-04-30-otel-business-tracing-improvements` 的 ctx 补齐模式），必要时补函数 ctx 参数
- [ ] 2.4 验证 trace 内 span 形成父子拓扑（非孤立 span）

## 3. topic_watch 异步接入

- [ ] 3.1 `daily_report_watch.go` 的 goroutine 改用 `tracing.GoWithTrace`，传 `parent_trace_id` attribute
- [ ] 3.2 确认 watch span 在聚合端点能按 session/trace 关联呈现

## 4. 测试

后端受影响包：`internal/admin`、`internal/topicgraph`。

- `go test ./internal/admin → PASS`（聚合端点查询、trace_id 反查、空态）
- `go test ./internal/topicgraph → PASS`（埋点后编排行为不变 + span 拓扑）

## 5. 文档

- [ ] `docs/reference/api/`：新增 session 聚合端点文档（并入 `traces.md` 或新建 `ai-sessions.md`）
- [ ] `docs/reference/architecture/tracing.md`：「当前问题与下一步建议」勾掉已补项（go-instrument attributes、异步 helper 接入）

## 6. 验证

- `cd backend-go && go vet ./... → 零警告`
- `cd backend-go && golangci-lint run ./... → 零失败`
- `go test ./internal/admin → PASS`
- `go test ./internal/topicgraph → PASS`
- `cd backend-go && go build ./... → 成功`
- 触发一次日报生成 → `GET /api/ai/sessions/<该 session_id>` → 响应 `timeline` 含 `workflow.daily_report.*` span（编排埋点校验）
