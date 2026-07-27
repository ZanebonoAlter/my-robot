# Design: db-ddl-hardening-architecture（切片2a — migrator 框架基建）

> 切片2a 只搭 migrator 框架能力，不改应用层迁移逻辑。切片2b（CONCURRENTLY 索引 / 向量维度统一 / tag 收口 / 运行时 ensurer lock_timeout）依赖本切片的 `RunOutsideTx` 能力。
>
> 来源：`docs/issues/02-quality-audit-2026-07-23/05-db-ddl.md` D-High-3（根因）/ D-Med-7 / D-High-4（迁移层部分）。

## 背景

切片1 `db-ddl-hardening-low-risk`（已归档 2026-07-24）清掉了低风险机械项。本切片处理架构级根因，关键事实（来自探索）：

| 事实 | 位置 | 影响 |
| ---- | ---- | ---- |
| `Migration` 结构体只有 `Version/Description/Up` | `migrator.go:14-18` | 无法声明事务外执行 / 回滚 |
| `RunMigrations` 把每条迁移包在 `db.Transaction` | `migrator.go:114-129` | `CREATE INDEX CONCURRENTLY` 在事务内必报错——全库 33 处索引只能阻塞式 |
| Up 失败 → 整个事务回滚 → `schema_migrations` INSERT 也回滚 | `migrator.go:114-127` | 失败迁移下次启动会重试（现有语义正确，事务外路径要对齐） |
| `daily_report_models.go:299` 有 5s `SET LOCAL lock_timeout` 先例 | 运行时 ensurer | 迁移层无此守卫，抽象成可复用 helper |
| 切片1 测试模式：`runMigrationsWithGate` + `testutil.OpenTestDB` 手动控 config | `destructive_guard_test.go:37-56` | 本切片测试的完美模板 |

## 决策

### D1. `RunOutsideTx` 语义：失败不记录 version，成功才记录（与现有事务路径对齐）

**选项**：
- (a) 事务外迁移 Up 失败仍记录 version（标记 failed，下次跳过）
- (b) 事务外迁移 Up 失败**不记录** version（下次启动重试），成功才记录

**选 (b)**。理由：
1. **与现有事务路径语义一致**——`migrator.go:114-127` 事务内 Up 失败会回滚整个事务（含 INSERT），现有迁移失败下次启动就重试。事务外路径必须对齐这个不变量，否则「迁移失败」在不同路径下行为分叉，调试困难。
2. **CONCURRENTLY 的失败模式需要重试**——`CREATE INDEX CONCURRENTLY` 可能因锁竞争或死锁失败并留下 INVALID 索引。下次启动应重试（切片2b 的迁移闭包会先 `DROP INDEX IF EXISTS` 清理 INVALID 残留再重建）。若标记 failed 跳过，INVALID 索引会永久残留。
3. **幂等性兜底**——PG 不允许跨事务的原子「DDL + 记录」，但迁移本身必须幂等（切片1 已建立的规范：`IF NOT EXISTS` / `ensureNotNullDefault` 守卫）。重试安全靠迁移自身幂等，不靠记录机制。

**实现**（`migrator.go` `RunMigrations` 循环）：
```go
for _, migration := range migrationsSorted() {
    if appliedVersions[migration.Version] {
        continue
    }
    if migration.RunOutsideTx {
        // 事务外路径：支持 CREATE INDEX CONCURRENTLY 等事务不兼容操作。
        // Up 失败不记录 version（下次启动重试）；成功才记录。
        if err := migration.Up(db); err != nil {
            return fmt.Errorf("apply migration %s (outside tx): %w", migration.Version, err)
        }
        if err := db.Exec(
            "INSERT INTO schema_migrations (version, driver) VALUES (?, 'postgres')",
            migration.Version,
        ).Error; err != nil {
            return fmt.Errorf("record migration %s: %w", migration.Version, err)
        }
    } else {
        // 事务内路径（现有，不变）：Up + INSERT 原子提交/回滚。
        if err := db.Transaction(func(tx *gorm.DB) error { ... }); err != nil {
            return err
        }
    }
}
```

