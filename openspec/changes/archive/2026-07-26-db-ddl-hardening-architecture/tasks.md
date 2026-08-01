# Tasks: db-ddl-hardening-architecture（切片2a — migrator 框架基建）

> 来源：`docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` D-High-3（根因）/ D-Med-7 / D-High-4（迁移层部分）。
>
> 切片2a 只搭 migrator 框架能力，不改应用层迁移逻辑。切片2b（CONCURRENTLY 索引 / 向量维度统一 / tag 收口 / 运行时 ensurer lock_timeout）依赖本切片的 `RunOutsideTx`，拆到后续 change。
>
> 垂直切片，每切片独立可交付、可验证。推荐顺序：A 结构体+执行器（TDD）→ B Down 字段+标注 → C lock_timeout helper+守卫。尾部遵循《开发执行规范》§11 归档门禁。

<!-- doc-impact: database, standard, architecture -->
<!--
  database 域：启发式命中（改了 internal/platform/database/），但无 schema 变化——
    DATABASE_FIELDS.md 无需更新（无新表/列）；_index.md 可选补迁移执行器能力说明。
  standard 域：必更——withLockTimeout / RunOutsideTx / Down 是新的迁移编写约定，
    必须落到 code-style.md 让未来迁移作者知道何时用（否则守卫模式不传播）。
  architecture 域：可选——map.md 的 platform/database 平台层能力扩展说明。
  实际必更的文档：standard/backend/code-style.md（迁移编写规范子节）。
-->

## 1. 后端：D-High-3 migrator 支持事务外迁移（A · db-migration-execution）

> TDD：先写失败测试（outside_tx_test.go），再改 migrator.go 让测试过。

- [x] 1.1 **[RED]** 新增 `backend-go/internal/platform/database/outside_tx_test.go`（`database_test` 包，testcontainer 集成）。先写 3 个测试（此时应失败，因为 `RunOutsideTx` 字段不存在）：
  - `TestRunOutsideTx_NotInTransaction`：注入一条 `RunOutsideTx: true` 测试迁移，Up 闭包内 `db.Raw("SELECT pg_is_in_transaction()").Scan(&inTx)` 断言 `inTx == false`（证明脱离外层事务）。
  - `TestRunOutsideTx_FailureNotRecorded`：测试迁移 Up 返回 `errors.New("simulated")`，跑完后 `SELECT COUNT(*) FROM schema_migrations WHERE version=?` 断言为 0（失败不记录，可重试）。
  - `TestRunOutsideTx_SuccessRecorded`：测试迁移 Up 成功（建个临时表），跑完后断言 version 被记录 + 临时表存在。
  - **测试迁移注入机制**：把 `RunMigrations` 的核心循环抽成私有 `runMigrationsList(db *gorm.DB, migrations []Migration) error`（接收迁移切片），`RunMigrations` 调它传入 `migrationsSorted()`。测试直接调 `runMigrationsList(db, []Migration{...测试迁移...})`。通过 `export_test.go`（切片1 已有先例）暴露 `RunMigrationsList = runMigrationsList` 给 `database_test` 包。
  - 验收：3 个测试在改 migrator 前编译失败（`RunOutsideTx` undefined）或运行失败——确认 RED。
  - **⚠️ 实现偏离（留痕）**：`TestRunOutsideTx_NotInTransaction` 的事务检测探针从 `pg_is_in_transaction()` 改为 `CREATE INDEX CONCURRENTLY`。原因：PG18（testcontainer 镜像 `pgvector/pgvector:pg18-trixie`）无 `pg_is_in_transaction()` 函数（SQLSTATE 42883）；`pg_current_xact_id`/`txid_current` 会分配 xact 破坏语义，`pg_stat_activity.state` 查询时恒为 active。`CREATE INDEX CONCURRENTLY` 在事务块内必报 SQLSTATE 25001，块外成功——这正是 D-High-3 要解锁的能力本身，语义最准确。测试注释已写明决策。
- [x] 1.2 **[GREEN]** `backend-go/internal/platform/database/migrator.go`：
  - `Migration` 结构体（:14-18）加 `RunOutsideTx bool` 字段（零值 false）。
  - 抽取 `runMigrationsList(db *gorm.DB, migrations []Migration) error`：包含 `ensureSchemaMigrationsTable` + `loadAppliedMigrationVersions` + 遍历循环；`RunMigrations` 改为调它。
  - 循环内分路径：`RunOutsideTx == false`（默认，现有事务路径不变）/ `RunOutsideTx == true`（先 `Up(db)` 无事务，成功后单独 `INSERT schema_migrations`；Up 失败直接 return err，不记录 version）。见 design.md D1。
  - `export_test.go` 加 `var RunMigrationsList = runMigrationsList`（若现有 export_test.go 无此行）。
  - 验收：1.1 的 3 个测试全 PASS。
