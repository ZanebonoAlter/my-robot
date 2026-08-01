# Design: fix-watch-delete-cascade

## Context

`DeleteWatch` 在 PostgreSQL 上不级联删 `topic_watch_hits`，留下孤儿行——根因已在 proposal 逐条坐实（model tag 声明 `OnDelete:CASCADE`，但 `DisableForeignKeyConstraintWhenMigrating=true` 致 AutoMigrate 不建 FK，而版本化迁移 `20260630_0001/_0002` 只加了 status CHECK + 复合唯一索引，漏建 FK）。本 change 只做一件事：补一条版本化 FK ON DELETE CASCADE 迁移，让 PG 行为对齐 model tag 意图，并 re-enable 被 `t.Skip` 的 2 个级联测试。

对照样板：`topic_tags` 在 `postgres_migrations.go:~590` 已用「DROP 旧约束 → ADD 新 CASCADE 约束」模式补过 FK，本 change 复用同一模式；`db-migration-execution` spec 已确立三条规矩（`RunOutsideTx` / 不可逆标注 / `withLockTimeout`），本 change 在其框架内落地。

## 决策

### D1. 版本号：`20260801_0002`

`20260801_0001` 已被同日 active change `feed-param-options`（route_param_options seed）占用；取下一个空闲号 `20260801_0002`，单调 > 当前最大 `20260801_0001`、与同日 change 不冲突。（proposal 早期假设的最大值 `20260727_0001` 实际已被多个 07/08 月迁移超过。）

### D2. 孤儿清理必须无条件执行（不套破坏性迁移守卫）

`ADD CONSTRAINT ... FOREIGN KEY` 会**校验全部现有行**；若存在引用已删 watch 的孤儿 hit 行，校验失败 → 迁移报错 → `InitDB` 失败 → 应用启动失败。因此迁移闭包内必须**先清理孤儿、再加 FK**，且清理**不能**套 `IsDestructiveAllowed()` 守卫。

- 清理 SQL：`DELETE FROM topic_watch_hits WHERE watch_id NOT IN (SELECT id FROM board_topic_watches)`，`logging.Infof` 打印删除行数。
- **不套守卫**的两层理由：
  1. **正确性硬约束**：守卫默认关闭（生产不设 `MIGRATIONS_ALLOW_DESTRUCTIVE`），若清理被跳过 → 孤儿残留 → FK 加不上 → 启动炸。清理是「让 FK 能加上」的前置必要步，不是可选的破坏性操作。
  2. **语义**：孤儿 hit 引用的是已不存在的 watch，是垃圾数据；与 `20260628_0001` prune underqualified candidates 同类（那也是裸 DELETE + log，不套守卫）。`db-migration-safety` spec 把守卫明确限定给 `TRUNCATE`/`DROP TABLE`/`DROP COLUMN` 等「数据销毁操作」，孤儿清理不在此列。
- **备选**：套 `IsDestructiveAllowed()` → **否决**（理由 1 直接致命：生产默认不清理→启动失败）。

### D3. FK 加约束套 `withLockTimeout`，并显式补进 spec 枚举

`ADD CONSTRAINT ... FOREIGN KEY` 与 spec 已列举的 `ADD CONSTRAINT ... UNIQUE` 同构——都要 `AccessExclusiveLock` + 全表行校验扫描。spec「长锁 DDL 须在 lock_timeout 守卫内执行」措辞「**等长 AccessExclusiveLock**」本就涵盖 FK，但枚举清单（「三类高危操作」scenario）只列了 vector TYPE / UNIQUE / 扩展 UNIQUE，没点名 FK。本 change：

- 迁移闭包内把 `ADD CONSTRAINT ... FK` 包进 `withLockTimeout(db, "5s", fn)`（helper 已有 `postgres_migrations.go:84`，同文件 3 处既有用法 125/637/918）。
- 同步把 FK 显式补进 `db-migration-execution` spec 的 lock_timeout requirement 枚举（MODIFIED requirement，见 `specs/db-migration-execution/spec.md`），让 proposal「Modified Capabilities: db-migration-execution」名副其实，并防后人加 FK 迁移时漏套守卫。
- **备选**：不套守卫（表小、不在枚举字面）→ **否决**：spec 精神已覆盖，且与同文件既有 3 处用法一致，reviewer 预期一致；表小只是「超时几乎不触发」，不构成「不套」的理由。

