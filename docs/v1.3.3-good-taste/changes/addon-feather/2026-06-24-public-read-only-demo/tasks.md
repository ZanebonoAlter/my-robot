## 0. 前置条件检查（§5.0）

- [x] 0.1 `cd backend-go && go build ./...` 当前可正常编译（基线）
- [ ] 0.2 本地真实运行库可连（`docker compose -f docker-compose.pg.yml up -d` 已起 + 后端跑过有业务数据）；近期 30 天内有文章/日报数据，否则 demo 展示稀薄
- [x] 0.3 Docker 可用：`docker compose version` 正常
- [x] 0.4 Go 版本确认：`findstr "^go " backend-go/go.mod` → `go 1.25.0`（Dockerfile.demo 用 `golang:1.25-alpine`）

## 1. 只读中间件（middleware/readonly.go）

- [x] 1.1 新建 `backend-go/internal/platform/middleware/readonly.go`：
  - 导出 `ReadOnly() gin.HandlerFunc`
  - 读 `os.Getenv("DEMO_READ_ONLY")`；非 `"1"` 时返回透传 handler（`c.Next()`），生产零影响（design D3）
  - `"1"` 时：放行 `OPTIONS`（CORS 预检）→ 非 `GET` 返回 `405 {"error":"read-only demo"}` → `GET` 但 path 含 `/merge-preview/scan/stream` 或 `/merge-preview/evaluate/stream` 返回 405 → 其余 `GET` 放行
- [x] 1.2 `backend-go/internal/app/router.go:26` 在 `api := r.Group("/api")` 后加 `api.Use(middleware.ReadOnly())`（import `syntopica-backend/internal/platform/middleware` 已存在，CORS 同包）

## 2. demo 模式跳过定时任务 + 禁用 WS（main.go / router.go）

- [x] 2.1 `backend-go/cmd/server/main.go:83` 包裹 `StartRuntime` + `SetupGracefulShutdown`：`if os.Getenv("DEMO_READ_ONLY") != "1" { ... }`（design D4）
- [x] 2.2 `backend-go/internal/app/router.go:24` 把 `r.GET("/ws", ws.HandleWebSocket)` 包裹：demo 模式不注册 `/ws`（前端 WS 连不上时静默降级，不影响浏览）
- [x] 2.3 确认 `main.go` 顶部已 `import "os"`（若无需补）

## 3. 导出脱敏工具（cmd/dump-sanitizer）

- [x] 3.1 新建 `backend-go/cmd/dump-sanitizer/main.go`：
  - 复用 `config.LoadConfig("./backend-go/configs")` 读 DSN（或支持 `DATABASE_DSN` 环境变量覆盖）；用 `gorm.io/driver/postgres` + `gorm.Open` 连库
  - 输出文件路径：`os.Getenv("SEED_OUT")` 或默认 `../demo/seed/seed.sql`（相对 `backend-go/`）
  - 时间窗口：`os.Getenv("EXPORT_DAYS")` 默认 `30`
  - 文件头写注释（生成时间、源库 dsn 脱敏、窗口天数）
- [x] 3.2 新建 `backend-go/cmd/dump-sanitizer/sanitize.go`：
  - 定义 `ExportSpec` 结构：`Table`、`Columns`（显式列，排除假列如 `articles.category_id`）、`Where`（`:days` 占位的时间/分批条件）、`VectorColumns`（SELECT 投影置 `NULL::vector`）、`Sanitizers map[string]func(string)string`、`ConflictClause`（脱敏后唯一键碰撞时的 `ON CONFLICT`，design D6a）、`NoSequence`（复合主键表跳过 setval）
  - 实现脱敏函数：`clearAll`（→ `""`）、`emptyJSON`（→ `{}`）、`stripQuery`（剥 URL query）、`sha256Hash`（确定性哈希）、`truncateContent(maxLen)`（**按 rune 截断** + 省略标记，避免切断多字节 UTF-8，design D6a）、`composeSanitizers(fns...)`（串联多个脱敏）、`redactSensitiveTokens`（`api_key/API_KEY/api-key` → `[redacted-token]`，design D6a）、`rewriteRSSHubHost`（自建 RSSHub host → 官方 `rsshub.app`，由 `RSSHUB_REWRITE=源host=目标host` 配置，默认替换 demo 数据源自建 host，防基础设施泄露，design D6a）
  - 实现单一 `quoteString(s)`：`'`→`''`，依赖 PG unknown 字面量对目标列类型的隐式转换（integer/numeric/bool/timestamp/jsonb 都成立）；NULL 由 exporter 在值无效时单独输出关键字（不再为每类型写 sqlLiteral）
  - 脱敏映射表按 design D7
