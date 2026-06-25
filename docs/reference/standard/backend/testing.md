# 后端测试（testing + testcontainer）

> **权威源**：本文件是后端测试约定的唯一权威。运行门禁见《开发执行规范》§4.1、集成测试执行纪律见 §6。
> 含 🛑 **DSN 安全红线（事故教训）**，必须遵守。

## 框架

- 标准 `testing` 包 + `github.com/stretchr/testify` 断言（仅当文件已用 testify 时才 import，否则用标准 testing）
- 测试文件以 `*_test.go` 与源码放在同一包中
- 偏好表驱动测试（table-driven）
- setup 函数中使用 `t.Helper()` 获得更清晰错误追踪

## 测试分层

| 层级 | 文件后缀 | 运行方式 | 需要 Docker | 说明 |
|------|----------|----------|-------------|------|
| 单元测试 | `xxx_unit_test.go` | `go test -short ./...` | 否 | 纯逻辑，无外部依赖 |
| 集成测试 | `xxx_test.go` | `go test ./...` | 是 | testcontainers-go 启动隔离 pgvector 容器 |

轻量 CRUD 测试（无 pgvector 依赖）可继续用内存 SQLite（`glebarez/sqlite` + `mode=memory`），参考 `internal/reader/service/feed_service_test.go` 的 `setupFeedsTestDB` 模式。`admin/handler`、`platform/airouter`、`reader/*`、`topicgraph/handler` 等包仍用内存 SQLite。

## `testutil.SetupTestDB` 使用模式

集成测试标准入口函数，负责：

1. **跳过单元模式**：`-short` 时自动 `t.Skip`
2. **启动隔离容器**：testcontainers-go 启动一次性 pgvector 容器（镜像 `pgvector/pgvector:pg18-trixie`，与生产同构；进程级单例，首次调用启动，后续复用，进程退出由 Ryuk sidecar 自动销毁）
3. **重建 schema**：每次删除并重建隔离容器的 `public` schema
4. **导入生产状态**：执行生产 `RunAutoMigrate` 与 `RunMigrations`，重新导入默认数据
5. **设置全局 DB**：兼容生产代码的 `database.DB`

```go
// xxx_test.go（集成）
func TestSomethingIntegration(t *testing.T) {
    db := testutil.SetupTestDB(t)
    // db 已连接，恢复为生产首次启动后的 schema 与默认数据
    // ... 测试逻辑
}

// xxx_unit_test.go（单元）
func TestSomethingUnit(t *testing.T) {
    // 纯逻辑，不调用 SetupTestDB
}
```

恢复首次启动状态用 `testutil.ReimportTestDB(t, db)`（只操作临时容器，不读开发/生产 DSN）。

## 🛑 DSN 安全红线（事故教训 — 不可违反）

> 早期版本曾通过默认 DSN 连到开发库并执行 `TRUNCATE`/`DROP TABLE`，**清空了业务数据**。该路径已彻底移除。

**硬约束：**

- `testutil` 包**没有**默认 DSN，**不读取**任何环境变量（包括历史上的 `TEST_DB_DSN`）
- 它**只能**启动隔离容器，**无法**连接 `docker-compose.pg.yml` 跑的开发库（那是生产数据所在）
- **禁止**重新引入任何「默认数据库连接」或「env 覆盖」机制

## 集成测试常见陷阱

### ⚠️ vector 列 seed 必须填合法值，不能依赖零值

部分 vector 列的 Go 字段是**非指针 `string`**（零值 `""`），pgvector 会拒绝空串：

| 模型 | 字段 | 类型 | 零值行为 |
|------|------|------|----------|
| `models.SemanticLabel` | `Embedding` / `MergeEmbedding` | `*string` | nil → NULL ✅ |
| `repository.DailyReportSection` | `Embedding` | `string` | `""` → **报错** ❌ |

**症状**：`invalid input syntax for type vector: "" (SQLSTATE 22P02)`
**修复**：seed 时用 `repository.FloatsToPgVector([]float64{0})` 填合法值（镜像生产 insert 前的填充）。

```go
section := repository.DailyReportSection{
    ReportID:     reportID,
    ClusterLabel: label,
    Embedding:    repository.FloatsToPgVector([]float64{0}),
}
```

### ⚠️ `ReimportTestDB` 内置重连，不可移除

`DROP SCHEMA public CASCADE` 会连坐销毁 pgvector 的 `vector` 类型，`CREATE EXTENSION IF NOT EXISTS` 重建时拿到**新 OID**（实测每周期递增：16387→17280→18173）。连接池里残留的 prepared statement 仍引用旧 OID，导致后续 vector 查询失败。

**症状（任一）：**
- `cache lookup failed for type <oid> (SQLSTATE XX000)`
- `cached plan must not change result type (SQLSTATE 0A000)`

**关键认知**：catalog 本身没坏（`pg_depend` 正常、扩展正确重注册），坏的是**连接的 prepared statement 缓存**。修复不是动 schema/扩展，而是 `ReimportTestDB` 重建后**关闭旧池、开新池**（已实现）。

> 🛑 **切勿**为「优化」删掉 `ReimportTestDB` 末尾的 `openGorm` + `Close`——那是修这个坑的核心，删掉会复现 `cache lookup failed`。回归测试 `TestReimportPreservesVectorInserts` 守住这行。

## 运行

```bash
cd backend-go
go test ./...                  # 含集成测试（需 Docker）
go test -short ./...           # 仅单元测试
go test ./internal/reader/service -v                         # 单个包
go test ./internal/topicgraph/repository -run TestName -v    # 单个测试
```

> AGENTS.md 约定：日常验证**只跑本次修改影响的包**，不跑全量。

## 资料来源

收敛自原 `testing.md`（§后端 / §后端测试分层约定 / §集成测试常见陷阱）与《开发执行规范》§4.2、§6。
