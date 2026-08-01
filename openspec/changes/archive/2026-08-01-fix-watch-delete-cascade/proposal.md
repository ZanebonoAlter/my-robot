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

- `db-migration-execution`: 把 `ADD CONSTRAINT ... FOREIGN KEY` 显式补进「长锁 DDL 须在 lock_timeout 守卫内执行」requirement 的枚举清单——FK 加约束与 `ADD CONSTRAINT ... UNIQUE` 同构（AccessExclusiveLock + 全表行校验），本就在该 requirement「等长 AccessExclusiveLock」精神内，本次显式化并防后人加 FK 迁移时漏套 `withLockTimeout`。本 change 的 FK 迁移即首个落地用例（design D3）。

## Impact

- **代码**：`backend-go/internal/platform/database/postgres_migrations.go`（加迁移 `20260801_0002`）。
- **spec delta**：`specs/db-migration-execution/spec.md` MODIFIED「长锁 DDL」requirement（FK 补进枚举 + 专属 scenario）。
- **测试**：`topic_watch_repository_test.go` 2 个级联测试去掉 `t.Skip`；`db_unit_test.go` 加迁移注册断言；新增 testcontainer PG 历史数据迁移测试（testing.md §146 硬约束）。
- **文档**：`docs/reference/database/ER_DIAGRAM.md`（「外键约束真相」全库真实 FK 1→2）+ `DATABASE_FIELDS.md`（`watch_id` FK 约束列 + 约束表）。
- **无 API 变化 / 无 model 变化 / 不改 DeleteWatch 产品代码**——model tag 已声明 CASCADE 意图，本 change 补齐迁移让 PG 行为对齐意图。
- **部署后影响**：① 生产 `DeleteWatch` 将真正级联删 hits（当前留孤儿）；② 迁移先无条件清理存量孤儿 hit 再加 FK（孤儿清理**不**套破坏性守卫——否则生产默认不清理→FK 校验失败→启动失败，见 design D2）；③ 全库真实 DB 级 FK 从 1 条变 2 条。**无需用户手动操作**，迁移随启动自动跑；旧孤儿 hit 行被清理（垃圾数据，不可逆但本就不该存在）。