- [x] 3.3 新建 `backend-go/cmd/dump-sanitizer/tables.go`（或在 main.go 内）：按 design D8 顺序定义 18 张导出表的 `ExportSpec`：
  - 叶子（无 WHERE）：categories, semantic_labels, ai_providers, ai_routes, ai_settings, embedding_config, scheduler_tasks, narrative_boards
  - topic_tags 批次1：`WHERE merged_into_id IS NULL AND updated_at >= NOW() - INTERVAL ':days days'`
  - feeds（`WHERE created_at >= ...`，`url` 用 `composeSanitizers(rewriteRSSHubHost, stripQuery)`——先把自建 RSSHub host 替换为官方 `rsshub.app` 防基础设施泄露，再剥 query；`ConflictClause: "ON CONFLICT (url) DO NOTHING"` 防脱敏后唯一键碰撞，design D6a）, articles（`WHERE created_at >= ...`，列排除 `category_id`；`content`/`ai_content_summary` 用 `composeSanitizers(redactSensitiveTokens, truncateContent(2000))`（按 rune 截断），`link`/`image_url` 剥 query，firecrawl/error 字段清空）
  - ai_settings：`value` 字段脱敏为 `{}`（配置 JSON 整体清空，design D6a）
  - topic_tags 批次2：`WHERE merged_into_id IS NOT NULL`
  - 关联：article_topic_tags, topic_tag_relations, topic_tag_semantic_labels, topic_tag_board_labels, board_composition, ai_route_providers, narrative_summaries
  - 日报：board_daily_reports（`WHERE period_date >= ...`）, daily_report_sections, daily_report_threads, daily_report_section_relations
  - 行为/队列（`WHERE created_at >= ...`）：reading_behaviors, user_preferences（其余 embedding_queues/tag_jobs/firecrawl_jobs 可选，默认跳过以减体积）
- [x] 3.4 主循环：遍历 specs → `db.Query(buildSelect(spec))` → `rows.Columns()` + 扫描到 `[]sql.NullString` → 逐行套用 sanitizers（`raw[i].Valid` 才脱敏，否则输出 `NULL`）→ `quoteString` 单引号转义 → 拼 `INSERT INTO <table> (cols) VALUES (...), (...)` → 每 500 行一条，`ConflictClause`（若有）拼在 `;` 前 → 末尾 `SELECT setval(pg_get_serial_sequence('<table>','id'), COALESCE(MAX(id),0)+1, false) FROM <table>;`
  - 向量列投影：若 `spec.VectorColumns` 非空，SELECT 里把该列替换为 `NULL::vector AS <col>`
  - 批量 INSERT 每 500 行一条语句（控制单语句大小）
  - error 用 `run() error` 模式返回，避免 `exitAfterDefer`（defer db/file.Close 在退出前执行）
- [x] 3.5 写完 flush 文件，日志输出每张表行数 + 总字节数（监控体积，design Risks）

## 4. demo 部署制品（Dockerfile.demo / compose / entrypoint）

- [x] 4.1 新建 `Dockerfile.demo`（三阶段，design D9/D9a）：
  - Stage 1 `node:22-alpine`：`corepack enable` → COPY `front/package.json`+`pnpm-lock.yaml`+`front/patches` → `ENV NUXT_PUBLIC_API_BASE=/api` → `pnpm install --no-frozen-lockfile`（仓库 lockfile patchedDependencies 不一致，非本 change 引入，design D9a）→ COPY `front/` → `pnpm generate`
  - Stage 2 `golang:1.25-alpine`：`ARG GOPROXY=https://goproxy.cn,direct` + `ENV GOPROXY` → COPY `go.mod/go.sum` → `go mod download`（3 次重试循环，design D9a）→ COPY `backend-go/` → `CGO_ENABLED=0 go build -trimpath -o /syntopica ./cmd/server`
  - Stage 3 `alpine:3.22`：`apk add --no-cache ca-certificates tzdata curl postgresql-client` + `adduser` → WORKDIR `/app` → COPY 二进制→`/app/syntopica` → COPY `backend-go/configs`→`/app/configs` → COPY `--from=front-build /app/.output/public/`→`/app/frontend/` → COPY `demo/seed/seed.sql`→`/app/seed.sql` → COPY `demo/entrypoint.sh`→`/app/entrypoint.sh` + `chmod +x` → `ENTRYPOINT ["/app/entrypoint.sh"]`
  - **不写** `# syntax=docker/dockerfile:1`（会触发额外 frontend 镜像拉取，design D9a）
