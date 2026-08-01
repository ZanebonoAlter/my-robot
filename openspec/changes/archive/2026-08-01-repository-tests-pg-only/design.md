## Context

`topicgraph/repository` 是直接发 SQL 的数据访问层。全仓 `*/repository/` 包共 13+ 个测试文件，其中 `topicgraph/repository` 同包已有 9 个文件走 `testutil.SetupTestDB`（testcontainer PG + 黄金 schema），但残留 2 个文件用内存 SQLite 测 DB 行为：

- `topic_watch_repository_test.go`：10 个 CRUD 测试（含级联删除、复合唯一去重）整文件用 `sqlite.Open` + 手动 `AutoMigrate(&BoardTopicWatch{}, &TopicWatchHit{})`。
- `daily_report_manual_topic_test.go`：混合文件，22 个测试中 8 个碰 DB（`CreateManualTopic`×4、`GetComposeCandidates`×4）走 `setupManualTopicTestDB`（SQLite），另 14 个是纯函数（AggregateEmbeddings / DetectOutliers / FloatsToPgVector）。

`standard/backend/testing.md` 已记录多起「SQLite 全绿、生产 PG 炸」事故（GROUP BY 缺列、vector/JSONB 空串、迁移 NOT NULL），并立了「改发往真实 DB 的 SQL 必须有 testcontainer PG 用例」的硬约束。本 change 把该约束前置为「repository 包常驻禁 SQLite」，并补齐最后一片半的迁移。

同包 `daily_report_assignment_test.go`、`daily_report_matching_test.go` 经核实为**零 DB 依赖的纯算法测试**（CosineDistance / 匈牙利分配 / Phase1·2），属健康 unit，不在本次迁移范围。

## Goals / Non-Goals

**Goals:**

- 把 `topic_watch_repository_test.go`（10 测试）迁到 `testutil.SetupTestDB`，断言逻辑零改动。
- 拆分 `daily_report_manual_topic_test.go`：14 个纯函数 → 新文件 `daily_report_manual_topic_unit_test.go`；8 个 DB 测试留原文件并迁 PG。
- 在 `test-infrastructure` spec 新增「repository 层禁 SQLite」requirement，并在 `standard/backend/testing.md` 增补对应硬约束段——spec 与文档两处呼应，防回退。

**Non-Goals:**

- **不**迁移非 repository 包的 SQLite 测试（`admin/handler`、`platform/airouter`、`reader/*`、`dataenrichment/*` 等）——本次只清数据访问层这个最高危温床；其它包的 SQLite 多为 handler/服务层轻量测试，按 testing.md 既有约定处理，留作后续。
- **不**引入新依赖、不改产品代码、不改 schema、不改 API。
- **不**实现自动强制（如 check-standards grep 拦截）——本次靠 spec + 文档约束；自动 check 可作为后续独立 change，避免本次范围蔓延。
- **不**动 `assignment` / `matching` 两个纯算法测试文件。

## Decisions

### 决策①：迁移模式——机械替换 setup，断言零改动

`setupXxxTestDB` 当前做两件事：`sqlite.Open(...)` 建内存库 + 手动 `AutoMigrate(...)` 建表。黄金 schema 模式下，`testutil.SetupTestDB` 已执行全量生产 AutoMigrate（`BoardTopicWatch` / `TopicWatchHit` 在 `daily_report_models.go`，已被纳入），因此迁移 = 删掉手动 setup，换成 `db := testutil.SetupTestDB(t)`，再 `repo := NewTopicGraphRepository(db)`。**断言逻辑一字不改**。

**备选**：保留 SQLite 作为 fast path、PG 作为完整 path 双轨。**否决**——双轨正是假绿根源（fast path 全绿掩盖 PG 问题），违背本 change 初衷。

### 决策②：manual_topic 拆分按现有命名规范

