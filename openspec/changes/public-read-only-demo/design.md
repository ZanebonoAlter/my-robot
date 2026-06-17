# Design — public-read-only-demo

> 本文档记录实现过程中的关键设计决策、约束发现与取舍。编号 D# 供 tasks.md / spec.md 引用。

## D1. 时间新鲜度——只导近期数据而非改后端查询

**问题**：侦探墙 timeline 查询硬编码 `period_date >= CURRENT_DATE - days * INTERVAL '1 day'`（`daily_report_repository.go:378`，默认 days=30）。若导出几个月前的真实数据，新用户打开看到空墙。同理 `articles` 列表、topic-graph 都有类似时间过滤。

**选项**：
- (a) 导出时把所有日期字段整体 rebasing 到"最近 N 天"
- (b) 改后端查询加 demo 模式（环境变量控制默认天数或允许看全量）
- (c) 只导近期数据

**决策：(c)**。理由：零后端改动、导出脚本最简、不打乱原时间间隔。代价是依赖近期确实有跑过抓取/日报任务——由用户在导出前确认。若近期数据稀薄，可在导出工具加 `--days` 参数放宽窗口。

**实现**：导出工具对带日期的表统一加 `WHERE created_at/pub_date/period_date >= NOW() - INTERVAL '30 days'`。可配置 `EXPORT_DAYS` 环境变量（默认 30）。

## D2. 前端 API 地址——同源相对路径，零前端改动

**问题**：`nuxt.config.ts:11` 默认 `apiBase = http://localhost:5000/api`，公网部署陌生人访问会白屏。

**关键发现**：`front/app/utils/api.ts:22 resolveApiBase()`：
```ts
function resolveApiBase(): string {
  const base = getConfigApiBase()
  if (base.startsWith('http')) return base   // 绝对 URL
  if (isDev()) return 'http://localhost:5000/api'
  return base   // 非绝对、非 dev → 原样返回（相对路径）
}
```
当 `apiBase = "/api"`（非 http 开头），生产构建后返回 `/api`，`apiClient`（`client.ts:42`）拼出 `/api/categories`，浏览器走同源。而后端 `static.go:12` 在 `/app/frontend` serve 前端静态文件——demo 容器前后端**同源同端口 5000**。

**决策**：构建期注入 `NUXT_PUBLIC_API_BASE=/api`（在 `Dockerfile.demo` 用 `ARG` + `ENV` 传给 `pnpm generate`）。**不改任何前端代码**。

**验证**：`resolveApiBase()` 返回 `/api` → `client.ts` fetch `/api/categories` → 同源 5000 端口命中后端。

## D3. 只读加固——单中间件覆盖，不改 routes.go

**问题**：全量路由有 ~40 个写端点（POST/PUT/DELETE）+ 2 个有副作用的 GET SSE（`merge-preview/scan/stream`、`merge-preview/evaluate/stream`，会触发嵌入+LLM）。

**发现**：`router.go:26` 所有业务路由都在 `api := r.Group("/api")` 下。挂一个中间件即可覆盖全部，无需逐个改 `routes.go`。

**决策**：新增 `middleware/readonly.go`，`DEMO_READ_ONLY=1` 激活：
- 放行 OPTIONS（CORS 预检）和 GET（除两个 SSE stream）
- 拦截其他 method → 405
- 拦截两个 stream path → 405

**`/ws` 处理**：WS 注册在 `router.go:24`（`r.GET("/ws")`），不在 `/api` group 下。WS 主要用于日报生成进度推送，demo 不生成日报，禁用无副作用。在 readonly 中间件外，于 `router.go` 注册 `/ws` 前判断 demo 模式：demo 模式不注册 `/ws`（前端 WS 连接失败时静默降级，不影响浏览）。

**AI 凭证返回**：`GET /api/ai/providers` 返回 `api_key` 字段——但导出工具已把 seed 里的 `api_providers.api_key` 清空为 `""`，GET 返回空值，无需改 handler。

## D4. 跳过 StartRuntime——demo 不跑定时任务

**问题**：`main.go:83 StartRuntime()` 会启动 8 个定时任务（`runtime.go`）：`auto_refresh`（拉 RSS）、`daily_report`（调 LLM）、`firecrawl`（爬取）、`content_completion` 等。demo 无网络/AI 凭证，这些会持续失败刷错误日志，且 `resetStaleStates` + 任务写库会污染只读快照。

**决策**：`main.go` 在 demo 模式（`DEMO_READ_ONLY=1`）跳过 `StartRuntime()` 和 `SetupGracefulShutdown(runtime)`，只保留 DB init + 路由 + 静态文件。`resetStaleStates` 的状态重置对正常 seed 数据无害（seed 里状态本就是 idle/pending），但为干净起见整体跳过。

**实现**：
```go
if os.Getenv("DEMO_READ_ONLY") != "1" {
    runtime := appbootstrap.StartRuntime()
    appbootstrap.SetupGracefulShutdown(runtime)
}
```

## D5. 数据导入时序——entrypoint healthcheck 等待

**问题**：seed.sql 必须在 AutoMigrate+Migration 建表之后导入，否则表不存在。

**选项**：
- (a) entrypoint 脚本：后台启后端 → 轮询 `/health` 200 → psql 导入 seed → wait 后端
- (b) docker-compose 用一次性 init 容器（depends_on 后端 healthy）跑 psql

