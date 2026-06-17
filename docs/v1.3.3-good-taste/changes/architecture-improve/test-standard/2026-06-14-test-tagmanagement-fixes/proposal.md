## Why

tagmanagement 集成测试当前维护了一套测试专用数据库初始化逻辑。测试 schema、约束和默认数据需要人工跟随生产迁移更新，容易与真实生产状态不一致，导致测试结果失真。

测试数据库应直接复用生产数据库初始化逻辑，以生产迁移定义的 schema、约束和默认数据作为唯一来源。

## What Changes

- 测试数据库首次启动时，直接执行生产的 `database.RunAutoMigrate` 和 `database.RunMigrations`
- 生产迁移中的默认数据自动导入测试数据库
- 提供回归测试重新导入入口，可将测试数据库恢复到生产首次启动后的状态
- 删除 testutil 中重复维护的生产 DDL 和默认数据副本
- 保持测试数据库由 testcontainers-go 隔离，不连接开发或生产数据库

## Scope

本 change 只修改测试数据库初始化与重新导入机制。

不修改：

- tagmanagement 生产业务逻辑
- 现有业务测试断言和测试数据
- 生产迁移内容与生产默认数据
- API、前端、数据库连接配置

## Capabilities

### Modified Capabilities

- `test-infrastructure`: 集成测试数据库使用生产迁移初始化，并支持回归测试重新导入生产初始化状态

## Impact

- `backend-go/internal/platform/testutil/testutil.go`
- `docs/reference/testing.md`
- tagmanagement 集成测试运行方式不变，仍使用 `testutil.SetupTestDB(t)`
