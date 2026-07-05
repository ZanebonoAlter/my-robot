## Context

后端集成测试反馈过慢：`go test ./internal/topicgraph/repository ./internal/topicgraph/service` 约 355 秒，其中 `topicgraph/repository` 一个包占 300+ 秒。

根因（已在 proposal 核实，非猜测）：容器**已经是单例**（`testutil.go` 用 `startOnce sync.Once`，每进程只起一个 pgvector 容器）。慢不在容器启动，而在 `SetupTestDB` 的数据重置策略——每次调用都触发 `ReimportTestDB`：`DROP SCHEMA public CASCADE` → `CREATE SCHEMA` → `runTestMigrations`（AutoMigrate 全部 28 个模型 + 全部版本化迁移）→ 因 pgvector `vector` 类型 oid 漂移而重开连接池。`topicgraph/repository` 有 41 个调用点，每次重置约 8 秒。

现状约束（关键事实，影响选型）：

- `topicgraph/repository` 的 41 个集成测试**不调用任何 pgvector 算子**（无 `<->` `<=>` cosine_distance），vector 相关代码全是字段填充（`Embedding: FloatsToPgVector([]float64{0})`），仅为满足 `vector(4096)` 列约束能 INSERT。
- 但这些测试全是日报**聚合 / 分组 / 血缘 / raw SQL** 查询，命中 [`standard/backend/testing.md`](docs/reference/standard/backend/testing.md) §集成测试常见陷阱的硬约束："改了发往真实 DB 的 SQL（GROUP BY/聚合/复杂 JOIN/raw SQL），必须有 testcontainer PG 用例真跑"。回退 SQLite 会重蹈 `quality-scoring-observability` 的 GROUP BY 生产事故。
- 版本化迁移注入的 seed 数据**仅落两张表**：`ai_settings` 与 `embedding_config`（已核实 `postgres_migrations.go` 975 行，所有 `INSERT/Create` seed 目标均为这两张表）。
- 现有 `TruncateAllTables`（CASCADE 快速清表）**调用点 = 0**，从未被真正使用；proposal "测试没走它"成立，但这函数本身待激活。
- OID 漂移修复机制（`ReimportTestDB` 末尾 `openGorm`+`Close`）有回归测试 `TestReimportPreservesVectorInserts` 守卫，[`standard/backend/testing.md`](docs/reference/standard/backend/testing.md) 明令禁止删除。

## Goals / Non-Goals

**Goals:**

- 把 `topicgraph/repository` 集成测试反馈时间从 ~355s 显著降低。**实测达成：382s → 147s（~2.6x）**。最初目标 ≤090s 未完全达成——代价来自决策⑥的「每次 reset 重开连接池」（见 Risks / 未来优化）。
- 测试调用点**零改动**：41 个 `db := testutil.SetupTestDB(t)` 调用一行都不动，靠 `testutil` 内部行为切换提速。
- 测试语义不变：每个测试仍拿到"与生产首次启动一致的干净 schema + seed"。
- 保留所有现有安全红线（§6.2 DSN 红线）和回归守卫。

**Non-Goals:**

- 不重写 testcontainer 基建（保留单例容器 + Ryuk 清理）。
- 不改版本化迁移的内容。
- 不把 `topicgraph/repository` 测试回退 SQLite（被项目标准否决，见 Context）。
- 不引入测试并行化（与共享 schema 模型冲突，且当前本就串行）。
- 不改任何生产代码、不改迁移本身、不改测试断言语义。

## Decisions

### 决策① 主方向：进程级一次性黄金 schema + 测试间 truncate

**选择**：方案 A——进程级只跑一次 `runTestMigrations` 建立"黄金 schema"，每个测试用 truncate 清数据而非 DROP+重建。

**rationale**：41 次完整迁移（每次含 AutoMigrate 28 模型 + 全部版本化迁移 + oid 漂移重连）→ 1 次迁移 + 41 次 truncate。

**备选否决**：

- **方向 B（降级回 SQLite）**——proposal 原列"见效最快"，但实测被项目标准否决：`topicgraph/repository` 测试全是日报 GROUP BY/聚合/raw SQL 查询，`standard/backend/testing.md` 硬约束要求这类 SQL 必须有 PG 集成测试真跑，回 SQLite 会重蹈 GROUP BY 生产事故。B 解不了 355s 主矛盾，最多优化纯 CRUD 包。
- **方向 C（快照 schema 克隆）**——实现复杂、收益不如 A，优先级最低。
- **方向 D（测试并行化）**——与共享黄金 schema 冲突（需每测试独立 schema/容器），复杂度高，且当前本就串行，无损失。

### 决策② 隔离模型：共享黄金 schema，放弃并行

**选择**：一个进程一张黄金 schema，测试串行轮流使用，测试间用 truncate 清数据。

