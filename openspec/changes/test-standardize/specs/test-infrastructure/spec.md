# Test Infrastructure

## Purpose

Provide a shared Go test utility package that gives integration tests access to a real Postgres database (with pgvector), replacing per-test SQLite setup. Define test tier conventions and file naming standards so developers can run fast unit tests or full integration tests with a single command.

## Requirements

### Requirement: testutil package with OpenTestDB

系统 SHALL 在 `backend-go/internal/platform/testutil/` 提供 `OpenTestDB(t *testing.T) *gorm.DB` 函数，连接到 Postgres 数据库实例（默认 `localhost:5432/syntopica`，可通过 `TEST_DB_DSN` 环境变量覆盖）。

#### Scenario: 连接成功时返回有效 *gorm.DB

- **WHEN** Postgres 实例运行在 `localhost:5432` 且 `OpenTestDB` 被调用
- **THEN** 返回一个连接到 `syntopica` 数据库的 `*gorm.DB` 实例，`db.Name()` 返回 `"postgres"`

#### Scenario: Postgres 不可用时测试立即失败并给出提示

- **WHEN** `localhost:5432` 无 Postgres 实例监听且未设置 `TEST_DB_DSN`
- **THEN** `t.Fatalf` 被调用，错误消息包含启动命令提示 `docker compose -f docker-compose.pg.yml up -d`

#### Scenario: TEST_DB_DSN 环境变量覆盖默认连接

- **WHEN** 环境变量 `TEST_DB_DSN` 设置为 `host=localhost port=5433 user=test password=test dbname=testdb sslmode=disable`
- **THEN** `OpenTestDB` 使用该 DSN 连接而非默认值

### Requirement: testutil package with SetupTestDB — single entry point

系统 SHALL 提供 `SetupTestDB(t *testing.T) *gorm.DB` 函数，作为唯一测试 DB 入口。该函数执行：连接 Postgres（调用 `OpenTestDB`）、运行 `AutoMigrate` 同步**所有** domain model 表结构（不按包筛选）、调用 `TruncateAllTables` 清空数据、设置 `database.DB = db`、返回 `*gorm.DB`。

#### Scenario: 返回干净且迁移完成的数据库

- **WHEN** `SetupTestDB` 被调用
- **THEN** 返回的 `*gorm.DB` 所有 domain model 表已创建（包括 `topic_tag_embeddings` 的 `embedding` vector 列），所有表数据为空

#### Scenario: database.DB 全局变量被设置

- **WHEN** `SetupTestDB` 被调用
- **THEN** `database.DB` 被设为同一 `*gorm.DB` 实例，被测代码通过 `database.DB` 读取时能正常工作

#### Scenario: 重复调用不报错

- **WHEN** `SetupTestDB` 在同一 Postgres 实例上被连续调用两次
- **THEN** 两次均成功返回，AutoMigrate 幂等，TruncateAllTables 清空数据

### Requirement: testutil package with TruncateAllTables

系统 SHALL 提供 `TruncateAllTables(t *testing.T, db *gorm.DB)` 函数，使用 `TRUNCATE TABLE ... CASCADE` 清空所有 domain 表数据，保证测试间隔离。

#### Scenario: 清空所有表数据

- **WHEN** 数据库中有 3 个表各含 10 行数据，`TruncateAllTables` 被调用
- **THEN** 所有 3 个表的行数为 0

#### Scenario: 外键关联表被 CASCADE 清空

- **WHEN** `topic_tags` 表有 1 行，`topic_tag_embeddings` 表通过外键关联且有 3 行
- **THEN** Truncate 后两张表均为空

### Requirement: 测试分层 — short 模式跳过集成测试

系统 SHALL 定义测试分层规范：带有数据库依赖的集成测试 MUST 检查 `testing.Short()` 并在 `-short` 模式下 `t.Skip("requires Postgres")`。

#### Scenario: go test -short 跳过 DB 测试

- **WHEN** 运行 `go test -short ./internal/tagmanagement/...`
- **THEN** 所有依赖 Postgres 的测试被跳过，无 Docker 依赖，纯单元测试正常运行

#### Scenario: go test 运行集成测试

- **WHEN** 运行 `go test ./internal/tagmanagement/...`（不带 `-short`）且 Docker Postgres 可用
- **THEN** 所有集成测试运行并通过，使用真实 Postgres + pgvector

### Requirement: 测试文件命名规范区分单元与集成

系统 SHALL 使用文件命名规范在同一 package 内区分单元测试和集成测试：

| 文件模式 | 含义 | 需要 DB | `testing.Short()` |
|---------|------|---------|-------------------|
| `xxx_test.go` | 集成测试 | 是 (Postgres) | 是 (跳过) |
| `xxx_unit_test.go` | 单元测试 | 否 | 否 |

真实 LLM 验证测试（如 `tag_context_dump_test.go`）保持现有 `testing.Short()` 保护模式不变。

#### Scenario: 单元测试文件无需 Docker 即可通过

- **WHEN** 运行 `go test -short ./internal/tagmanagement/service/core/...`
- **THEN** `embedding_unit_test.go` 中的 10 个测试全部运行并通过，无需 Docker Postgres

#### Scenario: 集成测试文件在 -short 下被跳过

- **WHEN** 运行 `go test -short ./internal/tagmanagement/service/core/...`
- **THEN** `embedding_test.go` 中的 3 个集成测试被 `testing.Short()` 跳过

#### Scenario: 混合文件被正确拆分

- **WHEN** 检查 `internal/tagmanagement/service/core/` 目录
- **THEN** `embedding_unit_test.go` 存在且只包含无 DB 依赖的测试函数
- **THEN** `embedding_test.go` 只包含需要 DB 的测试函数，每个函数开头有 `testing.Short()` 检查

### Requirement: 测试与源码同目录共存

测试文件 SHALL 与被测源码放在同一目录（Go 同 package 约定），不使用独立的 `tests/` 目录。

#### Scenario: 测试文件能访问 unexported 符号

- **WHEN** 源码文件定义了 unexported 函数（如 `evaluateSemanticBoardMatches`）
- **THEN** 同目录的 `*_test.go` 文件可以直接调用该函数，无需 export
