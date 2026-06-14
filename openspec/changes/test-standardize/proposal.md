## Why

当前测试基础设施存在三个核心问题：(1) 22 个测试文件使用 SQLite 内存数据库，走与生产不同的代码路径（pgvector 相关逻辑在测试中无法覆盖）；(2) 没有共享 testutil 包，每个测试文件重复编写 `setupXxxTestDB()`；(3) 缺乏清晰的单元测试 vs 集成测试边界定义。需要建立规范的测试框架，确保测试可靠性和生产代码路径的一致性。

## What Changes

- 新建 `backend-go/internal/platform/testutil/` 包，提供共享测试工具：
  - `testutil.SetupTestDB(t)` — 单一入口：连接 Postgres + 迁移所有 model + truncate + 设 `database.DB` + 返回 `*gorm.DB`
- 将 `tagmanagement/` 下全部 **14 个** 测试文件迁移到 Postgres（13 个直接依赖 SQLite + 1 个间接复用 setup 函数；从 repository → service → handler 自底向上）
- 在 `auxiliary_label_service.go` 中评估并清理 SQLite fallback 代码（Go 侧余弦比较路径）
- 明确测试分层：`-short` 跳过 DB 测试，默认走真实 Postgres

## Capabilities

### New Capabilities

- `test-infrastructure`: 统一的 Go 测试基础设施，提供 Postgres 连接（进程内单例）、单次全量迁移、清理工具，单元/集成测试分层规范，以及 CI 双层测试策略

### Modified Capabilities

- `tagging-domain`: 移除生产代码中的 SQLite fallback 路径，统一走 pgvector
- `tag-embedding-management`: 相关测试迁移到 Postgres，确保 embedding 比较逻辑与生产一致

## Impact

- `backend-go/internal/platform/testutil/`：新建包（~1 文件）
- `backend-go/internal/tagmanagement/`：14 个测试文件重写为 Postgres 测试（13 个直接依赖 SQLite + `semantic_board_backfill_test.go` 间接复用 setup）
  - `handler/` × 2：semantic_board_handler, merge_reembedding_queue
  - `repository/` × 2：tagger_embedding, tag_job_queue
  - `service/auxlabel/` × 1：auxiliary_label_service
  - `service/board/` × 4：semantic_board_matching, upgrade, backfill, tag_clustering
  - `service/core/` × 5：embedding, hard_merge, merge_tags_reembedding, metadata, quality_score
- `backend-go/internal/tagmanagement/service/auxlabel/auxiliary_label_service.go`：评估/简化 SQLite fallback
- `backend-go/go.mod`：`glebarez/sqlite` 暂保留（reader/admin/topicgraph 还有 9 个 SQLite 文件，不在本 change scope）
- `.github/workflows/`：CI 新增单元测试 job（`-short`，无 Docker）+ 集成测试 job（`pgvector` service container）
- 开发者需要本地 Docker 环境运行集成测试