**rationale**：单用户项目，串行 60s 远比并行但复杂的多容器/多 schema 方案划算。当前测试本就串行运行，等于无损失。

### 决策③ seed 保护：运行时 seed 快照恢复（非跳过表）

**选择**：黄金 schema 首次构建完成后，立即对迁移 seed 表（`ai_settings`、`embedding_config`）做**运行时快照**——从 DB 读出当时全部行存入内存。`ResetTestData` 的流程为：① `TRUNCATE TABLE ... CASCADE` 清空**所有**业务表（含 `ai_settings`、`embedding_config`，仅跳过 `schema_migrations`），② 从快照把 seed 行灌回 `ai_settings`、`embedding_config`。结果等价于"生产首次启动后的状态"——与现有 `SetupTestDB` 的保证**逐字一致**，测试断言不用动。

**rationale（为何放弃早先的"跳过 seed 表"方案）**：writing-plans 阶段深入核实代码发现，tagmanagement 测试**会往 `ai_settings` 插自定义配置键**（非迁移 seed，共 9 处，如 `semantic_board_match_sim_threshold`、`semantic_board_upgrade_cotag_hard_limit`）。

| | 跳过表方案（早先） | **快照恢复方案（采用）✅** |
|---|---|---|
| 测试插的自定义 ai_settings 键 | **残留**到下个测试 → 跨测试污染 | truncate 清掉，从快照恢复到 seed-only |
| 语义等价现状 ReimportTestDB | ❌ | ✅ |
| tagmanagement 测试改动 | 会红、需尴尬"修复" | 零改动 |

快照在运行时从黄金 schema 的 DB 读取，**不存手工 seed 副本**——seed 内容始终来自生产迁移，不会 drift。多出的代价仅 ~20 行快照/恢复逻辑。

**为何仍跳过 `schema_migrations`**：truncate 清掉迁移版本记录后，若任何路径重跑 `RunMigrations` 会误判"全部未应用"而重跑（虽幂等但无谓耗时且有风险）。

**备选否决**：

- **truncate 后重跑 seed 迁移**：依赖迁移幂等性，复杂度更高，且无法干净区分"seed 迁移"与"DDL 迁移"。
- **测试自建所需 config**：要改大量测试代码，违反 Goals"调用点零改动"。
- **跳过 seed 表（早先方案）**：见上表，会导致 tagmanagement 测试的 ai_settings 自定义键残留。

**实现**：新增 `ResetTestData(t, db)`；`TruncateAllTables`（清空一切，无快照）原义保留，供显式需要"真清空含 seed"的场景调用。

### 决策④ 黄金 schema 构建时机：SetupTestDB 内部 sync.Once，不要求每包加 TestMain

**选择**：在 `testutil` 内部新增一个 `goldenSchemaOnce sync.Once`，`SetupTestDB` 首次调用时跑 `runTestMigrations` 建黄金 schema（并打 `goldenSchemaBuilt` 标记），后续调用检测到已建则走 `ResetTestData`（truncate + seed 快照恢复）。

**rationale**：真正实现"调用点零改动"。若要求每个测试包加 `TestMain` 调 `testutil.InitGoldenSchema()`，则要改 N 个包的入口，违反 Goals。

**并发安全**：Go 测试默认串行（除非显式 `t.Parallel()`），受影响的包当前均未用 `t.Parallel()`，故 `sync.Once` 建黄金 schema + 标记判断在串行下安全。design 阶段需确认受影响包无 `t.Parallel()`（若有则需额外隔离机制）。

### 决策⑤ ReimportTestDB 保留作逃生口

**选择**：`ReimportTestDB` 原样保留，不删除、不改造。少数需要"干净迁移态"的回归测试（如 `TestReimportPreservesVectorInserts`）继续可用。

**rationale**：`standard/backend/testing.md` 明令禁止删除 `ReimportTestDB` 末尾的 `openGorm`+`Close`（修 OID 漂移的核心）。

**附带红利**：方向 A 落地后，黄金 schema 只建一次、从不 DROP/recreate，vector 类型 OID 永不变，OID 漂移 bug 在主路径上**根本不会触发**——等于顺手消灭一整类隐性 bug，而非绕过它。`ReimportTestDB` 退化为"极少数显式回归"才走的逃生口。

### 决策⑥ ResetTestData 必须恢复 vector 列维度 + 重开连接池（Task 3 实现中发现的设计缺口）

**背景**：实现 Task 3 时发现 `ResetTestData`（仅 truncate + 恢复 seed）与旧 `ReimportTestDB`（DROP + 重建 + 重开池）在**两个维度**不等价，导致 `topicgraph/repository` 17 个测试失败：

