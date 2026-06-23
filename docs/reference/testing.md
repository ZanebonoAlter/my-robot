# 测试指南

本项目有两套测试套件：Go 后端和 Nuxt 前端。

## 测试框架与配置

| 套件 | 框架 | 位置 | 语言 |
|-------|-----------|----------|----------|
| 后端单元测试 | Go `testing` + `testify` | `backend-go/**/*_test.go` | Go |
| 前端单元测试 | Vitest + Vue Test Utils | `front/app/**/*.test.ts` | TypeScript |
| 前端 E2E 测试 | Playwright | `front/tests/e2e/*.spec.ts` | TypeScript |

### 后端（Go）

测试使用标准 `testing` 包和 `github.com/stretchr/testify` 断言。每个测试文件以 `*_test.go` 形式与源码放在一起。

- **单元测试**（`*_unit_test.go`）：纯逻辑，无数据库依赖，`go test -short` 下运行。
- **集成测试**（`*_test.go`）：通过 `testutil.SetupTestDB(t)` 连接到一个**由 testcontainers-go 启动的隔离 pgvector Postgres 容器**（镜像 `pgvector/pgvector:pg18-trixie`，与生产同构）。容器按测试进程启动一次、进程退出时由 Ryuk sidecar 自动销毁。

> ⚠️ **安全约定（事故教训）**：`testutil` 包**没有**默认 DSN，也**不读取**任何环境变量（包括历史上的 `TEST_DB_DSN`）。它只能启动隔离容器，无法连接到 `docker-compose.pg.yml` 跑的开发库（那正是生产数据所在）。早期版本曾通过默认 DSN 连到开发库并执行 `TRUNCATE`/`DROP TABLE`，导致业务数据被清空——该路径已彻底移除。**禁止**重新引入任何「默认数据库连接」或「env 覆盖」机制。
>
> 部分 package（`admin/handler`、`platform/airouter`、`reader/*`、`topicgraph/handler`）仍使用内存 SQLite（`glebarez/sqlite`），仅用于无 pgvector 依赖的 CRUD 测试。

### 前端单元（Vitest）

Vitest 使用 `happy-dom` 作为 DOM 环境。配置在 `front/vitest.config.ts`。测试文件以 `*.test.ts` 命名约定与源文件同目录放在 `front/app/` 下。

### 前端 E2E（Playwright）

Playwright 配置在 `front/playwright.config.ts`。它在 `http://localhost:3000` 启动 Nuxt 开发服务器，然后运行浏览器测试。测试串行执行（`fullyParallel: false`、`workers: 1`）以保证启动稳定性。

## 运行测试

### 后端

```bash
# 所有后端测试
cd backend-go
go test ./...

# 单个包（详细输出）
go test ./internal/topicgraph/repository -v

# 按名称运行单个测试
go test ./internal/topicgraph/repository -run TestListReports_DefaultDays -v

# 仅单元测试（跳过需要 Docker 的集成测试）
go test -short ./...
```

### 前端单元测试

```bash
cd front

# 运行所有单元测试
pnpm test:unit

# 单个测试文件
pnpm test:unit -- app/utils/articleContentSource.test.ts

# 按名称模式运行单个测试
pnpm test:unit -- app/utils/articleContentSource.test.ts -t "prefers firecrawl"
```

### 前端 E2E 测试

```bash
cd front

# 运行所有 E2E 测试（自动启动开发服务器）
pnpm test:e2e

# 使用 Playwright UI 运行
pnpm test:e2e:ui

# 列出测试但不运行
pnpm test:e2e:list

# 传递额外 Playwright 参数
pnpm test:e2e:args -- --grep "topic-graph"
```

## 编写新测试

### 后端

- 测试文件放在源码旁，作为同包的 `*_test.go`。
- 多个用例共享相同逻辑时使用表驱动测试。
- 需要数据库的集成测试，调用 `testutil.SetupTestDB(t)` 连接隔离 pgvector 容器（见下文「`testutil.SetupTestDB` 使用模式」）。无 pgvector 依赖的轻量 CRUD 测试可继续用内存 SQLite + `AutoMigrate`。
- 仅当文件已经使用 `testify` 时才导入 `github.com/stretchr/testify`；否则直接使用标准 `testing` 包。
- 在 setup 函数中使用 `t.Helper()` 获得更清晰的错误追踪。

### 前端单元测试