**边界——CONCURRENTLY 的 INVALID 索引残留**：切片2a 只保证「Up 失败不记录 version」的框架语义。CONCURRENTLY 失败留下的 INVALID 索引清理，由切片2b 的迁移闭包负责（`DROP INDEX IF EXISTS` 守卫）——这是应用层职责，不是执行器职责。本切片的测试只验证「失败不记录 + 可重试」语义，不验证 INVALID 索引清理（那是切片2b 的迁移行为）。

### D2. `Down` 字段：只加结构体，不实现执行器

**选项**：
- (a) 加 `Down` 字段 + 实现 `RunDownMigrations` 执行器（接 CLI/HTTP）
- (b) 只加 `Down` 字段（nil = 不可逆），不实现执行器
- (c) 切片2a 不做 D-Med-7，拆到独立 change

**选 (b)**。理由：
1. **AGENTS.md「Simplicity First」**——「No features beyond what was asked」「No abstractions for single-use code」。当前无任何回滚执行入口需求（无 CLI、无 HTTP 回滚端点），写执行器是为不存在的需求写代码。
2. **报告建议的落地形态**——D-Med-7 原文「至少为破坏性迁移补 down 迁移**或明确「不可逆」标注** + 备份要求」。标注是合规形态，不需要执行器。
3. **机会成本**——与 D-High-3 结构体改造同期加一个字段，边际成本接近零；若拆到独立 change，反而要重读结构体上下文。
4. **TRUNCATE 物理不可恢复**——为 3 条破坏性迁移写"恢复 DDL"是假象（TRUNCATE 的数据没了就是没了），写 `Down` 实现反而误导。只标 `Description`「⚠️ 不可逆」是诚实做法。

**实现**：
```go
type Migration struct {
    Version     string
    Description string
    Up          func(db *gorm.DB) error
    // RunOutsideTx 控制 Up 是否脱离外层事务执行（默认 false）。
    // 设 true 支持 CREATE INDEX CONCURRENTLY 等事务不兼容操作。
    RunOutsideTx bool
    // Down 声明迁移的回滚逻辑（nil = 不可逆）。当前仅作声明性占位，
    // 执行器尚未实现；未来按需补 RunDownMigrations。
    Down func(db *gorm.DB) error
}
```

3 条破坏性迁移的 `Description` 追加（不改 Up 逻辑）：
- `20260706_0001`：「⚠️ 不可逆 TRUNCATE（破坏性，受 `MIGRATIONS_ALLOW_DESTRUCTIVE` 守卫；生产执行前需备份）」
- `20260712_0001` / `20260718_0001`：同模板。

### D3. `withLockTimeout` helper：SET LOCAL 事务内语义，复用 daily_report 先例

**选项**：
- (a) 迁移执行器统一注入 `SET LOCAL lock_timeout`（所有迁移都受保护）
- (b) 新增 `withLockTimeout(db, timeout, fn)` helper，由长锁迁移显式调用
- (c) 不做 helper，每个需要的地方内联 `db.Exec("SET LOCAL lock_timeout...")`

**选 (b)**。理由：
1. **(a) 过度**——绝大多数迁移是轻量 DDL（`CREATE INDEX IF NOT EXISTS` 在小表上瞬间完成），统一注入会让短操作也受 5s 限制，反而误杀正常路径。(a) 还隐藏了"哪个迁移需要长锁"的信息。
2. **(c) 重复**——内联 `SET LOCAL` 会复制 daily_report_models.go:299 的模式，且无法保证"SET LOCAL + ALTER + 隐式复位"的三步完整性（作者可能漏写或顺序错）。
3. **(b) 平衡**——helper 把"SET LOCAL lock_timeout → 执行 fn → SET LOCAL lock_timeout = DEFAULT 复位"封装成原子单元，调用方一行 `withLockTimeout(db, "5s", func(tx){ tx.Exec("ALTER ...") })`。显式调用 = 显式声明"这条是长锁 DDL"，自文档化。

