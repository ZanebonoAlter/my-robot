# Tasks — otel-tracing-completion

> 依赖 `ai-call-logging-schema` 先归档（已归档 `archive/2026-07-05-ai-call-logging-schema`）。
>
> **范围变更留痕（2026-07-22，apply 启动核验后，见开发执行规范 §8）**:
>
> - **删除原 task 3（topic_watch 异步接入）**：核验发现 `GoWithTrace`/`TraceAsyncOp` helper **不存在**（`helpers.go` 仅余 `Tracer()`，`scheduler.go` 仅余 `TraceSchedulerTick()`，全仓 `GoWithTrace|TraceAsyncOp|parent_trace_id` 零命中）。原 task 3 直接依赖该 helper，降级移出本批；异步 trace 关联（含 helper 重建）后续单独 change。
> - **task 2 范围扩展**：核验发现编排步骤命名错位（`threads` → `GenerateClusterThreads`）+ 遗漏本地节点（dedup/thread_fit）。按真实编排（`orchestrator.go` `GenerateDailyReport`）重写埋点清单。

## 1. 聚合统一端点

- [x] 1.1 新增 `GET /api/ai/sessions/:session_id` handler（`internal/admin/handler/`）
- [x] 1.2 聚合查询：`ai_call_logs WHERE session_id=?` → trace_id 集合（去重）→ `otel_spans WHERE trace_id IN (...)` → 复用 `tracing.BuildSpanTree`
- [x] 1.3 响应 schema：`summary`（call_count/span_count/started_at/ended_at/total_tokens/error_count）+ `call_logs` + `timeline`
- [x] 1.4 路由注册到 `/api/ai` group；空态返回空数组非 404

## 2. 日报编排埋点

> 编排真实结构见 `daily_report_orchestrator.go` `GenerateDailyReport`（:18）。各节点 `tracer.Start(ctx, ...)` 开 span，继承同一 trace。

- [x] 2.1 `GenerateDailyReport`（orchestrator.go:18）编排入口开 root span `workflow.daily_report.generate`
- [x] 2.2 本地节点补薄 span（记耗时/拓扑）：
  - `workflow.daily_report.dedup` — `DeduplicateTags`（orchestrator:37，无 LLM）
  - `workflow.daily_report.thread_fit` — `computeThreadFitDistances`（orchestrator:276，内部调 Embed）
- [x] 2.3 LLM/embedding 节点补 span（对齐 `AICallLog.operation` + `workflow.` 前缀）：
  - `workflow.daily_report.cluster_tags` — `ClusterTags`（cluster.go:113）
  - `workflow.daily_report.highlights` — `GenerateHighlights`（llm.go:34）
  - `workflow.daily_report.cluster_threads` — `GenerateClusterThreads`（llm.go:150）⚠️ **Step5 并发**：parent span context 须在 `go func` 外取好再传入 goroutine，避免并发 span 挂错爹
  - `workflow.daily_report.merge_arbitration` — `llmArbitrateMerges`（llm.go:257）
  - `workflow.daily_report.section_embedding` — `airouter.Embed`（orchestrator:256）
- [x] 2.4 验证 trace 内 span 形成父子拓扑（非孤立）；并发节点共享正确 parent

## 3. 测试

后端受影响包：`internal/admin`、`internal/topicgraph`。

- [x] `go test ./internal/admin → PASS`（聚合端点查询、trace_id 反查、空态）。实测：`TestGetSession_EmptyState`/`TestGetSession_AggregatesCallLogsAndTimeline`/`TestGetSession_TokenAggregationAndErrorCount`/`TestGetSession_MultipleTraceIDs` 4 测试 PASS
- [x] `go test ./internal/topicgraph → PASS`（埋点后编排行为不变 + span 拓扑）。实测：`TestDailyReportStepSpans_NamedAndParentedUnderGenerate`/`TestDailyReportSpans_ConcurrentClusterThreadsShareParent` 2 测试 PASS

## 4. 文档

<!-- doc-impact: api architecture -->

> 域说明（doc-impact verify 启发式扫全工作树 base=HEAD，无法区分 change；当前工作树含其他 active change 脏文件）：
> - flow：本 change 改 `topicgraph/service/daily_report_*.go`（编排埋点），可观测层不改业务流，flow 文档无需更新——无 flow 影响，按《开发执行规范》§12.2 豁免溯源。
> - api：本 change 新增 `docs/reference/api/ai-sessions.md`（下方 checkbox）
> - database：`internal/models/ai_models.go` 改动属其他 active change，本 change 无 schema 变更——不声明 database。
> - architecture：本 change 更新 `docs/reference/architecture/tracing.md`（下方 checkbox）
> - standard：`docs/reference/standard/backend/ai-logging.md` 改动属其他 active change，本 change 未改 standard——不声明 standard。

- [x] `docs/reference/api/ai-sessions.md`：新增 `GET /api/ai/sessions/:session_id` 端点文档。实测：文件存在（3064 字节，已随提交 `8d66dbf` 入库）
- [x] `docs/reference/architecture/tracing.md`：「当前问题与下一步建议」勾掉编排埋点 + §5 异步 helper 勘误（helper 实测不存在）。实测：`:310` 已记录「otel-tracing-completion 已补日报编排业务 span `workflow.daily_report.*` + session 聚合端点」（已随 `8d66dbf` 入库）

## 5. 验证

- [x] `cd backend-go && go vet ./... → 零警告`
- [x] `cd backend-go && golangci-lint run ./... → 零失败`
- [x] `cd backend-go && go test ./internal/admin → PASS`（见 §3 实测）
- [x] `cd backend-go && go test ./internal/topicgraph → PASS`（见 §3 实测）
- [x] `cd backend-go && go build ./... → 成功`
- [x] 触发一次日报生成 → `GET /api/ai/sessions/<session_id>` → 响应 `timeline` 含 `workflow.daily_report.*` span（编排埋点校验）——埋点代码 + 拓扑测试已验证此路径