**决策：(a)**。理由：单容器自包含、不引入额外 service、compose 文件简洁。entrypoint.sh 里 `until curl -sf http://localhost:5000/health; do sleep 1; done` 等后端起来（后端启动会跑完 AutoMigrate+Migration），然后 psql 导入。

**注意**：alpine 镜像需装 `curl` 和 `postgresql-client`（psql）。`Dockerfile.demo` runtime 阶段 `apk add curl postgresql-client`。

## D6. 导出工具实现策略——通用行扫描 + 统一转义

**问题**：逐表写 GORM struct 映射样板代码量大（20+ 表，含 JSONB/timestamp/NULL/vector 多种类型）。

**决策**：用 `database/sql` + `sql.RawBytes` 通用扫描，配合：
- 每张表一个 `ExportTable` 配置（表名、列清单、WHERE 条件、字段脱敏函数 map、SELECT 投影覆盖如向量列置 NULL）
- 统一的 `sqlValueLiteral(interface{}) string` 把任意 SQL 值转成 PG INSERT 字面量（处理 NULL、字符串转义、bool、数字、[]byte JSONB、time.Time）
- 向量列在 SELECT 投影里用 `NULL::vector AS embedding`（而非取出来再转）
- 自引用 FK（`topic_tags.merged_into_id`）：分两批 WHERE，先 `IS NULL` 后 `IS NOT NULL`
- 每张表导完附 `SELECT setval(pg_get_serial_sequence(...), COALESCE(MAX(id),0)+1, false)` 重置序列

**排除列**：`articles.category_id`（`gorm:"-"` 假列，不在 DB）——通过显式列清单控制，不 SELECT 它。

## D7. 脱敏字段映射表（保守策略）

| 表 | 字段 | 策略 | 理由 |
|---|---|---|---|
| ai_providers | api_key | `""` | 凭证 |
| ai_providers | base_url | `""` | 可能含内网网关 |
| ai_providers | metadata | `"{}"` | 可能含 headers |
| articles | link | 剥 query string | 去跟踪参数 |
| articles | image_url | 剥 query string | 同上 |
| articles | firecrawl_content | `""` | 最可能含 PII |
| articles | firecrawl_error | `""` | 可能泄露内部 URL |
| articles | completion_error | `""` | 同上 |
| feeds | url | 剥 query string | 去私域 token |
| feeds | refresh_error | `""` | 错误文本 |
| reading_behaviors | session_id | sha256 哈希 | 用户会话标识 |
| ai_call_logs | 整表 | 跳过 | request_meta/response_snippet 高危 |
| topic_tag_embeddings | embedding | NULL (SELECT 投影) | 维度耦合 |
| semantic_labels | embedding, merge_embedding | NULL | 同上 |
| daily_report_sections | embedding | NULL | 同上 |
| 标题/摘要/正文/标签名/板块名 | — | 保留 | 公开新闻非敏感 |

**跳过的表**：`schema_migrations`（会让新库 migration 失效）、`ai_call_logs`、`otel_spans`、`topic_tag_analyses`、`topic_analysis_cursors`、`tag_merge_suggestions`（均为日志/分析中间态，展示链路不用）。

## D8. 导出顺序（拓扑分层）

唯一物理 FK 是 `topic_tags_merged_into_id_fkey`（`ON DELETE CASCADE`，其余 FK 被 `DisableForeignKeyConstraintWhenMigrating` 禁用）。导入顺序：

1. 叶子表：categories, semantic_labels, ai_providers, ai_routes, ai_settings, embedding_config, scheduler_tasks, narrative_boards
2. topic_tags WHERE merged_into_id IS NULL
3. feeds, articles（排除 category_id 假列）
4. topic_tags WHERE merged_into_id IS NOT NULL
5. 关联表：article_topic_tags, topic_tag_relations, topic_tag_semantic_labels, topic_tag_board_labels, board_composition, ai_route_providers, narrative_summaries
6. 日报四件套：board_daily_reports, daily_report_sections, daily_report_threads, daily_report_section_relations
7. 队列/行为（可空）：embedding_queues, tag_jobs, firecrawl_jobs, reading_behaviors, user_preferences

## D9. Dockerfile.demo 多阶段构建

现有 `Dockerfile` 要求本地预编译 Go 二进制（`ARG BINARY_PATH`），demo 要自包含。三阶段：
1. `node:22-alpine`：`pnpm generate`（`NUXT_PUBLIC_API_BASE=/api` 通过 `ENV` 注入）
2. `golang:1.24-alpine`：`go build -o syntopica ./cmd/server`
3. `alpine:3.22`：拷前端（`.output/public` → `frontend/`）+ go 二进制 + `backend-go/configs` + `demo/seed/seed.sql` + `demo/entrypoint.sh`；`apk add curl postgresql-client`；`ENTRYPOINT ["/app/entrypoint.sh"]`

**Go 版本**：以 `backend-go/go.mod` 的 go directive 为准（实现时确认，预期 1.24）。

## Risks

- **seed.sql 体积**：若近期文章含大量 content 文本，可能达 10MB+。导出工具加行数/字节数统计日志；若超预期，可在脱敏时截断 `articles.content` 到前 N 字符（当前保留全文）。
- **唯一约束冲突**：`feeds.url` 脱敏剥 query 后若产生重复，psql 导入会撞 unique。导出工具对 url 脱敏后做去重（保留第一条），或在 INSERT 语句加 `ON CONFLICT (url) DO NOTHING`。
- **recent data 稀薄**：若近 30 天数据少，demo 展示效果差。由用户导出前确认；`EXPORT_DAYS` 可调。