- [x] 4.2 新建 `demo/entrypoint.sh`（design D5/D5a，LF 行尾）：
  - `#!/bin/sh` + `set -e`
  - 后台启后端 `/app/syntopica &`，记 `BACKEND_PID`
  - `until curl -sf http://localhost:5000/health; do sleep 1; done`（等 AutoMigrate+Migration 完成）
  - **导入前 TRUNCATE 清场**：`psql -v ON_ERROR_STOP=1` 执行 `TRUNCATE TABLE <所有 demo 涉及表> RESTART IDENTITY CASCADE`（后端启动会 seed 默认数据，不清场会撞 duplicate key，design D5a）
  - `psql -v ON_ERROR_STOP=1 -f /app/seed.sql`（compose 注入 `PGHOST/PGUSER/PGPASSWORD/PGDATABASE`）
  - `wait "$BACKEND_PID"`（前台等后端进程）
- [x] 4.3 新建 `demo/docker-compose.demo.yml`：
  - `postgres`：image `pgvector/pgvector:pg18-trixie`，env `POSTGRES_DB/USER/PASSWORD`，挂载 `../docker/postgres/init`（compose 文件在 `demo/`，相对路径 `..` 指仓库根，enable pgvector），healthcheck，**不挂载 `./data`**（每次 fresh），网络 `syntopica-demo-net`
  - `syntopica-demo`：build `context: ..`（仓库根）dockerfile `Dockerfile.demo`，depends_on postgres healthy，ports `${PORT:-5000}:5000`，environment `DATABASE_DSN=host=postgres user=... password=... dbname=syntopica port=5432 sslmode=disable`、`SERVER_MODE=release`、`DEMO_READ_ONLY=1`、`PGHOST=postgres`、`PGUSER`、`PGPASSWORD`、`PGDATABASE`
  - **注意**：compose 文件位于 `demo/`，build context 与 init 卷必须用 `..` 指仓库根，否则 `Dockerfile.demo` 找不到
- [x] 4.4 新建 `demo/seed/README.md`：说明 seed.sql 生成方式、脱敏策略摘要、重跑命令

## 5. 生成 seed.sql

- [ ] 5.1 `cd backend-go && go run ./cmd/dump-sanitizer` 生成 `demo/seed/seed.sql`
- [x] 5.2 抽查 `demo/seed/seed.sql`：文件头注释有生成时间/窗口；grep 无 `api_key` 明文（应只见列名）、无 `INSERT INTO ai_call_logs`、无 `INSERT INTO schema_migrations`；向量列值为 `NULL`

## 6. 主文档（demo/README.md）

- [x] 6.1 新建 `demo/README.md`：用途（只读脱敏 demo）、启动命令 `docker compose -f demo/docker-compose.demo.yml up -d --build`、访问 `http://localhost:5000`、如何重新生成 seed、明确安全警告（勿用于生产、AI 凭证已清空、数据为脱敏快照）

## 7. 测试

- [x] 7.1 只读中间件单元测试 `backend-go/internal/platform/middleware/readonly_test.go`：
  - 非 demo 模式（`DEMO_READ_ONLY` unset）：POST 请求透传（调用 `c.Next()` 可被后续 handler 命中）
  - demo 模式 `=1`：OPTIONS 放行 204 / GET 放行 / POST 返回 405 + `{"error":"read-only demo"}` / GET `/api/topic-tags/merge-preview/scan/stream` 返回 405
  - 用 `gin.CreateTestContext` +httptest 构造请求，断言 status code 和 body
- [x] 7.2 `cd backend-go && go test ./internal/platform/middleware` → PASS

## 8. 文档

- [x] 8.1 `demo/README.md`（§6 已含）
- [x] 8.2 `docs/reference/deployment.md` 末尾追加 "公开只读 Demo" 小节，指向 `demo/README.md`（保持 reference 活文档完整）
- [ ] 8.3 本 change 不涉及 `docs/reference/architecture|api|database` 结构性变更，归档说明中注明

## 9. 验证（归档前重跑，每条必须实测零失败）

### 后端门禁（§4.1）

- [x] 9.1 `cd backend-go && go vet ./...` → 0 error
- [x] 9.2 `cd backend-go && golangci-lint run ./...` → 本 change 改动的包（`internal/platform/middleware`、`internal/app`、`cmd/...`）0 issue；全量失败项（`reader/handler/opml.go` errcheck、`models/semantic_label.go`/`platform/airouter/router.go` gofmt、`admin/handler/ai_handler.go` unused）均为既有代码债务、非本 change 引入，按 §3 Surgical Changes 不动，归档说明中注明
- [x] 9.3 `cd backend-go && go test ./internal/platform/middleware` → PASS
- [x] 9.4 `cd backend-go && go build ./...` → 成功（含 cmd/dump-sanitizer、cmd/server）