- [x] 1.3 **[REFACTOR]** 检查抽取后的 `runMigrationsList` 与原 `RunMigrations` 行为完全一致（事务路径老迁移无变化）。验收：现有 `destructive_guard_test.go` / `constraints_test.go` / `db_unit_test.go` 全 PASS（回归无破坏）。

## 2. 后端：D-Med-7 Migration 加 Down 字段 + 破坏性迁移标注（B · db-migration-execution）

- [x] 2.1 `backend-go/internal/platform/database/migrator.go`：`Migration` 结构体加 `Down func(db *gorm.DB) error` 字段（nil = 不可逆），注释说明「当前仅作声明性占位，执行器尚未实现」。验收：`go build ./...` 通过（加字段零值兼容，无破坏）。
- [x] 2.2 `backend-go/internal/platform/database/postgres_migrations.go`：3 条破坏性迁移的 `Description` 追加不可逆标注：
  - `20260706_0001`（约 :1058）：末尾加「 ⚠️ 不可逆 TRUNCATE（破坏性，受 `MIGRATIONS_ALLOW_DESTRUCTIVE` 守卫；生产执行前需备份）」
  - `20260712_0001`（约 :1096）、`20260718_0001`（约 :1162）：同模板。
  - 验收：`db_unit_test.go` 若有 Description 关键词断言，确认仍 PASS（标注是追加不是替换）；`grep "不可逆 TRUNCATE" postgres_migrations.go` 返回 3 处。

## 3. 后端：D-High-4（迁移层）withLockTimeout helper + 3 处守卫（C · db-migration-execution）

> TDD：先写 helper 测试，再实现 helper，最后给迁移加守卫。

- [x] 3.1 **[RED]** 新增 `backend-go/internal/platform/database/lock_timeout_test.go`（`database_test` 包，testcontainer 集成）：
  - `TestWithLockTimeout_SetsGUC`：在事务内调 `withLockTimeout(db, "3s", func(tx){ var lt string; tx.Raw("SHOW lock_timeout").Scan(&lt); require.Equal("3s", lt) })`，断言 GUC 生效。
  - `TestWithLockTimeout_ResetAfterCall`：helper 返回后（同事务内）`SHOW lock_timeout` 回到 DEFAULT（验证显式复位）。
  - 通过 `export_test.go` 暴露 `WithLockTimeout = withLockTimeout`。
  - 验收：测试在实现 helper 前编译失败——确认 RED。
- [x] 3.2 **[GREEN]** `backend-go/internal/platform/database/postgres_migrations.go`（与 `ensureNotNullDefault` 同位置，约 :54 后）：新增 `withLockTimeout(db *gorm.DB, timeout string, fn func(*gorm.DB) error) error`。实现见 design.md D3（SET LOCAL lock_timeout → fn → SET LOCAL lock_timeout = DEFAULT 复位；`//nosec G201` 注释说明 timeout 为硬编码常量）。`export_test.go` 加 `var WithLockTimeout = withLockTimeout`。验收：3.1 的 2 个测试 PASS。
  - **⚠️ 实现偏离（留痕）**：`//nosec G201` 从行首注释改为内联块注释 `/* #nosec G201 */`。原因：gocritic `commentFormatting` 要求 `// nosec`（带空格），gosec 指令必须 `//nosec`（无空格），二者冲突。改用项目既有先例 `daily_report_models.go:299` 的内联块注释形式，golangci-lint 0 issues。
- [x] 3.3 给 3 处迁移语句加 `withLockTimeout` 守卫：
  - `20260403_0003`（约 :92）：`ALTER COLUMN embedding TYPE vector(4096)` 包进 `withLockTimeout(db, "5s", func(tx){ tx.Exec("ALTER TABLE ... TYPE vector(4096)") })`。
  - `20260603_0001`（约 :595-598）：`ADD CONSTRAINT uq_section_relations_pair UNIQUE` 包进 `withLockTimeout(db, "5s", ...)`。
  - `20260620_0001`（约 :867-870）：扩展 UNIQUE 约束包进 `withLockTimeout(db, "5s", ...)`。
  - 验收：`grep -c "withLockTimeout" postgres_migrations.go` ≥ 4（1 定义 + 3 调用）；testcontainer 全量迁移（`runTestMigrations`）无 error（守卫不影响成功路径）。

## 4. 文档（doc-impact: standard 必更，database/architecture 可选）

> **无 flow 影响**：本 change 为纯 `platform/database` 迁移执行器框架基建，无 API/schema/配置变化（见 proposal「Impact」节），不触及任何业务 flow，免 §12.2 变更溯源。

