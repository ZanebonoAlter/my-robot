## MODIFIED Requirements

### Requirement: 测试数据库使用生产初始化逻辑

系统 SHALL 使用生产的 `database.RunAutoMigrate` 和 `database.RunMigrations` 初始化隔离的测试数据库。生产迁移 SHALL 是测试 schema、约束和系统默认数据的唯一来源。黄金 schema 在进程内只构建一次；需要"干净迁移态"的少数回归测试 SHALL 显式调用 `ReimportTestDB` 逃生口。为维持"测试库跑全量生产迁移（含破坏性 TRUNCATE 清理迁移）"的不变量，测试迁移路径 SHALL 在执行前显式开启破坏性迁移开关（`t.Setenv("MIGRATIONS_ALLOW_DESTRUCTIVE","1")`），与生产默认拒绝破坏性迁移的安全收紧对齐。

#### Scenario: 黄金 schema 首次构建

- **WHEN** 进程内首次调用 `testutil.SetupTestDB`
- **THEN** 系统在 testcontainers-go 创建的临时 Postgres 中执行生产 AutoMigrate 与版本迁移
- **AND** 导入生产版本迁移定义的系统默认数据（seed）
- **AND** 该 schema 在进程内被缓存复用，不重复构建

#### Scenario: 测试迁移路径开启破坏性迁移开关

- **WHEN** `runTestMigrations` 执行前
- **THEN** 系统 SHALL 通过 `t.Setenv("MIGRATIONS_ALLOW_DESTRUCTIVE","1")` 开启破坏性迁移开关
- **AND** 破坏性迁移（如 `20260706_0001` TRUNCATE `topic_lifeline_context`）在测试库照常执行，维持测试库等价于"生产首次启动 + 全量历史清理"的状态

#### Scenario: 回归测试重新导入（逃生口）

- **WHEN** 回归测试需要"干净迁移态"（绕过黄金 schema 缓存）
- **THEN** 测试 SHALL 显式调用 `ReimportTestDB`，该函数重建 schema 时同样开启破坏性迁移开关