- 测试文件与源文件同目录：`front/app/<path>/file.test.ts`。
- 使用 Vitest 的 `describe`/`it` 块：
  ```typescript
  import { describe, expect, it } from 'vitest'
  import { myFunction } from './myFunction'

  describe('myFunction', () => {
    it('does the expected thing', () => {
      expect(myFunction('input')).toBe('expected')
    })
  })
  ```
- 测试在 `happy-dom` 环境中运行 — 不需要真实浏览器。
- `front/tests/e2e/` 中的 E2E 测试文件通过 `vitest.config.ts` 排除在 Vitest 之外。

### 前端 E2E 测试

- spec 文件放在 `front/tests/e2e/*.spec.ts`。
- 使用 Playwright 的 `test` 和 `expect` 导入：
  ```typescript
  import { test, expect } from '@playwright/test'

  test('page loads', async ({ page }) => {
    await page.goto('/some-page')
    await expect(page.locator('body')).toBeVisible()
  })
  ```
- 测试按顺序在 Chromium 上运行。开发服务器通过 `playwright.config.ts` 的 `webServer` 配置自动启动。

## 覆盖率

没有配置最低覆盖率阈值。

- **前端**：Vitest 在 `vitest.config.ts` 中没有覆盖率设置。运行 `pnpm test:unit` 只得到通过/失败结果。
- **后端**：Go 测试命令中没有 `cover` 配置文件或阈值标志。需要时可加 `-cover` 临时查看：`go test -cover ./...`。

## 后端测试分层约定

### 测试层级

| 层级 | 运行方式 | 需要 Docker | 说明 |
|------|----------|-------------|------|
| **单元测试** | `go test -short ./...` | 否 | 快速运行，跳过集成测试 |
| **集成测试** | `go test ./...` | 是（Docker） | testcontainers-go 自动启动隔离 pgvector 容器，无需手动配置 |

### 文件命名约定

| 文件后缀 | 类型 | 说明 |
|----------|------|------|
| `xxx_test.go` | 集成测试 | 默认需要数据库，使用 `testutil.SetupTestDB(t)` |
| `xxx_unit_test.go` | 单元测试 | 纯逻辑测试，无需外部依赖 |

### `testutil.SetupTestDB` 使用模式

集成测试的标准入口函数，负责：

1. **跳过单元模式**：运行 `-short` 时自动跳过
2. **启动隔离容器**：通过 testcontainers-go 启动一次性 pgvector 容器（进程级单例，首次调用启动，后续复用）
3. **重建测试 schema**：每次测试删除并重建隔离容器中的 `public` schema，清除上一次测试的结构和数据
4. **导入生产状态**：执行生产的 `RunAutoMigrate` 与 `RunMigrations`，重新导入生产版本迁移中的默认数据
5. **设置全局 DB**：兼容生产代码的 `database.DB`

需要在同一测试中恢复首次启动状态时，调用 `testutil.ReimportTestDB(t, db)`。该函数只操作 testcontainers-go 创建的临时数据库，不读取开发或生产 DSN。

**使用示例：**

```go
func TestSomethingIntegration(t *testing.T) {
    db := testutil.SetupTestDB(t)
    // db 已连接，并恢复为生产首次启动后的 schema 与默认数据
    // ... 测试逻辑
}
```

**单元测试示例：**

```go
// xxx_unit_test.go
func TestSomethingUnit(t *testing.T) {
    // 纯逻辑测试，不调用 SetupTestDB
    // ... 测试逻辑
}
```

### 集成测试常见陷阱

#### ⚠️ vector 列 seed 必须填合法值，不能依赖零值

部分 vector 列的 Go 字段是**非指针 `string`**（零值 `""`），pgvector 会拒绝空串：

| 模型 | 字段 | 类型 | 零值行为 |
|------|------|------|----------|
| `models.SemanticLabel` | `Embedding` / `MergeEmbedding` | `*string` | nil → NULL ✅ |
| `repository.DailyReportSection` | `Embedding` | `string` | `""` → **报错** ❌ |

**症状**：`invalid input syntax for type vector: "" (SQLSTATE 22P02)`

**修复**：seed 时用 `repository.FloatsToPgVector([]float64{0})` 填合法值（镜像生产——生产在 insert 前也会用 `FloatsToPgVector` 填充）：

```go
section := repository.DailyReportSection{
    ReportID:     reportID,
    ClusterLabel: label,
    Embedding:    repository.FloatsToPgVector([]float64{0}),
}
```

#### ⚠️ `ReimportTestDB` 内置重连，不可移除

