# 配置指南

Syntopica 使用分层配置系统：后端 YAML 配置文件、覆盖文件值的环境变量，以及 Nuxt 运行时配置。AI 相关设置（LLM、Firecrawl、Digest）存储在数据库中，通过 Web UI 配置。

## 环境变量

### 后端（Go）

以下环境变量会覆盖 `backend-go/configs/config.yaml` 中的值。未设置时使用配置文件默认值。

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `SERVER_PORT` | 否 | `"5000"` | 后端 HTTP 监听端口 |
| `SERVER_MODE` | 否 | `"debug"` | Gin 模式：`"debug"`、`"release"` 或 `"test"` |
| `DATABASE_DRIVER` | 否 | `"postgres"` | 数据库驱动，主分支仅支持 `"postgres"` |
| `DATABASE_DSN` | 否 | `"host=127.0.0.1 user=postgres password=postgres dbname=syntopica port=5432 sslmode=disable TimeZone=Asia/Shanghai"` | PostgreSQL 连接字符串 |
| `CORS_ORIGINS` | 否 | `"http://localhost:3000,http://localhost:3000"` | 逗号分隔的允许 CORS 来源列表 |

### 前端（Nuxt）

通过 `nuxt.config.ts` 的 `runtimeConfig` 设置，可用环境变量覆盖。

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `API_INTERNAL_BASE` | 否 | `"http://localhost:5000/api"` | 服务端 API 基础 URL（SSR 时使用） |
| `NUXT_PUBLIC_API_ORIGIN` | 否 | `"http://localhost:5000"` | 暴露给浏览器的公共 API 源 |
| `NUXT_PUBLIC_API_BASE` | 否 | `"http://localhost:5000/api"` | 暴露给浏览器的公共 API 基础 URL |

### Docker Compose

以下变量由 Docker Compose 文件使用，Docker 外部无效。

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `FRONT_PORT` | 否 | `"3000"` | 前端容器映射到宿主机的端口 |
| `BACKEND_PORT` | 否 | `"5000"` | 后端容器映射到宿主机的端口 |
| `POSTGRES_DB` | 否 | `"syntopica"` | PostgreSQL 数据库名 |
| `POSTGRES_USER` | 否 | `"postgres"` | PostgreSQL 用户名 |
| `POSTGRES_PASSWORD` | 否 | `"postgres"` | PostgreSQL 密码 |
| `POSTGRES_PORT` | 否 | `"5432"` | PostgreSQL 容器映射到宿主机的端口 |
| `TZ` | 否 | `"Asia/Shanghai"` | PostgreSQL 容器时区 |
| `GOPROXY` | 否 | *(空)* | 后端构建时的 Go 模块代理 |
| `GOSUMDB` | 否 | *(空)* | 后端构建时的 Go 校验数据库 |
| `HTTP_PROXY` / `HTTPS_PROXY` / `NO_PROXY` | 否 | *(空)* | 代理设置，传递到构建上下文 |

### Docker Compose（Firecrawl）

以下变量由 `docker-compose.firecrawl.yml` 使用，仅在启动 Firecrawl 服务时有效。

| 变量 | 必填 | 默认值 | 说明 |
|---|---|---|---|
| `FIRECRAWL_PORT` | 否 | `"3002"` | Firecrawl API 映射到宿主机的端口 |
| `FIRECRAWL_REDIS_PORT` | 否 | `"6380"` | Firecrawl Redis 映射到宿主机的端口（避免与本地 Redis 冲突） |

> **注意**：Firecrawl 的 API URL 和 API Key 通过 Web UI 配置（存储在 `ai_settings` 表），而非环境变量。自部署时 API URL 默认为 `http://firecrawl:3002`（容器间通信）或 `http://localhost:3002`（宿主机访问）。

> **Firecrawl 现为可选兜底**：文章正文抓取默认走进程内 readability（纯 Go，零外部依赖）。仅当 readability 提取不到合格正文（SPA 站点）时才降级调用 Firecrawl。Firecrawl 服务可随时关闭而不影响 SSR 站点的正文抓取。

## 配置文件格式

后端从 `backend-go/configs/config.yaml` 读取 YAML 配置文件，通过 Viper 在启动时加载。默认配置即为 PostgreSQL 连接，即使没有配置文件也能正常工作。

> **注意：主分支仅支持 PostgreSQL 数据库驱动。SQLite 支持仅在 `sqlite` 分支可用。**

```yaml
server:
  port: "5000"
  mode: "debug"           # debug | release | test

database:
  driver: "postgres"
  dsn: "host=127.0.0.1 user=postgres password=postgres dbname=syntopica port=5432 sslmode=disable TimeZone=Asia/Shanghai"
  postgres:
    max_idle_conns: 5
    max_open_conns: 25
    conn_max_lifetime_minutes: 60
    conn_max_idle_time_minutes: 10
```