**实现**（`postgres_migrations.go`，与 `ensureNotNullDefault` 同位置）：
```go
// withLockTimeout 在 lock_timeout 守卫内执行 fn，完成后复位 lock_timeout。
// 用于 ALTER COLUMN TYPE / ADD CONSTRAINT 等长锁 DDL，避免大表被无限阻塞。
// timeout 形如 "5s" / "10s"；用 SET LOCAL（事务内有效，事务结束自动复位）。
// 复用 daily_report_models.go:299 的先例，但参数化 + 保证复位（防御 SET LOCAL 泄漏）。
//
//nosec G201 — timeout 是硬编码常量字符串，非外部输入。
func withLockTimeout(db *gorm.DB, timeout string, fn func(*gorm.DB) error) error {
    if err := db.Exec(fmt.Sprintf("SET LOCAL lock_timeout = %q", timeout)).Error; err != nil {
        return fmt.Errorf("set lock_timeout=%s: %w", timeout, err)
    }
    if err := fn(db); err != nil {
        return err
    }
    // 显式复位（虽然 SET LOCAL 事务结束自动复位，但防御性复位避免连接池复用时的 GUC 泄漏）。
    _ = db.Exec("SET LOCAL lock_timeout = DEFAULT").Error
    return nil
}
```

**SET LOCAL 事务语义验证**（测试要钉死）：
- 迁移默认都在事务内跑（`migrator.go:114`），`SET LOCAL` 在事务结束自动复位——这是 PG 语义。
- 但运行时 ensurer（如 daily_report_models.go）在裸 `db` 上跑（无显式事务），`SET LOCAL` 行为依赖 GORM 连接池是否复用同一连接。helper 显式 `SET LOCAL lock_timeout = DEFAULT` 复位是防御性措施（虽然迁移路径有事务包裹，理论上不需要，但 helper 未来也可能被运行时路径复用）。

### D4. 3 处迁移加守卫的目标选择

| 位置 | 迁移 | 操作 | 为何需要守卫 |
| ---- | ---- | ---- | ---- |
| `postgres_migrations.go:92` | `20260403_0003` | `ALTER COLUMN embedding TYPE vector(4096)` | 全表重写，AccessExclusiveLock |
| `postgres_migrations.go:595-598` | `20260603_0001` | `ADD CONSTRAINT uq_section_relations_pair UNIQUE` | 扫全表验证唯一性 + 锁 |
| `postgres_migrations.go:867-870` | `20260620_0001` | 扩展 UNIQUE 约束（加 relation_type） | 同上，且是替换性重建 |

**不加守卫的（已幂等或轻量）**：
- 3 处 CHECK 约束（`:767`/`:967`/`:1045`）——已用 `DO $$ ... IF NOT EXISTS ... END $$` 守卫，且 CHECK 不扫全表。
- FK `topic_tags_merged_into_id_fkey`（`:556`）——单行级联，锁轻。
- 其余 `CREATE INDEX`（33 处）——切片2b 改 CONCURRENTLY 时统一处理（依赖 D-High-3），本切片不动。

### D5. 文档更新边界

| 文档 | 是否更新 | 内容 |
| ---- | ---- | ---- |
| `standard/backend/code-style.md` | ✅ 更新 | 「GORM model tag 与迁移」节后补「迁移编写规范」子节：何时用 `RunOutsideTx`（CONCURRENTLY）、`Down` 的声明性用法、`withLockTimeout` 用于长锁 DDL |
| `database/_index.md` | ⚠️ 可选 | 当前只列模型清单，可补一句迁移执行器能力说明（非强制，看实现后是否自然） |
| `architecture/map.md` | ⚠️ 可选 | `platform/database` 平台层能力扩展（非强制） |
| `configuration.md` | ❌ 不动 | 无新 env |
| `deployment.md` | ❌ 不动 | 无部署行为变化 |

