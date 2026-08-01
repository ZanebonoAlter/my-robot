# Tasks: fix-watch-delete-cascade（补 FK ON DELETE CASCADE 迁移，修 DeleteWatch 级联）

> 垂直切片，每切片独立可交付、可验证。推荐顺序：1 迁移闭包（清理 + FK + lock_timeout）→ 2 测试（注册单测 + testcontainer 历史数据迁移测试 + re-enable 2 个 t.Skip）→ 3 文档同步（ER 图「唯一真实 FK」声明 + DATABASE_FIELDS 约束列）。尾部遵循《开发执行规范》§11 归档门禁。
>
> ⚠️ 前置：根因已在 proposal 逐条坐实；决策见 `design.md` D1-D6。后端命令走 WSL；testcontainer 集成测试需 Docker daemon 可用（testcontainer 自管隔离容器）。
>
> 📌 本 change 选 proposal 的 option (a)：加版本化 FK 迁移对齐 model tag 意图；**不改** `DeleteWatch` 产品代码、**不动** model tag、**不**做全库 FK 治理。

<!-- doc-impact: database -->
<!--
  database 域：命中（改 internal/platform/database/postgres_migrations.go + 同步 ER_DIAGRAM.md /
    DATABASE_FIELDS.md）。落地后全库真实 DB 级 FK 从 1 条（topic_tags_merged_into_id_fkey）
    变 2 条（+ fk_topic_watch_hits_watch），ER 图「外键约束真相」节 + DATABASE_FIELDS watch_id
    约束列 + 约束表必须同步。
  flow 域：启发式命中（DeleteWatch 行为变化），但级联删 hits 是数据完整性兜底，非用户可见业务
    流程步骤，flow/ 无需更新——按《开发执行规范》§12.2 豁免溯源。
  无 api / standard / configuration / deployment 影响：不改 API、不碰 testing.md（§146 硬约束已
    存在本 change 只是遵守）、不加 env、不改部署流程。
-->

## 1. 后端：版本迁移闭包（清理孤儿 + 加 FK CASCADE + lock_timeout 守卫）

- [x] 1.1 `internal/platform/database/postgres_migrations.go` 新增迁移 `20260801_0002`，Description：`"Add FK ON DELETE CASCADE on topic_watch_hits(watch_id)→board_topic_watches(id); clean orphan hits first. Idempotent."`。`Up` 闭包内按序执行：① `tableExists(db,"topic_watch_hits") && tableExists(db,"board_topic_watches")` 守卫（表不存在则 no-op）；② 清理孤儿 `DELETE FROM topic_watch_hits WHERE watch_id NOT IN (SELECT id FROM board_topic_watches)`，`RowsAffected` 经 `logging.Infof("Migration 20260801_0002: cleaned %d orphan topic_watch_hits rows", n)` 打印；③ `DO $$ IF NOT EXISTS (... constraint_name='fk_topic_watch_hits_watch' ...) THEN ALTER TABLE topic_watch_hits ADD CONSTRAINT fk_topic_watch_hits_watch FOREIGN KEY (watch_id) REFERENCES board_topic_watches(id) ON DELETE CASCADE END $$` 幂等加 FK。验收：迁移出现在 `postgresMigrations()` 列表、Version 单调 > `20260727_0001`
- [x] 1.2 把第 ③ 步的 `ADD CONSTRAINT` 包进 `withLockTimeout(db, "5s", func(tx *gorm.DB) error { ... })` 守卫（对齐 db-migration-execution spec「长锁 DDL」+ design D3；与同文件既有 3 处用法 125/637/918 一致）。验收：grep 该迁移 Up 闭包含 `withLockTimeout(db, "5s"` ✓（实测命中）
- [x] 1.3 **不**声明 `RunOutsideTx`（ADD CONSTRAINT FK 事务兼容）；**不**在 Description 加「⚠️ 不可逆」标注（孤儿清理非 TRUNCATE/DROP，非破坏性，design D2）；保留 `Idempotent.` 注记。验收：迁移结构体只有 `Version/Description/Up` 三字段

## 2. 后端：测试补齐（testing.md §146 硬约束：schema 迁移须 testcontainer PG + 历史数据）