- [x] 4.1 **[必更]** `docs/reference/standard/backend/code-style.md`：「GORM model tag 与迁移」节（约 :49）后补「迁移编写规范」子节，含 3 条约定：
  - **何时用 `RunOutsideTx: true`**：仅用于单条事务不兼容 DDL（`CREATE INDEX CONCURRENTLY`），多步操作必须留在事务内。
  - **`Down` 字段的声明性用法**：nil = 不可逆；破坏性迁移 Description 必须标注「⚠️ 不可逆」。
  - **`withLockTimeout` 用于长锁 DDL**：`ALTER COLUMN TYPE` / `ADD CONSTRAINT UNIQUE` 必须用守卫，避免大表无限阻塞；timeout 默认 "5s"。
  - 验收：`grep -A2 "迁移编写规范\|RunOutsideTx\|withLockTimeout" code-style.md` 有内容。
- [x] 4.2 **[可选]** `docs/reference/database/_index.md`：在迁移真相源说明（约 :5）后补一句迁移执行器能力（`RunOutsideTx`/`Down`/`withLockTimeout`）。验收：若自然则补，不强制。
- [x] 4.3 **[可选]** `docs/reference/architecture/map.md`：`platform/database` 行（约 :43）补一句能力扩展。验收：若自然则补，不强制。（实际补充：为通过 doc-impact verify architecture 域对账，补了能力指向 + 链接到 code-style.md 规范）
- [x] 4.4 `docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` + `README.md`：标记 D-High-3 / D-Med-7 / D-High-4（迁移层部分）为 ✅ 已修复（切片2a）；D-High-1 / D-High-5 / M-High-2 / 运行时 lock_timeout 留 ⏳ 切片2b。验收：issue 状态表更新。

## 5. 归档门禁（开发执行规范 §11）

### 测试

- [x] 5.1 `cd backend-go && go test ./internal/platform/database ./internal/platform/config ./internal/platform/testutil` — 全 PASS（含新增 outside_tx_test.go / lock_timeout_test.go，需 Docker testcontainer）。实测：database 10.3s / config 0.1s / testutil 30.4s
- [x] 5.2 `cd backend-go && go test -short ./internal/platform/database` — 单元测试 PASS（testcontainer 集成测试在 -short 下 skip）。实测：1.348s ok
- [x] 5.3 `cd backend-go && go test ./internal/platform/database -run 'TestRunOutsideTx|TestWithLockTimeout|TestDestructive|TestModelTagConstraints'` — 新增 + 回归测试全 PASS。实测：7/7 PASS（TestRunOutsideTx×3 + TestWithLockTimeout×2 + TestDestructive×2 + TestModelTagConstraints×1，在 pgvector/pgvector:pg18-trixie testcontainer 上）

### 文档

- [x] 5.4 `bash scripts/doc-impact.sh verify openspec/changes/db-ddl-hardening-architecture/` — 0 FAIL（standard 域已更新；database/architecture 域若声明则必须有对应文件）。实测：通过（声明: database, standard, architecture 文件:4 个）
- [x] 5.5 `bash scripts/check-standards.sh` — 全 PASS（A-D 段 + F 段 doc-impact + G 段死链 + H 段 model tag 守门）。实测：本 change 相关全绿（F 段 db-ddl-hardening-architecture [OK]）；check-standards 整体 72 通过 / 2 失败，2 个 FAIL 均为**其它 change 的历史遗留**（E 段 `2026-07-24-db-ddl-hardening-low-risk` 切片1 误声明 flow 域未补溯源；F 段 `otel-tracing-completion` doc-impact），与本切片2a 无关

### 验证

- [x] 5.6 `cd backend-go && golangci-lint run ./...` — 0 issues（含 `/* #nosec G201 */` 注释正确）。实测：0 issues
- [x] 5.7 `cd backend-go && go vet ./... && go build ./...` — 无错误。实测：输出空，BUILD OK
- [x] 5.8 `cd backend-go && grep -c "withLockTimeout" internal/platform/database/postgres_migrations.go` — ≥ 4（1 定义 + 3 调用）。实测：5（1 定义 + 3 调用 + 1 docstring 文本提及，代码调用点 = 4 符合语义）
- [x] 5.9 `cd backend-go && grep -c "不可逆 TRUNCATE" internal/platform/database/postgres_migrations.go` — = 3（3 条破坏性迁移标注）。实测：3
- [x] 5.10 `cd backend-go && go test ./internal/platform/database -run TestRunOutsideTx_SuccessRecorded -count=2` — 连续两次 PASS（验证事务外迁移成功记录的可重试性）。实测：连续两次 PASS

## 部署后影响 + 需要的操作（AGENTS.md 强制项）

- **(a) 用户可见行为**：**无变化**。切片2a 只加框架能力，所有现有迁移执行路径（事务内）完全不变。`RunOutsideTx` 全为 false，`Down` 全为 nil，`withLockTimeout` 只被 3 处迁移调用（加守卫不影响成功路径，仅在锁竞争超 5s 时让该语句失败而非无限阻塞——纯安全收紧）。
- **(b) 需要的操作**：**无**。无新 env、无 schema 变化、无配置变化。正常部署即可。
- **(c) 旧数据降级**：**无影响**。纯迁移执行器内部改造，不碰业务数据。