### 主要配置段

- **server** — 控制 HTTP 服务器端口和 Gin 运行模式。`release` 模式下 Gin 会抑制调试输出。
- **database** — 配置持久化层。`driver` 字段始终为 `"postgres"`。PostgreSQL 有独立的连接池调优参数。
- **cors** — 跨域请求的允许来源、HTTP 方法和请求头。来源是列表形式；通过 `CORS_ORIGINS` 环境变量覆盖时，解析为逗号分隔的字符串。

## 必填与可选设置

所有设置都有默认值。应用程序无需任何配置文件或环境变量即可启动，使用 PostgreSQL 的合理默认值。

没有环境变量缺失会导致启动失败。配置加载代码（`config.go` 中的 `applyEnvOverrides`）仅当环境值非空时才覆盖，否则使用 YAML 文件或代码默认值。

唯一会导致启动失败的场景是数据库 DSN 无效或不可达 — `main.go` 中的 `database.InitDB` 调用会 `log.Fatalf`。

## 默认值

### 后端默认值

| 设置 | 默认值 | 来源 |
|---|---|---|
| Server port | `"5000"` | `viper.SetDefault` in `config.go` |
| Server mode | `"debug"` | `viper.SetDefault` in `config.go` |
| Database driver | `"postgres"` | `viper.SetDefault` in `config.go` |
| Database DSN | `"host=127.0.0.1 user=postgres password=postgres dbname=syntopica port=5432 sslmode=disable TimeZone=Asia/Shanghai"` | `viper.SetDefault` in `config.go` |
| Postgres max idle conns | `5` | `viper.SetDefault` in `config.go` |
| Postgres max open conns | `25` | `viper.SetDefault` in `config.go` |
| Postgres conn max lifetime | `60` min | `viper.SetDefault` in `config.go` |
| Postgres conn max idle time | `10` min | `viper.SetDefault` in `config.go` |
| CORS origins | `localhost:3000`, `localhost:3000` | `viper.SetDefault` in `config.go` |
| CORS methods | `GET, POST, PUT, DELETE, OPTIONS` | `viper.SetDefault` in `config.go` |
| CORS headers | `Content-Type, Authorization` | `viper.SetDefault` in `config.go` |
| Tracing enabled | `true` | `tracing.DefaultConfig()` |
| Tracing retention | `7` days | `tracing.DefaultConfig()` |

### 前端默认值

| 设置 | 默认值 | 来源 |
|---|---|---|
| API internal base | `"http://localhost:5000/api"` | `nuxt.config.ts` |
| Public API origin | `"http://localhost:5000"` | `nuxt.config.ts` |
| Public API base | `"http://localhost:5000/api"` | `nuxt.config.ts` |

## 各环境覆盖

### 本地开发

本地开发时默认值开箱即用：

- 后端运行在 `http://localhost:5000`，使用 PostgreSQL 数据库。
- 前端开发服务器（`pnpm dev`）运行在 `http://localhost:3000`。
- 无需配置文件或 `.env` 文件。
- 需要本地运行 PostgreSQL + pgvector，可通过 Docker 启动：

```bash
docker run -d --name rss-postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=syntopica pgvector/pgvector:pg18-trixie
```

### Docker（PostgreSQL + pgvector）— 推荐方式

```bash
docker compose up -d
```

启动三个服务：

- **postgres**: PostgreSQL（pgvector:pg18-trixie）端口 5432，数据通过 `./data/` 目录持久化。
- **backend**: Go API 服务器端口 5000，内部连接 postgres 服务。
- **front**: Nuxt SSR 服务器内部端口 3000，通过 `${FRONT_PORT:-3000}` 映射到宿主机。内部通过 `http://backend:5000/api` 代理 API 请求。

启动后：
- 前端：`http://localhost:3000`
- 后端 API：`http://localhost:5000/api`

## 数据库存储的设置（AI 功能）

AI 相关配置不存储在文件或环境变量中 — 通过 Web UI 管理并持久化到 PostgreSQL 的 `ai_settings` 表。后端通过 `aisettings` 包在运行时读取。