14 个纯函数归 `daily_report_manual_topic_unit_test.go`（`-short` 可跑、零 DB），8 个 DB 测试留 `daily_report_manual_topic_test.go` 并迁 PG。这直接兑现 spec「测试文件命名规范区分单元与集成」requirement（`_unit_test.go` vs `_test.go`），且纯函数测试无需 Docker、本地秒跑，开发体验更好。

### 决策③：规矩落 spec + testing.md 两处

- spec：`test-infrastructure` ADDED requirement（机器可读、archive 时 sync 进主 spec）。
- 文档：`standard/backend/testing.md` 增补硬约束段（人读、apply 时 doc-impact 注入）。

两处一致，避免「spec 改了文档没改」或反之的 drift。

### 决策④：assignment / matching 不迁

`grep` 核实：两文件 setup/NewRepository/gorm.Open/`database.DB` 调用均为 0，import 无任何 DB 包，是纯算法测试（CosineDistance 正交/同向/长度不匹配、匈牙利算法各场景、PlanLifecycle 状态机）。强行迁 PG 是无谓增成本，且会模糊「该层是纯逻辑、本就该 unit」的正确分层。

### 决策⑤：迁移暴露的 DeleteWatch 级联 bug —— 拆 change 追踪，不在本 change 修

切片 1 实测发现 `DeleteWatch` 在 PG 不级联删 hits（model tag CASCADE 形同虚设，迁移漏建 FK，详见 proposal「迁移副产品」）。三个选项：(a) 本 change 加 FK 迁移修；(b) `DeleteWatch` 内手动级联；(c) 改断言为留孤儿。**决策：都不在本 change 做**——(a)/(b) 超出「3 文件/不动产品代码」范围，且 FK 迁移涉及历史孤儿数据兼容（testing.md「schema 迁移要在 testcontainer PG + 历史数据下测」硬约束）；(c) 掩盖 bug。改为**拆 `fix-watch-delete-cascade` change** 独立追踪，本 change 2 个级联测试 `t.Skip` 指向它（skip message 已更新为明确归属，非「awaiting decision」悬空）。这是本次迁移的最大价值——把 SQLite 掩盖的生产 bug 真正抓出来了。

### 决策⑥：rollback 测试删除（SQLite 遗物）

`TestCreateManualTopic_RollbackOnRebuildRelationsFailure` 前提是「`period_date::date` 在 SQLite 必败→事务回滚」，迁 PG 后 cast 合法、调用成功，断言全翻转，是 SQLite 遗物。PG happy-path 已被 `TestManualTopic_CreateAndReassign` 覆盖。**决策：删除**（非 skip）——留个前提反转的死测试无意义；「事务回滚」保护若要 PG 下真测，应构造真实中途失败（独立工作，可后续补）。原 8 个 DB 测试现为 7 个。

## Risks / Trade-offs

- **[迁移后测试变慢]** repository 测试加入 testcontainer，但黄金 schema 进程内复用，增量只是每次 `ResetTestData` 的 reset 成本（秒级），且 CI 本就跑 testcontainer。→ 可接受，不额外优化。
- **[vector seed 空串坑]** `manual_topic` 的 8 个 DB 测试涉及 embedding/vector 列。SQLite 对 vector 空串宽松，PG 严格（`SQLSTATE 22P02`）。→ 迁移时逐一检查 seed，vector 列必须用 `repository.FloatsToPgVector(...)` 填合法值，不留零值空串（testing.md 已有先例）。
- **[规矩无自动强制]** 本次靠 spec + 文档人读约束，无 grep 拦截。→ 风险是日后可能回退；作为后续独立 change 加 check-standards 自动校验（扫 `*/repository/*_test.go` 无 `sqlite.Open`），本次不做以控范围。
- **[拆分文件漏迁测试]** manual_topic 拆分时可能漏掉某个测试的归类。→ 以「是否调用 `setupManualTopicTestDB`」为判据（已用 awk 逐函数配对核实：14 否 / 8 是），迁移后跑全包测试对账。