### D4. 幂等性 + 事务路径

- 清理 DELETE 天然幂等（再跑也是 0 行）。
- FK 加约束用 `DO $$ IF NOT EXISTS (SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='fk_topic_watch_hits_watch') THEN ALTER TABLE topic_watch_hits ADD CONSTRAINT ... END $$` 守卫，仿 `topic_tags` 既有模式。
- `ADD CONSTRAINT ... FK` 事务兼容（不像 `CREATE INDEX CONCURRENTLY`），走默认事务内路径，**不**声明 `RunOutsideTx`。
- 迁移 `Description` 含 `Idempotent.` 注记（仓库惯例，如 `20260719_0001`）。

### D5. 测试覆盖（testing.md §146 硬约束）

`standard/backend/testing.md` 硬约束：「凡 change 涉及 schema migration，门禁必须包含 testcontainer PG 迁移测试——先插历史行 → 跑迁移 → 验证」。本 change 落三条测试：

1. **testcontainer PG 迁移测试**（`db_..._test.go`）：插 1 个 watch + 2 个合法 hit + 1 个孤儿 hit → 单独跑本迁移闭包 → 断言 ① 孤儿被删（合法 hit 保留）② `information_schema.table_constraints` 能查到 `fk_topic_watch_hits_watch` ③ 删 watch → 其 hit 被 PG 级联清掉（无需应用层）。
2. **迁移注册断言单测**（`db_unit_test.go`，仿 `TestWatchHitUniqueIndexMigrationRegistered`）：断言 `20260801_0002` 在 `postgresMigrations()` 列表里、Description 含 `topic_watch_hits`、源码含清理 SQL + ADD CONSTRAINT + IF NOT EXISTS 守卫 + withLockTimeout 调用。
3. **re-enable 2 个级联测试**：去掉 `TestDeleteWatchCascadesHits` / `TestDeleteWatchDoesNotAffectOtherWatchHits` 的 `t.Skip`，断言逻辑一字不改（proposal 已述）。

### D6. spec delta 边界

只动 `db-migration-execution` 的 lock_timeout requirement（加 FK 进枚举 + 一个 FK 专属 scenario）。**不**动 `db-migration-safety`（破坏性守卫语义不变——孤儿清理本就不属破坏性，D2 已论证）。**不**新增 capability。

## 风险与回滚

- **[孤儿清理误删]**：清理只删 `watch_id NOT IN (SELECT id FROM board_topic_watches)` 的行，语义上必是引用已删 watch 的垃圾行；且若不清理，FK 加不上。→ 可接受；`logging.Infof` 打印删除行数可审计，运维可见。
- **[FK 加约束锁表]**：`topic_watch_hits` 是单用户 watch 场景的小表，5s `lock_timeout` 几乎不会触发；守卫是防御性一致，仅在锁竞争 >5s 时让语句失败而非无限阻塞。→ 可接受。
- **[存量孤儿数量未知]**：生产是否曾删过 watch 决定孤儿数量（可能为 0）。清理幂等、log 行数，不阻塞；最坏情况删若干垃圾行。
- **[ER 图「唯一真实 FK」声明需更新]**：落地后全库真实 DB 级 FK 从 1 条（`topic_tags_merged_into_id_fkey`）变 2 条（+`fk_topic_watch_hits_watch`）。`ER_DIAGRAM.md`「外键约束真相」节 + `DATABASE_FIELDS.md` watch_id 约束列 + 约束表必须同步（database 域 doc-impact，见 tasks §文档）。
- **[回滚]**：纯加法迁移（+清理垃圾行），git revert 迁移闭包即可；已清孤儿不可逆，但本就是垃圾数据。

## 不做

- **不改 `DeleteWatch` 产品代码**：FK 级联落地后，其 docstring「cascade-deletes its hits (via FK OnDelete:CASCADE)」名副其实，无需在应用层手动级联（手动级联是 proposal 列的 option (b)，本 change 选 option (a)）。
- **不动 model tag**：tag 已声明 CASCADE 意图，本 change 只补迁移让 PG 对齐意图。
- **不做全库 FK 治理**：`ER_DIAGRAM.md` 揭示全库大量 GORM 逻辑关联无 DB FK，那是独立架构 change，不在本次范围。
