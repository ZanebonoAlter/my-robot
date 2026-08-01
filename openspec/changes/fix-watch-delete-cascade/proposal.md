## Why

`repository-tests-pg-only` change 把 `topic_watch_repository_test.go` 从内存 SQLite 迁到 testcontainer PostgreSQL 时，暴露出一个**生产 bug**：`DeleteWatch` 在 PostgreSQL 上**不级联删除** `topic_watch_hits`，留下孤儿 hits 行。

根因（已从源码坐实）：
- model tag 声明 `gorm:"foreignKey:WatchID;constraint:OnDelete:CASCADE"`，意图级联。
- 但 `db.go:21` 与 `testutil.go:148` 都开 `DisableForeignKeyConstraintWhenMigrating: true`，AutoMigrate **不建任何 FK**。
- 版本化迁移 `20260630_0001/_0002` 只加了 status CHECK + 复合唯一索引，**漏建 FK CASCADE**（对比 `topic_tags` 在 `postgres_migrations.go:595` 显式加了 `FOREIGN KEY ... ON DELETE CASCADE`）。
- `DeleteWatch` 实现（`topic_watch_repository.go:74`）只删 watch、不手动删 hits，纯靠 FK 级联——但 FK 根本没建。

SQLite 测试靠 `PRAGMA foreign_keys=ON` 让级联生效所以全绿，**掩盖了这个 bug**——正是「SQLite 全绿、生产 PG 炸」的典型案例，也是 `repository-tests-pg-only` 迁移的价值证明。

## What Changes

- 加版本化迁移：`ALTER TABLE topic_watch_hits ADD CONSTRAINT ... FOREIGN KEY (watch_id) REFERENCES board_topic_watches(id) ON DELETE CASCADE`，对齐 model tag 意图。
- 处理历史孤儿数据：迁移前清理 `topic_watch_hits` 中引用已删除 `board_topic_watches` 的孤儿行（若有），否则 `ADD CONSTRAINT` 会因违约外键失败。遵循 `testing.md`「schema 迁移要在 testcontainer PG + 历史数据下测」硬约束。
- 迁移需 testcontainer PG + 历史数据测试覆盖：先插孤儿行 → 跑迁移 → 验证清理 + FK 建立 + 级联生效。
- 落地后 re-enable `repository-tests-pg-only` 暂存的 2 个级联测试（`TestDeleteWatchCascadesHits`、`TestDeleteWatchDoesNotAffectOtherWatchHits`），去掉 `t.Skip`。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `db-migration-execution`: 新增一条 FK ON DELETE CASCADE 迁移（board_topic_watches ← topic_watch_hits），含历史孤儿数据清理步骤。

## Impact

- **代码**：`backend-go/internal/platform/database/postgres_migrations.go`（加迁移）。
- **测试**：`topic_watch_repository_test.go` 2 个级联测试去掉 `t.Skip`（re-enable）。
- **无 API 变化 / 无 model 变化**——model tag 已声明 CASCADE 意图，本 change 只是补齐迁移让 PG 行为对齐意图。
- **部署后影响**：生产 `DeleteWatch` 将真正级联删 hits（当前留孤儿）；迁移会清理存量孤儿数据。
