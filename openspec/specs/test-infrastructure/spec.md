# Test Infrastructure

## Purpose

Provide a shared Go test utility package that gives integration tests access to a real Postgres database (with pgvector), replacing per-test SQLite setup. Define test tier conventions and file naming standards so developers can run fast unit tests or full integration tests with a single command.

## Requirements

### Requirement: testutil package with OpenTestDB — isolated testcontainer

系统 SHALL 在 `backend-go/internal/platform/testutil/` 提供 `OpenTestDB(t *testing.T) *gorm.DB` 函数，通过 testcontainers-go 启动一个隔离的 pgvector Postgres 容器（镜像 `pgvector/pgvector:pg18-trixie`）并返回连接到该容器的 `*gorm.DB`。该函数**不得**读取任何环境变量、**不得**存在默认 DSN、**不得**连接到 `docker-compose.pg.yml` 运行的数据库（那是生产数据所在）。容器由 Testcontainers Ryuk sidecar 在测试进程退出时自动销毁。

#### Scenario: 启动隔离容器并返回有效连接

- **WHEN** Docker daemon 可用且 `OpenTestDB` 被调用
- **THEN** 启动一个隔离的 pgvector 容器，返回连接到该容器的 `*gorm.DB`，该容器与开发库（`localhost:5432/syntopica`）完全隔离

#### Scenario: Docker 不可用时测试立即失败

- **WHEN** Docker daemon 未运行且 `OpenTestDB` 被调用
- **THEN** `t.Fatalf` 被调用，错误消息提示需要 Docker

#### Scenario: 无任何环境变量可重定向到生产库

- **WHEN** 设置任何环境变量（包括 `TEST_DB_DSN`）
- **THEN** `OpenTestDB` 忽略它，仍然启动隔离容器——这是防止测试连到生产库的安全保证

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

### Requirement: 测试数据库使用生产初始化逻辑

系统 SHALL 使用生产的 `database.RunAutoMigrate` 和 `database.RunMigrations` 初始化隔离的测试数据库。生产迁移 SHALL 是测试 schema、约束和系统默认数据的唯一来源。

#### Scenario: 测试数据库首次启动

- **WHEN** 集成测试首次调用 `testutil.SetupTestDB`
- **THEN** 系统在 testcontainers-go 创建的临时 Postgres 中执行生产 AutoMigrate
- **AND** 执行生产版本迁移
- **AND** 导入生产版本迁移定义的系统默认数据

#### Scenario: 回归测试重新导入

- **WHEN** 回归测试调用 `testutil.ReimportTestDB`
- **THEN** 系统重建临时数据库的 `public` schema
- **AND** 重新执行生产 AutoMigrate 和生产版本迁移
- **AND** 数据库恢复为生产首次启动后的 schema 与默认数据状态
- **AND** 重新导入前的测试数据不残留

#### Scenario: 测试初始化保持物理隔离

- **WHEN** 测试初始化或重新导入数据库
- **THEN** 所有操作只作用于 testcontainers-go 创建的临时 Postgres
- **AND** 系统不读取可将测试连接重定向到开发或生产数据库的 DSN

#### Scenario: 不维护生产数据副本

- **WHEN** 生产 schema、约束或系统默认数据发生变化
- **THEN** 测试通过执行生产迁移获得变化
- **AND** testutil 中不存在对应的测试专用 DDL、seed 列表或默认值副本

#### Scenario: 变更范围受限

- **WHEN** 实施本 change
- **THEN** 只修改测试数据库初始化、重新导入入口及对应文档
- **AND** 不修改 tagmanagement 业务逻辑、业务测试断言、生产迁移或生产默认数据

### Requirement: 进程内单例连接与单次迁移

系统 SHALL 在 testutil 包内通过 `sync.Once` 实现：(1) pgvector 容器与 `*gorm.DB` 连接的进程内单例——首次 `OpenTestDB` 启动容器并执行 `gorm.Open` 建立连接，缓存于包级 `sync.Once`（`startOnce`），后续调用复用同一实例，不重复启动容器或建池；(2) `AutoMigrate` 的进程内单次执行——无论 `SetupTestDB` 被调用多少次，全量 model 迁移只发生一次（`migrateOnce`）。

#### Scenario: 重复调用 SetupTestDB 只启动一次容器

- **WHEN** 同一测试进程内 `SetupTestDB` 被多次调用
- **THEN** 底层 testcontainer 容器与 `gorm.Open` 均仅执行一次，所有调用复用同一容器与连接池

#### Scenario: 重复调用 SetupTestDB 只迁移一次

- **WHEN** 同一测试进程内 `SetupTestDB` 被多次调用
- **THEN** `AutoMigrate` 仅在首次执行；后续调用仅执行 `TruncateAllTables`

#### Scenario: 首次连接或迁移失败不被吞掉

- **WHEN** 首次 `gorm.Open` 或 `AutoMigrate` 失败
- **THEN** 错误被记录到包级变量，后续每次 `SetupTestDB` 调用通过 `t.Fatal` 报出该错误，不静默返回无效 `*gorm.DB`

### Requirement: testutil package with TruncateAllTables

系统 SHALL 提供 `TruncateAllTables(t *testing.T, db *gorm.DB)` 函数，使用 `TRUNCATE TABLE ... CASCADE` 清空所有 domain 表数据，保证测试间隔离。

#### Scenario: 清空所有表数据

- **WHEN** 数据库中有 3 个表各含 10 行数据，`TruncateAllTables` 被调用
- **THEN** 所有 3 个表的行数为 0

#### Scenario: 外键关联表被 CASCADE 清空

- **WHEN** `topic_tags` 表有 1 行，`topic_tag_embeddings` 表通过外键关联且有 3 行
- **THEN** Truncate 后两张表均为空

### Requirement: 测试分层 — short 模式跳过集成测试

系统 SHALL 将 `testing.Short()` 跳过逻辑内置在 `SetupTestDB` 中：函数首行执行 `if testing.Short() { t.Skip("requires Postgres") }`。集成测试函数本身无需（也不应）重复编写该守卫——调用 `SetupTestDB` 即自动获得分层保护。

依赖真实 LLM 等其他外部资源但不走 `SetupTestDB` 的测试（如 `tag_context_dump_test.go`）SHALL 自行维护 `testing.Short()` 守卫。单元测试（`*_unit_test.go`）不调用 `SetupTestDB`，在 `-short` 下自然运行。

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
