# 后端测试（testing + testcontainer）

> **权威源**：本文件是后端测试约定的唯一权威。运行门禁见《开发执行规范》§4.1、集成测试执行纪律见 §6。
> 含 🛑 **DSN 安全红线（事故教训）**，必须遵守。

## 框架

- 标准 `testing` 包 + `github.com/stretchr/testify` 断言（仅当文件已用 testify 时才 import，否则用标准 testing）
- 测试文件以 `*_test.go` 与源码放在同一包中
- 偏好表驱动测试（table-driven）
- setup 函数中使用 `t.Helper()` 获得更清晰错误追踪

## 测试分层

| 层级 | 文件后缀 | 运行方式 | 需要 Docker | 说明 |
|------|----------|----------|-------------|------|
| 单元测试 | `xxx_unit_test.go` | `go test -short ./...` | 否 | 纯逻辑，无外部依赖 |
| 集成测试 | `xxx_test.go` | `go test ./...` | 是 | testcontainers-go 启动隔离 pgvector 容器 |

轻量 CRUD 测试（无 pgvector 依赖）可继续用内存 SQLite（`glebarez/sqlite` + `mode=memory`），参考 `internal/reader/service/feed_service_test.go` 的 `setupFeedsTestDB` 模式。`admin/handler`、`platform/airouter`、`reader/*`、`topicgraph/handler` 等包仍用内存 SQLite。

## `testutil.SetupTestDB` 使用模式

集成测试标准入口函数，**进程级「黄金 schema」模型**——首次调用构建一次 schema，后续调用走轻量重置：

1. **跳过单元模式**：`-short` 时自动 `t.Skip`
2. **启动隔离容器**：testcontainers-go 启动一次性 pgvector 容器（镜像 `pgvector/pgvector:pg18-trixie`，与生产同构；进程级单例，首次调用启动，后续复用，进程退出由 Ryuk sidecar 自动销毁）
3. **首次调用——构建黄金 schema（进程内只发生一次）**：执行一次生产 `RunAutoMigrate` 与 `RunMigrations`（含迁移注入的 seed），并立即快照①迁移 seed 表全部行、②所有 vector 类型列的类型声明，缓存为进程级黄金 schema
4. **后续调用——`ResetTestData` 快速重置**：不再重建 schema，改为清空业务表 + 从快照恢复 seed + re-ALTER vector 列回黄金维度 + 重开连接池，结果等价于「生产首次启动后的状态」
5. **设置全局 DB**：兼容生产代码的 `database.DB`

```go
// xxx_test.go（集成）
func TestSomethingIntegration(t *testing.T) {
    db := testutil.SetupTestDB(t)
    // db 已连接，等价于生产首次启动后的 schema 与默认数据
    // ... 测试逻辑
}

// xxx_unit_test.go（单元）
func TestSomethingUnit(t *testing.T) {
    // 纯逻辑，不调用 SetupTestDB
}
```

### 相关函数

- **`SetupTestDB(t) *gorm.DB`**：集成测试唯一入口（上述两阶段模型）。
- **`ResetTestData(t, db) *gorm.DB`**：黄金 schema 模式下测试间重置——truncate 业务表（跳过 `schema_migrations`）+ 从快照恢复 seed + re-ALTER vector 列回黄金维度 + 重开连接池。返回**重开后的新连接句柄**，调用方 MUST 重新赋值。黄金 schema 已构建后由 `SetupTestDB` 内部自动调用，一般无需手动调。
- **`TruncateAllTables(t, db)`**：CASCADE 清空所有业务表（**含 seed**，真·全空），无快照恢复、无重开。原义保留，供显式需要「连 seed 一起清空」的场景。
- **`ReimportTestDB(t, db) *gorm.DB`**：DROP schema + 重建 + 重跑迁移 + 重开连接池。作为**逃生口保留**——少数需要「干净迁移态」的回归测试（如 `TestReimportPreservesVectorInserts`）继续可用（只操作临时容器，不读开发/生产 DSN）。

### 🛑 黄金 schema 模型的两条约束

**约束①——新增迁移 seed 表必须同步登记快照**：当前快照表名清单仅 `ai_settings`、`embedding_config`（受控元数据，快照**内容**运行时从黄金 schema 的 DB 读取，不存手工副本，不会 drift）。未来若版本化迁移向第三张表注入 seed，MUST 同步把该表名加入 `ResetTestData` 的快照表名列表，否则该表 seed 不被恢复、测试失败。