| 配置键 | 说明 |
|---|---|
| `summary_config` | 文章摘要 LLM 凭证（base URL、API key、model） |
| `auto_summary_config` | 自动摘要调度器设置（时间范围、模型参数） |
| `firecrawl_config` | Firecrawl 集成设置（启用、API URL、API key、模式、超时、最大内容长度） |
| `open_notebook_config` | Open Notebook digest 导出设置（启用、base URL、API key、model、目标笔记本、prompt 模式、自动发送日报/周报） |
| `daily_report_time` | 日报生成时刻（HH:MM 格式，默认 `21:00`） |
| `persistent_topic_match_threshold` | 新 section 锚定已有话题的余弦距离阈值（默认 `0.30`） |
| `persistent_topic_upgrade_threshold` | candidate 允许人工确认所需、同时为管理 UI 可见门槛的连续命中天数（默认 `3`；不会自动转 active） |
| `persistent_topic_candidate_decay_window` | candidate 被注入聚类 prompt 的观察窗口天数（默认 `7`）；仅用于 prompt 卫生过滤，不触发状态变更 |
| `persistent_topic_candidate_prompt_limit` | 每次聚类注入与归属锚定集合的 candidate 数量上限（默认 `20`） |
| `persistent_topic_cluster_threshold` | 历史回刷 complete-link 聚类阈值（默认 `0.28`） |

这些设置通过 `aisettings.LoadSummaryConfig()`、`aisettings.LoadFirecrawlConfig()` 等函数加载，在前端设置页面中配置。

文章手动总结会在每次请求时重新读取 AI Provider 配置：优先使用 `summary` capability 的启用路由；未配置该路由时，回退到任一启用且具有 Base URL 和 Model 的 Provider。因此服务启动后新增 Provider 无需重启。API Key 对本地 Ollama、llama.cpp 或无需鉴权的 OpenAI-compatible 服务是可选项。

### Provider 思考开关（`enable_thinking`）

`ai_providers.enable_thinking` 字段控制是否让被调用的模型进行推理思考——后端在请求体中透传 `chat_template_kwargs.enable_thinking=true`（适用于 Qwen3.x / Qwythos 等带思考模板的 llama.cpp 模型）。**注意语义**：该字段只控制「模型是否思考」，服务器会把思考内容分离到 `reasoning_content` 字段，`content` 始终是干净答案。

**典型场景：同一台本地模型，打标签不思考、生成日报思考**。靠配两条 provider 指向同一服务实现差异化（无需改代码）：

1. 后台新建 provider `qwythos-nothink`：`base_url=http://127.0.0.1:8080`、`model=<模型名>`、`enable_thinking=false`。挂到 `topic_tagging` route（打标签走它，省 token、更快）。
2. 后台新建 provider `qwythos-think`：同样的 `base_url` 和 `model`、`enable_thinking=true`。挂到 `digest_polish` route（日报走它，思考后质量更高）。

> 历史语义提示：该字段曾表示「事后剥离 `<think>` 标签」，migration `20260626_0001` 已将所有 provider 的该字段重置为 `false` 以兜底语义反转。升级后请按需手动开启日报 provider 的思考。

### 数据增强（Data Enrichment）

数据增强功能（循环A新闻汇总 + 循环B三角色编排）依赖以下部署配置：

#### 1. `ai_routes` 表 seed 两条 Capability

| capability | 路由建议名 | 说明 |
|------------|-----------|------|
| `data_enrichment_news` | `data-enrichment-news` | 循环A新闻汇总（`summarize_context`），量大可配便宜模型 |
| `data_enrichment_analysis` | `data-enrichment-analysis` | 循环B分析认知（`interpret` / `tool_use` / `analyze` / `review_judge`） |

两条路由均需在 `ai_routes` 表 seed 为启用状态，并绑定到 `ai_route_providers` 中的至少一个 provider。

#### 2. Provider 需配 `enable_thinking=false`

根据设计决策（design.md §11 决策①），`data_enrichment_news` 和 `data_enrichment_analysis` 路由指向的 provider 必须设置 `enable_thinking=false`。Qwen3 等带思考模板的模型在 thinking 模式下会烧光 token 导致 `content` 为空——当前 agent loop 的 system prompt + 低 max_tokens 设计不兼容 thinking 模式。

> **注意**：此配置是 provider 级别的（`ai_providers` 表 `enable_thinking` 字段），不是 per-request 参数。domain 代码不做特殊处理，照常调用 `airouter.Router.Chat`。airouter 请求层始终透传 `chat_template_kwargs.enable_thinking = provider.EnableThinking`（参见 `openai_compatible.go:206` `buildPayload`）。

#### 3. 板块编辑需开 `enrichment_enabled=true`

循环B增强仅对 `enrichment_enabled=true` 的板块（SemanticLabel）允许触发。用户需先在板块详情页的「分析」tab 中：
1. 绑定至少一个数据源（`board_data_sources`）
2. 开启 `enrichment_enabled` 开关（默认 false）
3. （可选）调整 `window_days`（默认 14）和 `context_layers`（默认 `["week","month","year","all"]`）

管理员无需额外配置；以上操作均可通过 Web UI 完成。

