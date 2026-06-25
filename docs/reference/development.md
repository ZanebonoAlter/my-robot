# 开发指南

本地开发与构建命令参考。如果你是首次参与，请先阅读 [Getting Started](../getting-started.md) 完成环境搭建。

> **规范已迁移**：代码风格、目录约定、测试规范、提交前检查、Branch/PR 流程、编码注意事项的**权威定义**已迁至 [`standard/`](standard/README.md)（前后端分文件夹）。本文件只保留**环境搭建**与**构建命令**。

## 本地开发环境搭建

### 前置条件

- Go 1.25+
- Node.js + pnpm
- Docker（用于运行 PostgreSQL）
- PostgreSQL with pgvector（通过 Docker 启动）

### 启动顺序

1. **先启动 PostgreSQL**（使用 Docker）：

```bash
docker run -d --name rss-postgres -p 5432:5432 -e POSTGRES_PASSWORD=postgres -e POSTGRES_DB=syntopica pgvector/pgvector:pg18-trixie
```

或使用项目自带的 docker-compose：

```bash
docker compose up -d
```

> SQLite 版本已归档到 `sqlite` 分支，主分支不再支持。

2. **再启动后端** — 后端需要先初始化数据库和调度器：

```bash
cd backend-go
go mod tidy
go run cmd/server/main.go
```

后端默认运行在 `http://localhost:5000`，首次启动会自动连接 PostgreSQL 数据库并执行版本化迁移。

开发时日志现在按级别分流：常规运行日志和 warning 走 `stdout`，error / fatal / panic 走 `stderr`。如果你在 PowerShell、Docker 或 systemd 里单独收集错误输出，可以直接利用这条分流。

3. **再启动前端**（新终端）：

```bash
cd front
pnpm install
pnpm dev
```

前端开发服务器运行在 `http://localhost:3000`。

4. **验证联调** — 打开 `http://localhost:3000`，确认前端能连接到 `http://localhost:5000/api`。

### Docker Compose 启动

#### 仅启动 PostgreSQL 数据库（开发用）

```bash
docker compose up -d
```

这会启动一个 pgvector 容器，端口和数据目录可在 `.env` 中配置。

### 配置说明

本地开发无需任何配置文件或 `.env` 文件即可启动——后端和前端均有开箱即用的默认值。

后端配置文件位于 `backend-go/configs/config.yaml`，通过 Viper 加载，环境变量可覆盖文件值。详见 [Configuration](configuration.md)。

AI 相关设置（LLM、Firecrawl、Digest）通过 Web UI 的设置页面配置，存储在数据库中，无需手动编辑配置文件。

## 构建命令

### 前端命令（在 `front/` 目录执行）

| 命令 | 说明 |
|------|------|
| `pnpm install` | 安装依赖 |
| `pnpm dev` | 启动开发服务器（`http://localhost:3000`） |
| `pnpm build` | 生产构建 |
| `pnpm generate` | 静态站点生成 |
| `pnpm preview` | 预览生产构建 |
| `pnpm lint` | ESLint 代码检查 |
| `pnpm exec nuxi typecheck` | TypeScript 类型检查 |
| `pnpm test:unit` | 运行 Vitest 单元测试 |
| `pnpm test:e2e` | 运行 Playwright E2E 测试 |
| `pnpm test:e2e:ui` | Playwright 测试 UI 模式 |

运行单个单元测试文件：

```bash
pnpm test:unit -- app/utils/articleContentSource.test.ts
```

按测试名称过滤：

```bash
pnpm test:unit -- app/utils/articleContentSource.test.ts -t "prefers firecrawl"
```

### 后端命令（在 `backend-go/` 目录执行）

| 命令 | 说明 |
|------|------|
| `go mod tidy` | 整理 Go 模块依赖 |
| `go run cmd/server/main.go` | 启动后端服务 |
| `go build ./...` | 编译所有包 |
| `go test ./...` | 运行所有 Go 测试 |
| `go vet ./...` | Go 官方静态检查 |
| `golangci-lint run ./...` | 综合静态分析（staticcheck, revive, gosec 等） |

运行单个包的测试：

```bash
go test ./internal/reader/service -v
```

运行单个测试：

```bash
go test ./internal/reader/service -run TestBuildArticleFromEntryTracksOnlyRunnableStates -v
```

### Python 集成测试（在 `tests/workflow/` 目录执行）

```bash
uv venv
.venv\Scripts\activate    # Windows
uv pip install -r requirements.txt
pytest test_*.py -v
```

运行单个测试文件或测试用例：

```bash
pytest test_schedulers.py -v
pytest test_schedulers.py::TestAutoRefreshScheduler::test_name -v
```

带覆盖率报告：

```bash
pytest --cov=. --cov-report=html
```

> **注意**：Python 集成测试需要 Go 后端运行在 `localhost:5000`。

### Firecrawl 集成检查（在 `tests/firecrawl/` 目录执行）

先启动后端，然后运行：

```bash
python test_firecrawl_integration.py
```

---

## 规范文档导航

本文件原有的代码风格、目录约定、测试、提交前检查、Branch/PR、编码注意事项等内容已迁入 `standard/`，不再在此重复：

| 原内容 | 权威源 |
|--------|--------|
| 代码风格（前端 / 后端） | [`standard/frontend/code-style.md`](standard/frontend/code-style.md)、[`standard/backend/code-style.md`](standard/backend/code-style.md) |
| 目录结构约定 | [`standard/frontend/code-style.md`](standard/frontend/code-style.md) §2、[`standard/backend/package-layout.md`](standard/backend/package-layout.md) |
| 测试规范 | [`standard/frontend/testing.md`](standard/frontend/testing.md)、[`standard/backend/testing.md`](standard/backend/testing.md) |
| 提交前检查 / Branch / PR / 编码注意 | [`standard/shared/commit-pr.md`](standard/shared/commit-pr.md) |

其他参考：[架构总览](architecture/overview.md)、[详细设计地图](architecture/map.md)、[业务流程](flow/README.md)、[测试指南](testing.md)。