**约束②——共享 schema 模型下禁止对集成测试加 `t.Parallel()`**：黄金 schema 是进程级共享态，所有测试串行轮流使用、测试间用 truncate 清数据。给集成测试加 `t.Parallel()` 会并发 truncate 串台、跨测试数据污染。

> **性能现实**：黄金 schema + reset 把 `topicgraph/repository` 包（41 个 `SetupTestDB` 调用点）从 ~382s 降到 ~147s（~2.6x）。瓶颈在每次 reset 重开连接池（撤销测试对 vector 列的 ALTER 变异、清除失效 prepared statement）；进一步优化路径见 `openspec/changes/speed-up-testcontainer-setup/design.md` 决策⑥。

## 🛑 DSN 安全红线（事故教训 — 不可违反）

> 早期版本曾通过默认 DSN 连到开发库并执行 `TRUNCATE`/`DROP TABLE`，**清空了业务数据**。该路径已彻底移除。

**硬约束：**

- `testutil` 包**没有**默认 DSN，**不读取**任何环境变量（包括历史上的 `TEST_DB_DSN`）
- 它**只能**启动隔离容器，**无法**连接 `docker-compose.pg.yml` 跑的开发库（那是生产数据所在）
- **禁止**重新引入任何「默认数据库连接」或「env 覆盖」机制

## 集成测试常见陷阱

### ⚠️ vector 列 seed 必须填合法值，不能依赖零值

部分 vector 列的 Go 字段是**非指针 `string`**（零值 `""`），pgvector 会拒绝空串：

| 模型 | 字段 | 类型 | 零值行为 |
|------|------|------|----------|
| `models.SemanticLabel` | `Embedding` / `MergeEmbedding` | `*string` | nil → NULL ✅ |
| `repository.DailyReportSection` | `Embedding` | `string` | `""` → **报错** ❌ |

**症状**：`invalid input syntax for type vector: "" (SQLSTATE 22P02)`
**修复**：seed 时用 `repository.FloatsToPgVector([]float64{0})` 填合法值（镜像生产 insert 前的填充）。

```go
section := repository.DailyReportSection{
    ReportID:     reportID,
    ClusterLabel: label,
    Embedding:    repository.FloatsToPgVector([]float64{0}),
}
```

### ⚠️ `ReimportTestDB` 内置重连，不可移除

`DROP SCHEMA public CASCADE` 会连坐销毁 pgvector 的 `vector` 类型，`CREATE EXTENSION IF NOT EXISTS` 重建时拿到**新 OID**（实测每周期递增：16387→17280→18173）。连接池里残留的 prepared statement 仍引用旧 OID，导致后续 vector 查询失败。

**症状（任一）：**
- `cache lookup failed for type <oid> (SQLSTATE XX000)`
- `cached plan must not change result type (SQLSTATE 0A000)`

**关键认知**：catalog 本身没坏（`pg_depend` 正常、扩展正确重注册），坏的是**连接的 prepared statement 缓存**。修复不是动 schema/扩展，而是 `ReimportTestDB` 重建后**关闭旧池、开新池**（已实现）。

> 补充：主路径（黄金 schema + `ResetTestData`）已不再触发 OID 漂移——黄金 schema 只建一次、从不 DROP 扩展，`vector` 类型 OID 全程稳定；`ResetTestData` 末尾的重开连接池改为清除「测试 `ALTER COLUMN ... TYPE vector(N)` 变异导致的 prepared statement 失效」。`ReimportTestDB` 仅作少数显式回归的逃生口走这条 DROP+重建+重开路径。

> 🛑 **切勿**为「优化」删掉 `ReimportTestDB` 末尾的 `openGorm` + `Close`——那是修这个坑的核心，删掉会复现 `cache lookup failed`。回归测试 `TestReimportPreservesVectorInserts` 守住这行。

### ⚠️ 改了发往真实 DB 的 SQL（GROUP BY / 聚合 / 复杂 JOIN / 手写 raw SQL），必须有 testcontainer PG 用例真跑该路径

SQLite（内存单测）对 GROUP BY 语义、类型、约束都宽松；PostgreSQL（生产）严格。**两者语义不一致**——纯逻辑/SQLite 单测覆盖不了真实 SQL 的运行时错误。

**症状**：生产全板失败、报 `SQLSTATE 42803 column "..." must appear in the GROUP BY clause or be used in an aggregate function`（或其它 SQL 语义/类型错误），而本地 SQLite 单测全绿。

