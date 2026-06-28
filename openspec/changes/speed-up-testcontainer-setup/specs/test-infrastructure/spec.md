## MODIFIED Requirements

### Requirement: testutil package with SetupTestDB — single entry point

系统 SHALL 提供 `SetupTestDB(t *testing.T) *gorm.DB` 函数，作为唯一测试 DB 入口。该函数的行为分两阶段：

1. **首次调用（进程内）**：建立"黄金 schema"——调用 `OpenTestDB` 启动/复用单例 pgvector 容器，执行一次 `runTestMigrations`（生产 `RunAutoMigrate` + `RunMigrations`，含迁移注入的 seed 数据），结果缓存为进程级黄金 schema。
2. **后续调用（进程内）**：**不再重建 schema**，改为调用 `ResetTestData`（seed 保护的 truncate）清空业务数据、保留迁移 seed，使每次调用后数据库状态等价于"生产首次启动后的状态"。

每次调用 SHALL 设置 `database.DB = db` 并返回 `*gorm.DB`。函数首行 SHALL 执行 `testing.Short()` 跳过守卫（见"测试分层"requirement）。

#### Scenario: 首次调用建立黄金 schema（含 seed）

- **WHEN** 进程内首次调用 `SetupTestDB`
- **THEN** 启动/复用单例 pgvector 容器，执行一次生产 AutoMigrate 与版本迁移（含 seed）
- **AND** 返回的 `*gorm.DB` 所有 domain model 表已创建（含 `topic_tag_embeddings.embedding` vector 列），迁移 seed 数据（`ai_settings`、`embedding_config`）已就位

#### Scenario: database.DB 全局变量被设置

- **WHEN** `SetupTestDB` 被调用（首次或后续）
- **THEN** `database.DB` 被设为同一 `*gorm.DB` 实例，被测代码通过 `database.DB` 读取时能正常工作

#### Scenario: 后续调用走轻量重置而非重建 schema

- **WHEN** 同一进程内 `SetupTestDB` 被第 N 次（N>1）调用
- **THEN** 不执行 `DROP SCHEMA` / `CREATE SCHEMA` / `runTestMigrations`
- **AND** 仅执行 `ResetTestData`（truncate 业务表，保留 seed），返回连接到同一黄金 schema 的 `*gorm.DB`

#### Scenario: 后续调用后状态等价于生产首次启动

- **WHEN** `SetupTestDB` 后续调用完成
- **THEN** 业务表数据为空，迁移 seed 数据（`ai_settings`、`embedding_config`）从快照恢复为 seed-only 状态
- **AND** 数据库状态等价于生产首次启动后的 schema + 默认数据

### Requirement: 测试数据库使用生产初始化逻辑

系统 SHALL 使用生产的 `database.RunAutoMigrate` 和 `database.RunMigrations` 初始化隔离的测试数据库。生产迁移 SHALL 是测试 schema、约束和系统默认数据的唯一来源。黄金 schema 在进程内只构建一次；需要"干净迁移态"的少数回归测试 SHALL 显式调用 `ReimportTestDB` 逃生口。

#### Scenario: 黄金 schema 首次构建

- **WHEN** 进程内首次调用 `testutil.SetupTestDB`
- **THEN** 系统在 testcontainers-go 创建的临时 Postgres 中执行生产 AutoMigrate 与版本迁移
- **AND** 导入生产版本迁移定义的系统默认数据（seed）
- **AND** 该 schema 在进程内被缓存复用，不重复构建

#### Scenario: 回归测试重新导入（逃生口）

- **WHEN** 回归测试显式调用 `testutil.ReimportTestDB`
- **THEN** 系统重建临时数据库的 `public` schema，重新执行生产 AutoMigrate 与版本迁移
- **AND** 数据库恢复为生产首次启动后的 schema 与默认数据状态
- **AND** 重新导入前的测试数据不残留
- **AND** `ReimportTestDB` 末尾的重开连接池逻辑（修 pgvector OID 漂移）不被移除

#### Scenario: 测试初始化保持物理隔离

- **WHEN** 测试初始化、重置或重新导入数据库
- **THEN** 所有操作只作用于 testcontainers-go 创建的临时 Postgres
- **AND** 系统不读取可将测试连接重定向到开发或生产数据库的 DSN

#### Scenario: seed 表名清单是受控元数据而非数据副本

- **WHEN** 生产迁移向新的表注入 seed 数据
- **THEN** 该 seed 表名 MUST 被加入 `ResetTestData` 的快照表名列表
- **AND** 快照**内容**始终运行时从黄金 schema 的 DB 读取，testutil 中不存在 seed 数据副本
- **AND** seed 数据始终来自生产迁移，不会 drift

#### Scenario: 变更范围受限

- **WHEN** 实施本 change
- **THEN** 只修改测试数据库初始化、重置、重新导入入口及对应文档
- **AND** 不修改业务逻辑、业务测试断言、生产迁移或生产默认数据

### Requirement: 进程内单例连接与单次迁移

系统 SHALL 在 testutil 包内通过 `sync.Once` 实现：(1) pgvector 容器与 `*gorm.DB` 连接的进程内单例——首次 `OpenTestDB` 启动容器并执行 `gorm.Open` 建立连接，缓存于包级 `sync.Once`（`startOnce`），后续调用复用同一实例，不重复启动容器或建池；(2) **黄金 schema 的进程内单次构建**——`runTestMigrations`（AutoMigrate 全量 model + 版本迁移 + seed）在进程内只发生一次（`migrateOnce`），无论 `SetupTestDB` 被调用多少次。

