## Why

当前测试基础设施存在三个核心问题：(1) 20+ 测试文件使用 SQLite 内存数据库，走与生产不同的代码路径（pgvector 相关逻辑在测试中无法覆盖）；(2) 没有共享 testutil 包，每个测试文件重复编写 `setupXxxTestDB()`；(3) 缺乏清晰的单元测试 vs 集成测试边界定义。需要建立规范的测试框架，确保测试可靠性和生产代码路径的一致性。

## What Changes

- 新建 `backend-go/internal/platform/testutil/` 包，提供共享测试工具：
  - `testutil.OpenTestDB(t)` — 基于 Docker/Postgres 的测试数据库连接（支持 testcontainer 或预置 Postgres 实例）
  - `testutil.SetupTestDB(t)` — 连接 + 迁移 + 清理
  - `testutil.TruncateTables(t, db)` — 测试间数据隔离
- 定义测试分层规范：
  - 单元测试（`go test -short`）：使用 stub/mock，不依赖数据库
  - 集成测试（`go test`）：使用真实 Postgres，覆盖 pgvector/GORM/事务等
- 将 `tagging/` 包中依赖 pgvector 的关键测试从 SQLite 迁移到 Postgres（优先级最高的测试文件：`semantic_board_matching_test.go`、`embedding_test.go`、`auxiliary_label_service_test.go`）
- 删除生产代码中为 SQLite 测试编写的 fallback 代码（如 `auxiliary_label_service.go` 中的 Go 侧余弦比较 fallback）
- 移除 `github.com/glebesz/sqlite` 依赖

## Capabilities

### New Capabilities

- `test-infrastructure`: 统一的 Go 测试基础设施，提供 Postgres 测试容器连接、迁移、清理工具，以及单元/集成测试分层规范

### Modified Capabilities

- `tagging-domain`: 移除生产代码中的 SQLite fallback 路径，统一走 pgvector
- `tag-embedding-management`: 相关测试迁移到 Postgres，确保 embedding 比较逻辑与生产一致

## Impact

- `backend-go/internal/platform/testutil/`：新建包
- `backend-go/internal/domain/tagging/`：约 10+ 测试文件重写为 Postgres 测试
- `backend-go/internal/domain/content/`、`narrative/`、`topicgraph/` 等：测试文件逐步迁移
- `backend-go/internal/domain/tagging/auxiliary_label_service.go`：移除 SQLite fallback 代码
- `backend-go/go.mod`：移除 `github.com/glebarez/sqlite` 依赖
- CI 需要确保 Postgres 服务可用（testcontainer 或预置服务）
- 开发者需要本地 Docker 环境运行集成测试
