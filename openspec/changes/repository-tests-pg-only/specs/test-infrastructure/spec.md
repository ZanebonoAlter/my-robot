## ADDED Requirements

### Requirement: 数据访问层（repository）测试禁用内存 SQLite

数据访问层（`backend-go` 下所有 `*/repository/` 包）的测试 SHALL 使用 `testutil.SetupTestDB`（隔离 pgvector testcontainer）作为数据库来源，SHALL NOT 使用内存 SQLite（`github.com/glebarez/sqlite` / `sqlite.Open`）模拟数据库行为来测试数据访问逻辑。

**理由**：SQLite 与 PostgreSQL 在 GROUP BY 语义、JSONB/vector 类型校验、约束执行上存在系统性差异——SQLite 对上述宽松放过，PostgreSQL 严格拒绝。数据访问层是直接发 SQL 的层，用 SQLite 测试会掩盖生产 SQL 的运行时错误（已记录至少 4 起同类事故：GROUP BY 缺列、vector 空串、JSONB 空串、迁移 NOT NULL）。把 `standard/backend/testing.md` 已有的事故硬约束从「改发往真实 DB 的 SQL 时才补 PG 用例」前置为「repository 包常驻禁用 SQLite」，从源头消除假绿温床。

**例外**：不依赖数据库的纯算法/纯函数测试（如距离计算、匈牙利分配、向量聚合）SHALL 归入 `*_unit_test.go`（见「测试文件命名规范区分单元与集成」requirement），不受本约束——因其本就不产生 SQLite 与 PostgreSQL 的语义漂移。

#### Scenario: repository 包测试文件不含 SQLite 依赖

- **WHEN** 扫描 `backend-go` 下所有 `*/repository/*_test.go` 文件
- **THEN** 无任何文件 import `github.com/glebarez/sqlite` 或调用 `sqlite.Open`
- **AND** 数据访问层测试统一通过 `testutil.SetupTestDB` 获取 `*gorm.DB`

#### Scenario: repository 数据访问逻辑走真 PG

- **WHEN** `*/repository/` 包内测试需要操作数据库（CRUD / 查询 / 级联 / 唯一约束去重）
- **THEN** 该测试 SHALL 调用 `testutil.SetupTestDB(t)` 获取连接到隔离 pgvector 容器的 `*gorm.DB`
- **AND** 不使用内存 SQLite 模拟 PostgreSQL 行为

#### Scenario: repository 包内纯函数测试豁免迁移

- **WHEN** `*/repository/` 包内存在不依赖数据库的纯算法/纯函数测试
- **THEN** 该类测试 SHALL 归入 `*_unit_test.go` 文件，不调用 `SetupTestDB`
- **AND** 本 requirement 不强制其迁移，因其不产生 SQLite 与 PostgreSQL 的语义漂移
