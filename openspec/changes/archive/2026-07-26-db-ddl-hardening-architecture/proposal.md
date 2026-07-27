## Why

`docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` DDL 专项审计的架构级整改。切片1 `db-ddl-hardening-low-risk`（已归档 2026-07-24）已清掉低风险机械项（破坏性迁移守卫 / SET NOT NULL 幂等 / tag 剥离 / 假注释）。本 change 收口**剩余的架构级根因**，按用户决策**再拆 2 个切片**：

- **切片2a（本 change 当前范围）**：migrator 框架基建——只搭能力，不改应用层迁移逻辑。
- **切片2b（后续 change）**：应用层改造——依赖切片2a 的能力（CONCURRENTLY 索引 / 向量维度统一 / tag 长尾收口 / 运行时 ensurer lock_timeout 统一）。

切片2a 解决三个根因：

1. **D-High-3（根因）**：`migrator.go:114` 把每条迁移包在 `db.Transaction(func(tx){ Up(tx); INSERT schema_migrations })`，而 PostgreSQL 不允许 `CREATE INDEX CONCURRENTLY` 在事务内执行——这是全库 33 处索引只能用阻塞式 `CREATE INDEX`、大表（`articles` FTS GIN / `ai_call_logs` / `topic_tag_embeddings`）锁表写入的**根因**。切片2b 要把索引改 CONCURRENTLY，必须先让 migrator 支持「事务外执行」。
2. **D-Med-7**：`Migration` 结构体（`migrator.go:14-18`）只有 `Up`，破坏性操作（TRUNCATE/DROP）一旦执行无法回退。报告建议「至少为破坏性迁移补 down 或明确「不可逆」标注」。与 D-High-3 的结构体改造同期做，机会成本最低。
3. **D-High-4（迁移层部分）**：迁移文件内的 `ALTER COLUMN TYPE vector(N)`（全表重写）、`ADD CONSTRAINT UNIQUE`（扫全表）长时间 AccessExclusiveLock，**无 lock_timeout 保护**。复用 `daily_report_models.go:299` 的 5s `SET LOCAL lock_timeout` 先例，抽象成可复用 helper 并给 3 处高危迁移加守卫。

**用户决策（已确认）**：
- ✅ 再拆 2 个切片（而非报告默认的「一个 change 全收口」）
- ✅ 高风险 3 项（D-High-6 向量索引方案 / D-High-2 外键 / D-Med-3 CASCADE 语义）**不纳入本 change**，留独立决策

## What Changes

### A. D-High-3：migrator 支持事务外迁移 —— 修改 `db-migration-execution` capability
- `Migration` 结构体加 `RunOutsideTx bool` 字段（零值 false，老迁移行为完全不变）。
- `RunMigrations` 循环分两条路径：
  - `RunOutsideTx == false`（默认）：走现有事务路径（`db.Transaction(Up(tx) + INSERT schema_migrations)`，不变）。
  - `RunOutsideTx == true`：先 `Up(db)`（无外层事务，支持 `CONCURRENTLY`），成功后单独 `INSERT INTO schema_migrations`（直接 `db.Exec`，不在事务内）；**Up 失败则不记录 version**，下次启动重试。
- **切片2a 只搭框架，不改任何现有迁移的 `RunOutsideTx`**（全为 false）。切片2b 才把 CONCURRENTLY 索引迁移改为 `RunOutsideTx: true`。

### B. D-Med-7：Migration 加 Down 字段 —— 修改 `db-migration-execution` capability
- `Migration` 结构体加 `Down func(db *gorm.DB) error` 字段（nil = 不可逆）。
- **不实现 down 执行器**（不接 CLI/HTTP 入口）——纯结构体就位，未来按需补 `RunDownMigrations`。避免为不存在需求写代码（AGENTS.md「Simplicity First」）。
- 为 3 条破坏性迁移（`20260706_0001`/`20260712_0001`/`20260718_0001`）的 `Description` 追加「⚠️ 不可逆，生产执行前需备份」标注。不为它们写实际 Down 实现——TRUNCATE 数据物理不可恢复，写"恢复 DDL"是假象。

### C. D-High-4（迁移层部分）：lock_timeout 守卫 helper + 3 处迁移加守卫 —— 修改 `db-migration-execution` capability
- `postgres_migrations.go` 新增 helper（复用 `daily_report_models.go:299` 先例，参数化 + 适配事务内语义）：
  ```go
  // withLockTimeout 在 lock_timeout 守卫内执行 fn。
  // 用于 ALTER TYPE / ADD CONSTRAINT 等长锁操作，避免大表被无限阻塞。
  // SET LOCAL 在事务内有效（迁移默认都在事务内跑），事务结束自动复位。
  func withLockTimeout(db *gorm.DB, timeout string, fn func(*gorm.DB) error) error
  ```
