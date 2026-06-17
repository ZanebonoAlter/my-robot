## Why

Syntopica 当前没有任何"开箱即看"的体验：陌生人想了解产品，必须先准备 PostgreSQL+pgvector、配 AI 凭证、跑 init.sh 拉数据，门槛极高。需要一个**只读、脱敏、公网可部署**的 demo 实例，让任何人 `docker compose up` 就能浏览完整产品形态（首页文章流、主题图谱、标签管理+侦探墙、设置页）。

数据流基础已具备：前端纯静态站 + 后端同源 serve 前端、AutoMigrate+版本迁移能自动建齐全部 schema、侦探墙等可视化直接从 daily_report 四张表取数展示。缺的是：(1) 业务数据 seed 机制完全不存在；(2) 没有脱敏代码；(3) 没有只读加固；(4) 没有 demo 专属部署制品。

## What Changes

- **导出脱敏工具** `backend-go/cmd/dump-sanitizer`：连真实运行库 → 按拓扑序导出脱敏数据 → 生成 `demo/seed/seed.sql`。只导最近 30 天数据（避免后端 `CURRENT_DATE - N days` 查询过滤导致空墙）。保守脱敏：清 AI 凭证/错误日志/firecrawl 原文，剥 URL query，哈希 session_id；保留标题/摘要/正文/标签名（公开新闻非敏感）。向量列置 NULL（展示链路不用向量，且规避维度耦合）。
- **只读加固**：新增 `middleware/readonly.go`，环境变量 `DEMO_READ_ONLY=1` 激活，拦截所有非 GET 请求（返回 405）+ 2 个有副作用的 GET SSE 端点（`merge-preview/scan/stream`、`merge-preview/evaluate/stream`）；`main.go` 在 demo 模式跳过 `StartRuntime`（否则定时任务会拉 RSS、调 LLM 破坏只读快照）；禁用 `/ws`。
- **demo 部署制品**：
  - `Dockerfile.demo`：多阶段自包含构建（node 构建前端 + golang 构建后端 + alpine 运行），构建期注入 `NUXT_PUBLIC_API_BASE=/api`（同源相对路径）
  - `demo/docker-compose.demo.yml`：postgres（pgvector，不挂载 data 卷即每次 fresh）+ syntopica-demo（注入 `DEMO_READ_ONLY=1`）
  - `demo/entrypoint.sh`：启动后端 → 等 health → psql 导入 seed.sql
  - `demo/README.md`：使用说明 + 安全警告

## Capabilities

### New Capabilities

- `public-read-only-demo`: 一键启动的只读脱敏 demo 实例，覆盖全页面展示

### Modified Capabilities

（无现有能力的需求变更——只读中间件对生产零影响，`DEMO_READ_ONLY` 默认未设）

## Impact

- `backend-go/cmd/dump-sanitizer/`：新增导出脱敏工具（`main.go` + `sanitize.go`，~350 行）
- `backend-go/internal/platform/middleware/readonly.go`：新增只读中间件（~45 行）
- `backend-go/internal/app/router.go`：挂载只读中间件（1 行）+ `/ws` demo 模式拦截
- `backend-go/cmd/server/main.go`：demo 模式跳过 `StartRuntime`（~5 行）
- `Dockerfile.demo`：新增多阶段构建
- `demo/`：新增目录（`entrypoint.sh`、`docker-compose.demo.yml`、`seed/seed.sql`、`seed/README.md`、`README.md`）
- 不改任何 `routes.go`、不改任何业务 handler 查询逻辑、不改现有 `Dockerfile`/`docker-compose.yml`、不改前端代码（同源相对路径 `/api` 已被 `front/app/utils/api.ts:resolveApiBase` 支持）

## Engineering Standards

本 change 遵循 `@docs/reference/开发执行规范.md`，适用条款：

- **§4.1 后端门禁**：`golangci-lint run ./... && go vet ./... && go test ./... && go build ./...`（改了 middleware/app 层，跑全量；dump-sanitizer 作为独立 cmd 单独 build 验证）
- **§11 归档门禁**：tasks.md 以「测试 / 文档 / 验证」三节收尾，验证节每条为可执行命令 + 期望结果；归档前重跑验证节确认零失败
- **§12 文档流转**：归档后把 change 移入当前里程碑 `docs/v1.x/changes/`
- **豁免**：demo seed.sql 为真实库导出的脱敏产物，不纳入单元测试（手工抽查）；docker compose 端到端验证记录在验证节但不计入 CI 门禁（本地执行确认）
- **安全约束**：导出工具必须零保留 `ai_providers.api_key`、`ai_call_logs` 全表不导、`session_id` 必须哈希——验证节用 grep 确认 seed.sql 无明文凭证