- [x] 2.1 `internal/platform/database/db_unit_test.go` 新增 `TestWatchHitFKCascadeMigrationRegistered`（仿 `TestWatchHitUniqueIndexMigrationRegistered`）：`mustFindMigration(postgresMigrations(), "20260801_0002")`，断言 Description 含 `topic_watch_hits`；读 `postgres_migrations.go` 源码 `mustContainAll` 含清理 SQL `DELETE FROM topic_watch_hits WHERE watch_id NOT IN (SELECT id FROM board_topic_watches)`、`ADD CONSTRAINT fk_topic_watch_hits_watch`、`ON DELETE CASCADE`、`withLockTimeout(db, "5s"`、`IF NOT EXISTS`。验收：该单测 PASS（无 Docker）
- [x] 2.2 testcontainer PG 历史数据迁移测试（testcontainer，需 Docker）：seed —— 插 1 个 watch(w1) + 2 个 w1 的合法 hit + 1 个 `watch_id=99999`（不存在的 watch）孤儿 hit；单独调用本迁移闭包 `postgresMigrations()[i].Up(db)`；断言 ① 孤儿 hit 被删、w1 的 2 个合法 hit 保留 ② `SELECT 1 FROM information_schema.table_constraints WHERE constraint_name='fk_topic_watch_hits_watch'` 命中 ③ `DELETE FROM board_topic_watches WHERE id=w1` 后 w1 的 hit 被 PG 级联清掉（`SELECT count(*) FROM topic_watch_hits WHERE watch_id=w1` = 0，无需应用层）。验收：该集成测试 PASS
- [x] 2.3 `internal/topicgraph/repository/topic_watch_repository_test.go`：去掉 `TestDeleteWatchCascadesHits`、`TestDeleteWatchDoesNotAffectOtherWatchHits` 的 `t.Skip(...)` 行，**断言逻辑一字不改**（proposal 已述）。验收：`grep -n 't.Skip' topic_watch_repository_test.go` → 零命中
- [x] 2.4 黄金 schema 已通过 `RunMigrations` 自动拾取新 FK（testutil 跑全量生产迁移），无需改 testutil。验收：`go test ./internal/topicgraph/repository` 两个级联测试在 PG 下 PASS（不再 SKIP）

## 3. 文档：database 域同步（ER 图「唯一真实 FK」声明 + DATABASE_FIELDS 约束列）

- [x] 3.1 `docs/reference/database/ER_DIAGRAM.md`「⚠️ 外键约束真相（必读）」节：把「全库唯一真实存在的 DB 级外键只有 1 条」更新为 2 条——补 `fk_topic_watch_hits_watch`（`topic_watch_hits.watch_id → board_topic_watches.id ON DELETE CASCADE`，`postgres_migrations.go` 迁移 `20260801_0002`）；同步 line ~458「除 board_topic_watches → topic_watch_hits（声明 CASCADE）外…」表述（该关系现已落地为真实 DB FK）+ line ~527 该关系说明。验收：`grep -n 'fk_topic_watch_hits_watch' docs/reference/database/ER_DIAGRAM.md` → 命中
- [x] 3.2 `docs/reference/database/DATABASE_FIELDS.md`：① §9.7 `topic_watch_hits.watch_id` 行约束列补 `FK fk_topic_watch_hits_watch → board_topic_watches(id) ON DELETE CASCADE`（迁移 `20260801_0002`）；② 约束/索引总表（line ~1227/1243 附近）补一行 FK 约束。验收：`grep -n 'fk_topic_watch_hits_watch' docs/reference/database/DATABASE_FIELDS.md` → 命中

## 4. 归档门禁（开发执行规范 §11）

### 测试

- [x] 4.1 `cd backend-go && go test ./internal/platform/database -run 'TestWatchHitFKCascadeMigration|TestWatchHitUniqueIndexMigration'` → 全 PASS（注册单测，无 Docker）
- [x] 4.2 `cd backend-go && go test ./internal/platform/database -run '<2.2 testcontainer 迁移测试>'` → PASS（需 Docker）
- [x] 4.3 `cd backend-go && go test ./internal/topicgraph/repository` → 全 PASS、**0 SKIP**（2 个级联测试 re-enable 后通过）
- [x] 4.4 `grep -rn 't.Skip' backend-go/internal/topicgraph/repository/topic_watch_repository_test.go` → 零命中

### 文档

- [x] 4.5 `bash scripts/doc-impact.sh verify openspec/changes/fix-watch-delete-cascade` → 通过（声明: database，文件: ER_DIAGRAM.md + DATABASE_FIELDS.md）
- [x] 4.6 `bash scripts/check-standards.sh` → 本 change 相关全绿（F 段 doc-impact 通过）

### 验证

- [x] 4.7 `cd backend-go && golangci-lint run ./internal/platform/database ./internal/topicgraph/repository` → 0 issues
- [x] 4.8 `cd backend-go && go vet ./internal/platform/database ./internal/topicgraph/repository && go build ./...` → 无错误
- [x] 4.9 `openspec validate fix-watch-delete-cascade --strict` → Change is valid
- [x] 4.10 `grep -rn 'fk_topic_watch_hits_watch\|20260801_0002' docs/reference/database/ backend-go/internal/platform/database/` → 文档与迁移代码双向一致