**根因（事故）**：`quality-scoring-observability` change 在 `collectBoardTags` 的 SELECT 加了 `topic_tag_board_labels.downgraded`，却漏在 GROUP BY 同步加该列。SQLite 不报错（允许 SELECT 非聚合列不进 GROUP BY），PG 直接拒掉 → 日报生成每个 board 都炸、界面全红。当时 testcontainer 集成测试没真跑到这条 SQL，漏网到生产。

**硬约束**：
- 凡改动涉及 ① SELECT 新列 + GROUP BY，② 复杂 JOIN/聚合，③ 手写 raw SQL——**门禁必须包含一条真正调用该查询的 testcontainer PG 集成测试**（seed 多行数据触发 GROUP BY/聚合），SQLite 单测**不算**覆盖。
- 判定法：改的 SQL 换成 PG 跑会不会和 SQLite 表现不同？会 → 必须补 PG 集成测试。
- 回归守卫样本：`TestCollectBoardTags_PGHonorsDowngradedColumn`（`internal/topicgraph/service/collect_board_tags_test.go`）。

### ⚠️ 绿灯 ≠ 功能有效：测试通过后必须用真实数据核对"效果是否达预期"

测试断言的是**契约**（函数返回了非空 `ArticleContext`），不是**业务效果**（日报头条是否不再混淆）。二者之间隔着数据质量、依赖覆盖率、LLM 实际行为——这些测试抓不到。

**真实教训（`qwythos-thinking-toggle-report-grounding` change，2026-06-27）**：

为修复"日报头条看不见事件详情导致混淆"，给 `TagInput` 加了 `ArticleContext`（注入代表文章标题+摘要）。三层测试全绿：
- 单元测试 `TestBuildHighlightsPrompt_InjectsArticleContext` ✅
- testcontainer PG 集成测试 `TestCollectBoardTags_PopulatesArticleContext` ✅
- 门禁 lint/vet/build 全过 ✅

但连开发库核对真实数据时发现：
- 全库 1656 篇文章只有 **227 篇（13.7%）** 有 AI 摘要——`buildArticleContextForTag` 取 `ai_content_summary` 时大量命中空值；
- 根因不在 context 注入代码（实现是对的、fallback 到原始 content 也正确），而在**上游 AI 摘要覆盖率低**（18/25 个 feed 没开 `article_summary_enabled`、`summary_status` 状态机卡死），context 注入改不了这个上游缺陷。

**测试全绿掩盖了"功能上线了但效果受限于数据覆盖率"的事实**——如果只看测试结果就交付，用户会发现日报质量没明显改善，却要等部署后才察觉。

**硬约束（适用于"效果依赖数据质量/依赖覆盖率/LLM 实际行为"的功能）**：
1. **测试通过只是起点**。对于这类功能，完成 TDD 红绿循环后，**必须连真实库（或贴近真实的样本）核对实际产出**，不能只靠测试绿灯就交付。
2. **核对方法**：跑真实数据看输出，量化"覆盖率/命中率/实际注入量"，而非"函数返回了非空值"。例：不只看 `ArticleContext != ""`，要看"30 个 tag 里几个真的注入了内容、注入了多长、空的有几个"。
3. **发现效果不达预期时，先别急着改自己的实现**。像这次——context 注入没问题，问题在上游摘要覆盖率。先定位瓶颈在哪一层（自己这层 vs 上游数据层），再决定是改代码还是反馈给用户。
4. **主动和用户沟通**：当"测试全绿但实际效果受限"时，把量化数据摆给用户（如"覆盖率只有 13.7%，瓶颈在 feed 没开摘要"），让用户决定是接受现状、先补上游、还是调整预期。**不要等用户部署后自己发现效果不行。**

**判定法**：这个功能的效果是否依赖测试断言之外的"数据/环境/模型行为"？是 → 绿灯后必须做真实数据核对 + 效果评估，不能直接交付。

## 运行

```bash
cd backend-go
go test ./...                  # 含集成测试（需 Docker）
go test -short ./...           # 仅单元测试
go test ./internal/reader/service -v                         # 单个包
go test ./internal/topicgraph/repository -run TestName -v    # 单个测试
```

> AGENTS.md 约定：日常验证**只跑本次修改影响的包**，不跑全量。

## 资料来源

收敛自原 `testing.md`（§后端 / §后端测试分层约定 / §集成测试常见陷阱）与《开发执行规范》§4.2、§6。
