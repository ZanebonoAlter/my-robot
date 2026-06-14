## Context

生产数据库启动时依次执行：

1. `database.RunAutoMigrate`
2. `database.RunMigrations`

版本迁移同时定义生产约束、索引、vector 列和系统默认数据。测试基础设施若复制其中一部分，就会形成第二套数据库定义并随时间漂移。

本 change 中的“生产数据”仅指生产版本迁移内置的系统默认/种子数据，不包含任何开发或生产数据库中的用户运行数据。

## Goals / Non-Goals

**Goals:**

- 首次启动测试数据库时直接运行生产初始化逻辑
- 测试使用与生产一致的 schema、约束和默认数据
- 回归测试可以重新导入生产初始化状态
- 生产迁移成为测试数据库定义的唯一来源

**Non-Goals:**

- 不复制、dump 或恢复真实生产数据库
- 不修改生产迁移和生产默认数据
- 不修改 tagmanagement 业务代码、测试断言或测试夹具
- 不增加测试专用业务接口

## Decisions

### D1: 直接调用生产迁移

testutil 不再手写生产 DDL 或 seed。初始化统一调用 `database.RunAutoMigrate` 和 `database.RunMigrations`。

### D2: 重新导入通过重建隔离 schema 完成

仅执行 `TRUNCATE` 会删除默认数据，但保留 `schema_migrations`，之后 `RunMigrations` 会跳过已记录版本。因此重新导入需要在 testcontainer 内重建 `public` schema，再执行完整生产初始化。

### D3: 首次启动与回归重新导入共用实现

`SetupTestDB` 使用统一的导入函数完成初始化；回归测试通过 `ReimportTestDB` 显式调用同一实现，不维护第二条重置路径。

### D4: 严格隔离真实数据库

testutil 只使用 testcontainers-go 创建的临时 Postgres，不接受外部 DSN 覆盖。schema 重建只发生在该临时数据库中。

## Trade-offs

- 完整生产迁移比测试专用 seed 慢，但避免两套 schema 和默认数据长期漂移
- 生产迁移出现问题时，集成测试会直接失败；这是期望行为，因为测试应反映真实生产初始化能力
