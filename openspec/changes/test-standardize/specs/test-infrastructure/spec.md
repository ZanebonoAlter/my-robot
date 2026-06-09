# Test Infrastructure

## Purpose

Provide a shared Go test utility package that gives integration tests access to a real Postgres database (with pgvector), replacing per-test SQLite setup. Define test tier conventions so developers can run fast unit tests or full integration tests with a single command.

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

### Requirement: testutil package with SetupTestDB

系统 SHALL 提供 `SetupTestDB(t *testing.T) *gorm.DB` 函数，执行：连接 Postgres（调用 `OpenTestDB`）、运行 `RunAutoMigrate` 同步所有 model 表结构、调用 `TruncateAllTables` 清空数据。

#### Scenario: 返回干净且迁移完成的数据库

- **WHEN** `SetupTestDB` 被调用
- **THEN** 返回的 `*gorm.DB` 所有 domain model 表已创建（包括 `topic_tag_embeddings` 的 `embedding` vector 列），所有表数据为空

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

系统 SHALL 定义测试分层规范：带有数据库依赖的集成测试 MUST 检查 `testing.Short()` 并在 `-short` 模式下 `t.Skip`。

#### Scenario: go test -short 跳过 DB 测试

- **WHEN** 运行 `go test -short ./internal/domain/tagging/...`
- **THEN** 所有依赖 Postgres 的测试被跳过，无 Docker 依赖，测试在 5 秒内完成

#### Scenario: go test 运行集成测试

- **WHEN** 运行 `go test ./internal/domain/tagging/...`（不带 `-short`）且 Docker Postgres 可用
- **THEN** 所有集成测试运行并通过，使用真实 Postgres + pgvector

### Requirement: 测试不直接修改 database.DB 全局变量

使用 testutil 的测试 SHALL NOT 直接设置 `database.DB` 全局变量。如果被测代码引用 `database.DB`，通过显式传递 `*gorm.DB` 或依赖注入方式提供。

#### Scenario: 迁移后的测试文件不引用 database.DB 赋值

- **WHEN** 检查已迁移的 5 个 tagging 测试文件
- **THEN** 不存在 `database.DB =` 的直接赋值语句