1. **schema 变异残留**：两个测试对共享 DB 做 `ALTER COLUMN embedding TYPE vector(N)`（`daily_report_topic_lineage_test.go` → `vector(3)`，`daily_report_realdata_test.go` → `vector(2560)`）。旧路径每次 DROP+重建会撤销变异；新路径只 truncate 数据不撤销 → 列维度残留 → 后续测试插入 `[1,0,0]` 报 `expected 2560 dimensions, not 3`。
2. **prepared statement 缓存失效**：ALTER 改列类型使连接池的 prepared statement 失效。旧路径每次重开连接池；新路径复用池 → `cached plan must not change result type (SQLSTATE 0A000)`。

**选择**（Option 1，infra 层）：让 `ResetTestData` 维持"reset 后 = 干净黄金态"不变量：
- 黄金 schema 构建时快照所有 vector 列的类型声明（`pg_catalog.format_type`）。
- `ResetTestData` 每次：truncate 后 re-ALTER 每个 vector 列回黄金类型；恢复 seed；**重开连接池**（黄金 schema 从不 DROP 扩展，OID 稳定，重开只清被 ALTER 作废的 prepared statement）。
- 签名改为返回 `*gorm.DB`（镜像 `ReimportTestDB`），调用方重新赋值。

**rationale**：符合本项目"testutil 保证干净生产启动态"哲学；唯一能修全部 17 个失败的解；不动任何业务/测试文件。

**代价 / 未来优化路径**：每次 reset 重开连接池成本 ≈61s（78s 无重开 → 147s 有重开），使总耗时未达 ≤090s 软目标。尝试过"仅在检测到变异时才重开"的优化，但**不可行**——重开还需保护"执行中变异"的测试（如 `TestTopicLineageSurvivesClusterDrift` 在 SetupTestDB 返回后才 ALTER，reset 时无法检测）。真正降到 ~80s 的两条未来路径（**超出本 change 范围**）：
- (a) 禁用 prepared statement 缓存（pgx simple protocol）→ 无 stale plan → 无需重开；
- (b) 让 2 个执行中变异测试 opt-in 重开池。

## Risks / Trade-offs

- **[风险] truncate 暴露测试间隐式数据共享耦合** → 某测试若偷偷依赖上个测试残留的数据，truncate 后会失败。**缓解**：这正是揪出隐藏耦合的好机会；归档前预期会有少量此类返工，逐个修复（让测试显式 seed 自己需要的数据）。这是"测试质量提升"的副作用，非缺陷。
- **[风险] 快照范围遗漏新 seed 表** → 未来若新增迁移 seed 到第三张表，快照表名列表未同步则该表的 seed 不被恢复、测试失败。**缓解**：快照表名是受控元数据（仅 `ai_settings`、`embedding_config` 两个表名），design/task 明确"新增 seed 表必须同步加入快照表名列表"，并在 `standard/backend/testing.md` 增补一句约束；快照**内容**始终运行时从 DB 读，不存手工副本。
- **[风险] 黄金 schema 串行假设被打破** → 若某受影响包未来加了 `t.Parallel()`，共享 schema + truncate 会数据串台。**缓解**：apply 前确认受影响包当前无 `t.Parallel()`；在 testing.md 注明"共享 schema 模型下不得对集成测试加 t.Parallel()"。
- **[trade-off] 放弃并行** → 无法靠并行再提速。当前可接受；若未来集成测试数继续膨胀，再评估方向 D。
- **[风险] 修改 `testutil` 影响所有集成测试** → `testutil` 是所有集成测试入口，改动波及面大。**缓解**：`testutil` 属 `internal/platform/`，改动通过 `codegraph impact` 复核；按 TDD 先写"黄金 schema 只建一次"的红测试守行为。

## Migration Plan

**部署/合并影响（按执行规范要求汇报）**：

- (a) **用户可见行为变化**：无。本 change 仅改测试基建（`testutil` + 受影响包的测试运行方式），**不触碰任何生产代码、不碰迁移、不改 API、不改前端**。部署后产品行为零变化。
- (b) **需用户手动执行的操作**：无数据迁移、无配置变更、无清理。开发者下次跑集成测试自动享受提速，CI 同理。
- (c) **旧数据降级**：无（无生产数据变更）。

**唯一"行为变化"**：集成测试的 DB 重置策略从"DROP+重建 schema"变为"truncate 清数据"。对开发者可见的现象是测试跑得快了 6 倍；潜在副作用见上方"truncate 暴露隐式耦合"风险。

**回滚**：纯测试基建改动，`git revert` 即可，无状态残留。

## Open Questions

1. 快照数据结构的内存形态：`[]map[string]any`（GORM Raw 通用）还是具体 `[]models.AISettings`/`[]models.EmbeddingConfig`？→ 倾向具体类型（类型安全、GORM 灌回简单），apply 时定稿。
2. 黄金 schema 失败重试策略：首次 `runTestMigrations` 失败（如镜像拉取失败）是否需要比现在 `t.Fatalf` 更友好的处理？→ 现状 `OpenTestDB` 已有 `t.Fatalf` + hint，沿用，不在本 change 扩展。