`DROP SCHEMA public CASCADE` 会**连坐销毁** pgvector 的 `vector` 类型，`CREATE EXTENSION IF NOT EXISTS` 重建时拿到**新 OID**（实测每周期递增：16387→17280→18173）。连接池里残留的服务端 prepared statement 仍引用旧 OID，导致后续 vector 查询失败。

**症状**（任一）：
- `cache lookup failed for type <oid> (SQLSTATE XX000)`
- `cached plan must not change result type (SQLSTATE 0A000)`

**关键认知**：catalog 本身没坏（`pg_depend` 正常、扩展正确重注册），坏的是**连接的 prepared statement 缓存**。所以修复不是动 schema/扩展，而是 `ReimportTestDB` 在重建后**关闭旧池、开新池**（已实现）。新连接的 cache 与新 catalog 一致，等价于生产重启。

> 🛑 **切勿**为「优化」删掉 `ReimportTestDB` 末尾的 `openGorm` + `Close`——那是修这个坑的核心，删掉会复现 `cache lookup failed`。回归测试 `TestReimportPreservesVectorInserts` 会守住这行。

## 测试文件结构

> 清单随代码演进，以 `find backend-go/internal -name '*_test.go'` 和 `find front/app -name '*.test.ts'` 的实际结果为准。

```
backend-go/
├── internal/admin/
│   ├── handler/ai_handler_test.go
│   └── scheduler/base_test.go
├── internal/models/
│   ├── semantic_label_test.go
│   └── topic_graph_test.go
├── internal/platform/
│   ├── airouter/openai_compatible_test.go
│   ├── airouter/router_test.go
│   ├── airouter/store_test.go
│   ├── aisettings/config_store_test.go
│   ├── config/config_test.go
│   ├── database/db_test.go
│   ├── database/db_unit_test.go
│   ├── database/slow_logger_test.go
│   ├── jsonutil/sanitize_test.go
│   ├── logging/logging_test.go
│   ├── testutil/diag_test.go
│   └── ws/hub_test.go
├── internal/reader/
│   ├── handler/article_handler_test.go
│   ├── handler/content_completion_handler_test.go
│   ├── repository/firecrawl_job_queue_test.go
│   ├── service/content_completion_service_test.go
│   ├── service/feed_service_test.go
│   └── service/feed_service_unit_test.go
├── internal/tagmanagement/
│   ├── handler/merge_reembedding_queue_test.go
│   ├── handler/semantic_board_handler_test.go
│   ├── handler/semantic_board_match_detail_test.go
│   ├── repository/tag_job_queue_test.go
│   ├── service/auxlabel/auxiliary_label_service_test.go
│   ├── service/board/semantic_board_backfill_test.go
│   ├── service/board/semantic_board_matching_test.go
│   ├── service/board/semantic_board_matching_unit_test.go
│   ├── service/board/semantic_board_upgrade_test.go
│   ├── service/board/tag_clustering_test.go
│   ├── service/board/tag_clustering_unit_test.go
│   ├── service/core/article_tagger_test.go
│   ├── service/core/embedding_test.go
│   ├── service/core/embedding_unit_test.go
│   ├── service/core/extractor_test.go
│   ├── service/core/hard_merge_test.go
│   ├── service/core/helpers_test.go
│   ├── service/core/merge_tags_reembedding_test.go
│   ├── service/core/metadata_test.go
│   ├── service/core/metadata_unit_test.go
│   ├── service/core/quality_score_test.go
│   ├── service/core/quality_score_unit_test.go
│   ├── service/core/tag_cache_test.go
│   ├── service/core/tag_queue_test.go
│   └── service/merge/tag_merge_suggest_test.go
├── internal/topicgraph/
│   ├── handler/daily_report_handler_test.go
│   ├── handler/graph_handler_test.go
│   ├── repository/daily_report_matching_test.go
│   ├── repository/daily_report_repository_test.go
│   ├── service/daily_report_cluster_test.go
│   └── service/daily_report_dedup_test.go

front/
├── app/api/scheduler.test.ts
├── app/features/articles/components/ArticleTagList.test.ts
├── app/features/tags/components/SectionLifecyclePanel.test.ts
├── app/stores/api.test.ts
├── app/utils/api.test.ts
├── app/utils/articleContentGuards.test.ts
├── app/utils/articleContentSource.test.ts
├── app/utils/schedulerMeta.test.ts
└── tests/e2e/
    ├── baseline.spec.ts
    └── topic-graph.spec.ts
```