- 给 3 处迁移语句加守卫：
  - `20260403_0003:92`（`ALTER COLUMN embedding TYPE vector(4096)`）
  - `20260603_0001:595-598`（`ADD CONSTRAINT uq_section_relations_pair UNIQUE`）
  - `20260620_0001:867-870`（扩展的 UNIQUE 约束）
- **运行时向量 ensurer 的 lock_timeout**（`embedding.go:ensureVectorDimension` 无守卫 + 其它 3 处统一）→ **留切片2b**（跟「3 套向量逻辑统一」一起做）。

## Capabilities

### Modified Capabilities
- `db-migration-execution`（切片1 的 `db-migration-safety` 兄弟能力）：数据库迁移执行器的框架级行为约束——
  - **事务外执行能力**：迁移可声明 `RunOutsideTx: true` 脱离外层事务（为 `CREATE INDEX CONCURRENTLY` 等事务不兼容操作解锁）；事务外迁移 Up 失败不记录 version（保证可重试），成功才记录。
  - **回滚声明能力**：迁移可声明 `Down`（nil = 不可逆）；破坏性迁移的 Description 必须标注不可逆性。
  - **锁表保护**：长锁 DDL（`ALTER TYPE` / `ADD CONSTRAINT UNIQUE`）必须用 `withLockTimeout` 守卫，避免大表无限阻塞。

> capability 命名说明：切片1 已建 `db-migration-safety`（破坏性守卫 + 幂等性）。本切片建的是执行器框架能力（事务外执行 / 回滚 / 锁保护），语义不同于「安全守卫」，单列为 `db-migration-execution`。

## Impact

- **代码**：
  - `backend-go/internal/platform/database/migrator.go`（`Migration` 结构体加 `RunOutsideTx`/`Down` + `RunMigrations` 分路径 + helper 暴露）
  - `backend-go/internal/platform/database/postgres_migrations.go`（新增 `withLockTimeout` helper + 3 处迁移加守卫 + 3 条破坏性迁移 Description 标注不可逆）
- **测试**（新增，仿 `destructive_guard_test.go` 的 `runMigrationsWithGate` + `testutil.OpenTestDB` 模式）：
  - `outside_tx_test.go`：`RunOutsideTx` 三路径（不在事务内 / 失败不记录 / 成功记录）
  - `lock_timeout_test.go`：`withLockTimeout` SET LOCAL 语义（GUC 生效 + 事务后复位）
- **文档**（doc-impact: `database, standard, architecture`）：
  - `docs/reference/standard/backend/code-style.md`「GORM model tag 与迁移」节后补「迁移编写规范」子节（`RunOutsideTx` / `Down` / `withLockTimeout` 的使用时机）
  - `docs/reference/database/_index.md`（迁移执行器机制说明，可选——当前只列模型清单，可补一句框架能力）
  - `docs/reference/architecture/map.md`（可选——`platform/database` 平台层能力扩展说明）
- **无 API 变化** / **无 schema 变化** / **无配置变化**（无新 env）
- **风险**：低。
  - `RunOutsideTx`/`Down` 字段零值兼容，老迁移行为完全不变（全为 false/nil）。
  - `withLockTimeout` 只给 3 处迁移加，成功路径不受影响（仅在锁竞争超 5s 时让该语句失败而非无限阻塞——纯安全收紧）。
  - 测试覆盖 testcontainer 全路径，仿切片1 成熟模式。

## 部署后影响 + 需要的操作（AGENTS.md 强制项）

- **(a) 用户可见行为**：**无变化**。切片2a 只加框架能力，所有现有迁移的执行路径（事务内）完全不变。`RunOutsideTx` 全为 false，`Down` 全为 nil，`withLockTimeout` 只被 3 处迁移调用。
- **(b) 需要的操作**：**无**。无新 env、无 schema 变化、无配置变化。正常部署即可。
- **(c) 旧数据降级**：**无影响**。纯迁移执行器内部改造，不碰业务数据。

## 不做（留切片2b 或独立决策）

- ❌ D-High-1 大表索引改 CONCURRENTLY（依赖切片2a 的 `RunOutsideTx`）
- ❌ D-High-5 + M-High-1 向量维度三方矛盾（迁移改无维度列 + tag 对齐）
- ❌ 运行时向量 ensurer 的 4 处 lock_timeout + 3 套逻辑统一（跨 tagmanagement/topicgraph/auxlabel 3 包，与切片2b 的向量逻辑统一强相关）
- ❌ M-High-2 补 13 文件 NOT NULL/DEFAULT 收敛迁移
- ❌ D-High-6（>2000 维向量索引方案）、D-High-2（外键）、D-Med-3（CASCADE 语义）—— 用户已确认不纳入本 change
