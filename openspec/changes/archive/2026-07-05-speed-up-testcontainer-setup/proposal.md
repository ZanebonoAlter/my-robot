## Why

后端集成测试反馈过慢：`go test ./internal/topicgraph/repository ./internal/topicgraph/service` 约 355 秒，`topicgraph/repository` 一个包就占 300+ 秒。这拖慢所有后端 PR 的 TDD 红绿循环，也导致开发时倾向跳过集成测试。

## 根因（已核实，非猜测）

容器**已经是单例**（`testutil.go` `startOnce sync.Once`，每进程只起一个 pgvector 容器）。慢不在容器启动，而在 `SetupTestDB` 的数据重置策略：

1. 每个 `SetupTestDB` 调用都会执行 `ReimportTestDB`：`DROP SCHEMA public CASCADE` → `CREATE SCHEMA` → `runTestMigrations`（AutoMigrate 全部模型 + **重跑全部版本化迁移**）→ 因 pgvector `vector` 类型 oid 变化而重开连接池。
2. `topicgraph/repository` 有 41 个 `SetupTestDB` 调用点，每次重置约 8 秒，41 × 8 ≈ 328 秒。
3. 已有 `TruncateAllTables`（CASCADE 快速清表），但测试代码普遍直接调 `SetupTestDB`（内部触发完整重建），未走轻量重置路径。

所以这是"数据重置策略选了最重的方案"，不是"容器多开"。

## 设计约束（不能破坏的）

- **版本化迁移是 source of truth**：FK/index/trigger/seed（ai_settings、embedding_config）必须由真实迁移产出，不能手工 DDL 维护。
- **pgvector oid 漂移**：重建 schema 后 `vector` 类型 oid 变化，旧连接池的 prepared statement 失效（`cache lookup failed for type <oid>`）。当前靠重开连接池绕过，优化方案必须保留这点。
- **安全红线**（执行规范 §6.2）：`testutil` 不得有默认 DSN / 读环境变量 / 连开发库——历史事故曾因此误 `TRUNCATE` 生产数据。任何改动不得引入"连同一容器多个库"等可被误配的路径。
- **测试语义不变**：每个测试必须仍拿到"与生产一致的干净 schema + seed"。

## 候选优化方向（决策待与用户讨论，不在本 change 现阶段拍板）

以下方向**互不冲突**，最终组合由讨论决定：

### 方向 A：进程级一次性 schema 构建 + 测试间 Truncate
- 用 `TestMain` 起容器 + 跑一次迁移，建立"黄金 schema"。
- 每个测试用 `TruncateAllTables`（已存在）清数据，而非 DROP+重建。
- 节省：41 次完整迁移 → 1 次迁移 + 41 次 truncate。
- Trade-off：需审计每个测试是否依赖 seed；truncate 后需重新 seed 必要配置；并发测试需数据隔离（独立 board_id / schema）。

### 方向 B：分层——把不需要 pgvector 的测试降级回 SQLite 内存
- 执行规范 §0 已规定"单元测试用内存 SQLite（glebarez/sqlite），无 pgvector 依赖"。
- 现状：许多测试本不需要 pgvector（纯 GORM CRUD / 状态机 / 排序逻辑）却用了 testcontainer。
- 审计 `topicgraph/repository` 41 个集成测试，凡不涉及 `vector` 列 / embedding cosine 的，迁回 SQLite 单元测试。
- Trade-off：需逐个判断测试是否真的依赖 pgvector；SQLite 与 Postgres 行为差异（如 JSONB、事务、并发）需注意；可能是最快见效的方向。

### 方向 C：减少版本化迁移的重复执行
- 若方向 A 落地，迁移天然只跑一次。若不落地 A，可考虑"快照 schema 后克隆"，但实现复杂、收益不如 A。
- Trade-off：复杂度高，优先级最低。

### 方向 D：测试并行化
- 当前测试串行（共享单 DB 状态）。并行需数据隔离（每测试独立 schema/database）。
- Trade-off：与 A 的"共享黄金 schema + truncate"冲突，需二选一或多容器。

## 影响面

- 改 `testutil.SetupTestDB` / `ReimportTestDB` / `TruncateAllTables` 的协作方式。
- 改受影响包的集成测试调用模式（取决于选定方向，可能 41 个调用点都要调整）。
- 不改任何生产代码、不改迁移本身、不改测试断言语义。

## Non-Goals

- 不重写 testcontainer 基建（保留单例容器 + Ryuk 清理）。
- 不改版本化迁移的内容。
- 不引入新的测试框架。
- 不为追求速度牺牲测试的"与生产一致"保证。

## Open Questions（决策点，待讨论）

1. 优先做哪个方向？倾向 B（降级回 SQLite，符合规范、见效快）还是 A（一次性 schema）？
2. 测试是否允许共享 schema + truncate，还是必须每测试独立隔离（影响能否并行）？
3. seed 数据如何管理：固定黄金 seed 还是每测试 seed 自己需要的？
4. 是否保留 `ReimportTestDB` 作为"需要干净迁移态"的逃生口（少数回归测试可能仍要）？

> 本 change 暂不补 design / specs / tasks，待上述决策定稿后再按 OpenSpec 流程补齐。
