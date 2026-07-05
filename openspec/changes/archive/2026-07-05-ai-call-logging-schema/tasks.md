# Tasks — ai-call-logging-schema

## 1. schema 迁移

- [x] 1.1 编写迁移：`ai_call_logs` 新增 `operation varchar(80)`、`prompt text`、`token_usage jsonb`、`session_id varchar(120)` 四列
- [x] 1.2 历史行回填 `operation='unknown'`，再加 NOT NULL 约束
- [x] 1.3 建索引：`idx_call_logs_session(session_id)`、`idx_call_logs_op_time(operation, created_at)`
- [x] 1.4 更新 `docs/reference/database/DATABASE_FIELDS.md` 的 `ai_call_logs` 表（补本次 4 列 + 漏列 response_snippet/trace_id）

## 2. airouter 改造

- [x] 2.1 `ChatRequest`（或入参 struct）新增 `Operation string`、`SessionID string` 字段
- [x] 2.2 `Router.Chat` 入口校验 `Operation != ""`，空则 return error
- [x] 2.3 `LogCall` 落 `Operation`/`SessionID`/`Prompt`/`TokenUsage`；实现 `formatMessages` 拼接 + 20000 runes 截断
- [x] 2.4 `Router.Embed` 路径补 Operation/SessionID（不记 prompt，记 operation/token）
- [x] 2.5 从 provider 响应提取 `usage` 写入 `token_usage`
- [x] 2.6 `Router.Chat`/`Router.Embed` span attribute 改为从 `req.Operation`/`req.SessionID` 一等字段注入（替换 `router.go:111-113` 的 Metadata 弱读取），新增 `ai.session_id` attribute，补齐 `Router.Embed` 的 `ai.operation`（同步移除 airouter 对 `Metadata["operation"]` 的读取依赖）

## 3. 现有调用补齐（规范补齐跟踪表执行）

- [x] 3.1 日报编排入口 `GenerateAndSaveReport` 生成 `SessionID=daily_report_{board_id}_{uuid8}`，通过 context 传递
- [x] 3.2 `ClusterTags` → Operation=`daily_report.cluster_tags`
- [x] 3.3 `GenerateHighlights` → `daily_report.highlights`
- [x] 3.4 `GenerateClusterThreads` → `daily_report.threads`
- [x] 3.5 `llmArbitrateMerges` → `daily_report.merge_arbitration`
- [x] 3.6 section `.Embed` → `section.embedding`
- [x] 3.7 `EvaluateWatchHits` → `topic_watch.evaluate`（复用日报 session_id）

## 4. 查询 API

- [x] 4.1 `internal/admin/` 新增 `GET /api/ai/call-logs` handler，支持 operation/session_id/capability/from/to/limit/offset
- [x] 4.2 路由注册到 `/ai` group（`admin/routes.go`）
- [x] 4.3 limit 上限 200 校验

## 5. 测试

后端受影响包：`internal/platform/airouter`、`internal/topicgraph`、`internal/admin`。

- `go test ./internal/platform/airouter → PASS`（Operation 必填校验、formatMessages 截断、token 落库）
- `go test ./internal/topicgraph → PASS`（日报调用补齐 Operation/SessionID 后行为不变）
- `go test ./internal/admin → PASS`（call-logs 查询过滤与分页上限）

## 6. 文档

- [x] `docs/reference/database/DATABASE_FIELDS.md`：ai_call_logs 表字段表补齐（本次 5 列 + 漏列 response_snippet/trace_id + model）
- [x] `docs/reference/api/`：新增 `ai-call-logs.md` 记录 GET 端点
- [x] `docs/reference/standard/backend/ai-logging.md`：补齐跟踪表的 ⚠️ 项转为 ✅
- [x] 事故复盘规范（token_usage JSONB 空串 + operation AutoMigrate 竞争）：`standard/backend/code-style.md` 加「GORM model tag 与迁移」节、`testing.md` 加「JSONB 列空串陷阱」「schema 迁移要在 testcontainer PG + 历史数据下测」两节、开发执行规范 §10 加强 DDL 变更门禁
- 无 flow 影响：本 change 是横切 AI 调用日志 schema（给 airouter 调用补 prompt/session_id/operation 落库 + 查询端点），不改任何业务 flow 的生成/编排流程，按《开发执行规范》§12.2 豁免 flow 变更溯源

## 7. 验证

- `cd backend-go && go vet ./... → 零警告`
- `cd backend-go && golangci-lint run ./... → 零失败`
- `go test ./internal/platform/airouter → PASS`
- `go test ./internal/topicgraph → PASS`
- `go test ./internal/admin → PASS`
- `cd backend-go && go build ./... → 成功`
- `grep -rn "Operation:" backend-go/internal/topicgraph/service/*.go → 至少命中 6 处（5 日报 + 1 watch）`（补齐校验）
- `grep -rn "session_id" backend-go/internal/platform/airouter/ → LogCall 已落 session_id`
- `grep -rn "ai.session_id\|ai.operation" backend-go/internal/platform/airouter/router.go → Chat 与 Embed 两处均注入 session_id + operation 一等字段`（span 桥梁校验）