### 脱敏安全性

- [x] 9.5 `findstr /c:"api_key" demo\seed\seed.sql | findstr /v "api_key"` 校验：seed.sql 中 `api_key` 仅以列名形式出现在 INSERT 列清单，VALUES 侧无明文（人工目检 `INSERT INTO ai_providers (...,api_key,...) VALUES (...,'',...)`）
- [x] 9.6 `findstr "INSERT INTO ai_call_logs" demo\seed\seed.sql` → 零命中
- [x] 9.7 `findstr "INSERT INTO schema_migrations" demo\seed\seed.sql` → 零命中
- [x] 9.8 目检 seed.sql 含 `topic_tags` 两批 INSERT（`WHERE merged_into_id IS NULL` 与 `IS NOT NULL` 对应数据，可通过行数日志确认分批）
- [x] 9.8a 目检 seed.sql `INSERT INTO ai_settings (...) VALUES` 的 `value` 列均为 `'{}'`（design D6a 配置 JSON 清空）
- [x] 9.8b 抽查 `articles` VALUES 行无 `api_key`/`API_KEY`/`api-key` 字面量（已替换为 `[redacted-token]`，design D6a token 逃逸防护）
- [x] 9.8c 目检 seed.sql `INSERT INTO feeds` 末尾含 `ON CONFLICT (url) DO NOTHING`（design D6a 唯一键碰撞防护）
- [x] 9.8d RSSHub host 抽查：`findstr "rsshub.app" demo\seed\seed.sql` → 零命中（自建 host 已全部替换为 `rsshub.app`，design D6a 防基础设施泄露）
- [x] 9.8e UTF-8 完整性：`uv run python -c "open(r'demo\\seed\\seed.sql',encoding='utf-8').read()"` 无 UnicodeDecodeError（truncateContent 按 rune 截断，design D6a）

### demo 端到端

- [x] 9.9 `docker compose -f demo/docker-compose.demo.yml up -d --build` → 两容器 healthy
- [x] 9.9a 命中确认：`docker compose -f demo/docker-compose.demo.yml ps` 端口绑定到 demo 容器；本地已有 5000 服务时先关或用 `PORT=其他`（hard-lessons 硬骨头 6，避免 curl 命中本地服务误判）
- [x] 9.9b entrypoint.sh 行尾 LF（CRLF 会让 alpine `/bin/sh` 报 `not found`）；`docker compose ... logs syntopica-demo` 含 `[demo] clearing bootstrap/default data` + `[demo] seed import complete`（design D5a TRUNCATE 清场生效）
- [x] 9.10 `curl -sf http://127.0.0.1:5000/health` → `{"status":"healthy","database":"connected"}`（用 `127.0.0.1` 减少解析差异）
- [x] 9.11 `curl -sf http://127.0.0.1:5000/api/categories` → 返回 JSON 数组（seed 数据）
- [x] 9.12 `curl -sf http://127.0.0.1:5000/api/semantic-boards` → 返回板块列表
- [x] 9.13 只读验证：`curl -X POST http://127.0.0.1:5000/api/categories -H "Content-Type: application/json" -d "{}"` → HTTP 405 + `{"error":"read-only demo"}`
- [x] 9.14 只读验证：`curl -X POST http://127.0.0.1:5000/api/daily-reports/generate` → HTTP 405
- [x] 9.14a `/ws` 未注册验证：`curl -s http://127.0.0.1:5000/ws` → 404（demo 模式 WS 不注册，前端静默降级）
- [x] 9.15 浏览器访问 `http://127.0.0.1:5000/` → 首页有文章列表；`/topics` 有图谱；`/tags` 有板块且能进侦探墙；`/settings` 各 section 不崩
- [x] 9.16 幂等性：`docker compose -f demo/docker-compose.demo.yml down && docker compose -f demo/docker-compose.demo.yml up -d --build` → 数据一致，页面正常

### 前端零改动确认

- [ ] 9.17 `git diff --stat front/` → 本 change 未改动前端（design D2 同源 `/api`）。注：仓库 `front/` 当前存在与本 change 无关的 dirty changes（见 hard-lessons 尾巴），归档时需确认这些 diff 非本 change 引入

### 生产零影响确认

- [x] 9.18 `cd backend-go && DEMO_READ_ONLY= go test ./internal/platform/middleware` → PASS（非 demo 模式透传验证）
