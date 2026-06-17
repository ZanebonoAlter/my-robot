## MODIFIED Requirements

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