> standard 域是必更——`withLockTimeout` 和 `RunOutsideTx` 是新的迁移编写约定，必须落到规范让未来迁移作者知道何时用（否则 D-High-4 的守卫模式不会传播，新迁移还会裸 ALTER）。

## 测试设计（仿切片1 成熟模式）

### `outside_tx_test.go`（testcontainer 集成，`database_test` 包）

仿 `destructive_guard_test.go` 的 `runMigrationsWithGate` 模式，但需要**注入测试专用迁移**。机制：
- `postgresMigrations()` 返回的切片是私有的，测试无法直接追加。
- 方案：测试用 `database_test` 包的 `export_test.go` 模式（切片1 已有先例：`ExportedRunAuxLabelDupMerge`）暴露一个 hook，或测试直接构造 `[]Migration` 调用一个可测的内部函数。
- **更简洁的方案**：把 `RunMigrations` 的核心循环抽成 `runMigrationsList(db, []Migration, appliedVersions)`（私有），测试直接调它传入含 `RunOutsideTx: true` 的测试迁移。这样不动 `postgresMigrations()`，测试完全隔离。

三个测试：
1. **`TestRunOutsideTx_NotInTransaction`**：测试迁移 Up 闭包内 `SELECT pg_is_in_transaction()` 返回 false（证明真的脱离了外层事务）。
2. **`TestRunOutsideTx_FailureNotRecorded`**：Up 返回 error → `schema_migrations` 无该 version（下次可重试）。
3. **`TestRunOutsideTx_SuccessRecorded`**：Up 成功 → version 被记录。

### `lock_timeout_test.go`（testcontainer 集成）

1. **`TestWithLockTimeout_SetsGUC`**：在事务内调 `withLockTimeout(db, "3s", ...)`，fn 内 `SHOW lock_timeout` 返回 `3s`。
2. **`TestWithLockTimeout_ResetAfterCall`**：helper 返回后（同事务内）`SHOW lock_timeout` 回到 DEFAULT（验证显式复位）。

## 风险与回滚

- **`RunOutsideTx` 误用**：若未来迁移错误声明 `RunOutsideTx: true` 但 Up 里有需要原子性的多步操作（如 ALTER + UPDATE + DROP），失败会留中间状态。缓解：standard/code-style.md 规范明确「`RunOutsideTx: true` 仅用于单条事务不兼容 DDL（CONCURRENTLY），多步操作必须留在事务内」。
- **`withLockTimeout` 误杀正常路径**：5s 对小表足够，但若大表 ALTER 在 5s 内完不成会失败。缓解：这是有意的安全收紧（报告 D-High-4 明确要求「避免大表被无限阻塞」），失败比无限锁表好；切片2b 改 CONCURRENTLY 后大表 ALTER 路径会减少。
- **回滚**：纯加法（结构体字段 + helper + 3 处守卫 + 标注），`git revert` 即可，无数据迁移不可逆风险。3 处迁移加守卫不影响成功路径（只在超时时失败）。

## 不做（留切片2b 或独立决策）

- D-High-1 大表索引改 CONCURRENTLY（依赖本切片 `RunOutsideTx`）
- D-High-5 + M-High-1 向量维度三方矛盾
- 运行时向量 ensurer 4 处 lock_timeout + 3 套逻辑统一（跨 tagmanagement/topicgraph/auxlabel 3 包）
- M-High-2 补 13 文件 NOT NULL/DEFAULT 收敛迁移
- D-High-6（>2000 维向量索引方案）、D-High-2（外键）、D-Med-3（CASCADE 语义）—— 用户已确认不纳入