#### Scenario: 重复调用 SetupTestDB 只启动一次容器

- **WHEN** 同一测试进程内 `SetupTestDB` 被多次调用
- **THEN** 底层 testcontainer 容器与 `gorm.Open` 均仅执行一次，所有调用复用同一容器与连接池

#### Scenario: 重复调用 SetupTestDB 只迁移一次

- **WHEN** 同一测试进程内 `SetupTestDB` 被多次调用
- **THEN** `runTestMigrations`（AutoMigrate + 版本迁移 + seed）仅在首次执行
- **AND** 后续调用仅执行 `ResetTestData`（seed 保护的 truncate），不重建 schema

#### Scenario: 黄金 schema 构建后 OID 稳定

- **WHEN** 黄金 schema 已构建，后续测试通过 `ResetTestData` 重置
- **THEN** 不发生 `DROP SCHEMA` / `CREATE EXTENSION`，pgvector `vector` 类型 OID 保持不变
- **AND** 连接池的 prepared statement 不失效，不出现 `cache lookup failed for type <oid>` 错误

#### Scenario: 首次连接或迁移失败不被吞掉

- **WHEN** 首次 `gorm.Open` 或 `runTestMigrations` 失败
- **THEN** 错误被记录到包级变量，后续每次 `SetupTestDB` 调用通过 `t.Fatal` 报出该错误，不静默返回无效 `*gorm.DB`

## ADDED Requirements

### Requirement: ResetTestData — 测试间黄金态恢复重置

系统 SHALL 提供 `ResetTestData(t *testing.T, db *gorm.DB) *gorm.DB` 函数（返回重开后的新连接池句柄），作为黄金 schema 模式下测试间的重置入口，维持"reset 后 = 干净黄金态"不变量。黄金 schema 首次构建后，系统 SHALL 快照：(a) 迁移 seed 表（`ai_settings`、`embedding_config`）的全部行，(b) 所有 vector 类型列的类型声明（`pg_catalog.format_type`）。`ResetTestData` 的流程 SHALL 为：① `TRUNCATE TABLE ... RESTART IDENTITY CASCADE` 清空所有业务表（仅跳过 `schema_migrations`），② 无条件 re-ALTER 每个 vector 列回黄金类型（撤销测试可能的 `ALTER COLUMN ... TYPE vector(N)` 变异；无条件是为了保护"执行中变异"的测试，详见 design 决策⑥），③ 从快照灌回 seed 行并推进序列，④ 重开连接池（黄金 schema 从不 DROP 扩展，OID 稳定；重开只清被 ALTER 作废的 prepared statement）。返回新连接句柄（调用方 MUST 重新赋值）。`TruncateAllTables`（清空一切，无快照恢复、无重开）SHALL 原义保留。

#### Scenario: 清空业务表数据

- **WHEN** 黄金 schema 中 3 个业务表各含 10 行数据，`ResetTestData` 被调用
- **THEN** 3 个业务表的行数均为 0

#### Scenario: 从快照恢复迁移 seed 数据

- **WHEN** 黄金 schema 已构建（`ai_settings`、`embedding_config` 含迁移 seed 行），`ResetTestData` 被调用
- **THEN** `ai_settings`、`embedding_config` 恢复为快照记录的 seed 行（等价迁移刚跑完的状态），业务表为空

#### Scenario: 清除测试插入的自定义 seed 表行

- **WHEN** 某测试在 `ai_settings` 插入了非迁移 seed 的自定义键（如 `semantic_board_match_sim_threshold`），下个测试调用 `ResetTestData`
- **THEN** 该自定义键被 truncate 清除，未出现在快照中，不被恢复
- **AND** 下个测试拿到的 `ai_settings` 只含迁移 seed 行（等价生产首次启动）

#### Scenario: 恢复被测试变异的 vector 列维度

- **WHEN** 某测试执行了 `ALTER COLUMN embedding TYPE vector(2560)` 变异了黄金态的 vector 列，下个测试调用 `ResetTestData`
- **THEN** 该 vector 列被 re-ALTER 回黄金构建时的类型声明（如无维度 `vector`）
- **AND** 下个测试插入 3 维向量 `[1,0,0]` 不再报维度不匹配

#### Scenario: 重开连接池清除失效的 prepared statement

- **WHEN** 某测试的 ALTER 变异使连接池的 prepared statement 失效，`ResetTestData` 被调用
- **THEN** 连接池被重开，返回新句柄，后续查询不再报 `cached plan must not change result type`
- **AND** 调用方 MUST 使用返回的新句柄（传入的旧句柄连接已关闭）

#### Scenario: 保留 schema_migrations 版本记录

- **WHEN** `ResetTestData` 被调用
- **THEN** `schema_migrations` 表的迁移版本记录不被清空，避免误判迁移未应用

#### Scenario: 外键关联表被 CASCADE 清空

- **WHEN** `topic_tags` 表有 1 行，`topic_tag_embeddings` 表通过外键关联且有 3 行
- **THEN** `ResetTestData` 后两张业务表均为空
