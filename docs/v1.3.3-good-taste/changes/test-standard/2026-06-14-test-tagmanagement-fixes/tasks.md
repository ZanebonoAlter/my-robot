## 1. 复用生产初始化

- [x] 1.1 `SetupTestDB` 使用生产 `RunAutoMigrate` 和 `RunMigrations`
- [x] 1.2 删除 testutil 中重复维护的生产 DDL 和默认数据 seed
- [x] 1.3 保持 testcontainers-go 隔离和 `-short` 跳过行为

## 2. 支持回归重新导入

- [x] 2.1 新增 `ReimportTestDB`
- [x] 2.2 重新导入时重建临时数据库 `public` schema
- [x] 2.3 重新导入与首次初始化共用生产迁移路径

## 3. 范围约束

- [x] 3.1 不修改 tagmanagement 业务逻辑
- [x] 3.2 不修改业务测试断言和测试夹具
- [x] 3.3 不修改生产迁移和生产默认数据

## 4. 文档与验证

- [x] 4.1 更新 `docs/reference/testing.md`
- [ ] 4.2 验证 `go test ./internal/platform/testutil ./internal/tagmanagement/... -count=1`
- [ ] 4.3 验证 `go test -short ./internal/tagmanagement/...`
- [x] 4.4 验证 `go build ./...`
- [x] 4.5 验证 `golangci-lint run ./internal/platform/testutil/... ./internal/tagmanagement/...`
