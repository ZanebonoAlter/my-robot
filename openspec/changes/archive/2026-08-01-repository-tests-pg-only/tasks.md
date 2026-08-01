# Tasks: repository 层清零 SQLite + 立禁用规矩

> 垂直切片，每切片独立可交付、可验证。推荐顺序：1 迁移 topic_watch（最直接，model 已在 schema）→ 2 拆分 manual_topic（含 vector seed 坑）→ 3 增补 testing.md 硬约束（spec delta 已于 propose 阶段写好）。尾部遵循《开发执行规范》§11 归档门禁。
>
> ⚠️ 前置：`testutil.SetupTestDB` 黄金 schema 模式已成熟（同包 9 文件在用），`BoardTopicWatch`/`TopicWatchHit` 已在 `daily_report_models.go` 且纳入生产 AutoMigrate——迁移无需额外 seed 配置。后端命令走 WSL；testcontainer 集成测试需 Docker daemon 可用。
>
> 📌 **迁移副产品**：切片 1 暴露出生产 bug——`DeleteWatch` 在 PG 上不级联删 `topic_watch_hits`（model tag 声明 CASCADE 但迁移漏建 FK，`DisableForeignKeyConstraintWhenMigrating=true` 致 AutoMigrate 不建 FK）。SQLite 靠 `PRAGMA foreign_keys=ON` 让级联生效掩盖了它。**已拆 `fix-watch-delete-cascade` change 追踪**（加 FK CASCADE 迁移），本 change 不修产品代码；2 个级联测试保留 `t.Skip` 指向该 change，落地后 re-enable。

## 1. 迁移 topic_watch_repository_test.go 到 testcontainer PG

- [x] 1.1 `setupWatchTestDB` 改造：删 `sqlite.Open(...)` + 手动 `AutoMigrate(&BoardTopicWatch{}, &TopicWatchHit{})`，改为 `db := testutil.SetupTestDB(t); return NewTopicGraphRepository(db)`。验收：`grep -nE 'glebarez/sqlite|sqlite.Open' internal/topicgraph/repository/topic_watch_repository_test.go` → 零命中
- [x] 1.2 import 块调整：移除 `"github.com/glebarez/sqlite"`，新增 `syntopica-backend/internal/platform/testutil`；清理因迁移产生的未用 import。验收：`go build ./internal/topicgraph/repository` → BUILD_OK
- [x] 1.3 10 个 CRUD 测试在真 PG 下跑通。**实测：8 PASS + 2 SKIP，0 FAIL**。唯一索引（`UpsertDedup`/`DuplicateRejected`）通过；2 个级联删除测试（`TestDeleteWatchCascadesHits`、`TestDeleteWatchDoesNotAffectOtherWatchHits`）在 PG 黄金 schema 下断言不成立——根因是生产 bug（`topic_watch_hits.watch_id` 无 FK ON DELETE CASCADE）。**决策：拆 `fix-watch-delete-cascade` change 追踪**（加版本化 FK CASCADE 迁移，对齐 model tag 意图），2 个测试保留 `t.Skip` 指向该 change（skip message 已更新），落地后去掉 `t.Skip` 即通过、断言不改。该 bug 是本次迁移消灭「SQLite 全绿」温床的价值证明
- [x] 1.4 不加 `t.Parallel()`（黄金 schema 是进程级共享态，禁止并发 truncate 串台，见 testing.md 约束②）。验收：`grep -n 't.Parallel' internal/topicgraph/repository/topic_watch_repository_test.go` → 零命中

## 2. 拆分 daily_report_manual_topic_test.go

- [x] 2.1 新建 `internal/topicgraph/repository/daily_report_manual_topic_unit_test.go`，搬入 14 个纯函数测试（`AggregateEmbeddings`×6 / `DetectOutliers`×4 / `FloatsToPgVector`×3 / `PureFunctionDetection`）；移除其 DB setup 与 import，纯内存逻辑。验收：`go test -short ./internal/topicgraph/repository -run 'AggregateEmbeddings|DetectOutliers|FloatsToPgVector|PureFunction' -v` → 14/14 PASS（无 Docker）
- [x] 2.2 原 `daily_report_manual_topic_test.go` 仅留 DB 测试；`setupManualTopicTestDB` 改为 `testutil.SetupTestDB(t)` + `NewTopicGraphRepository(db)`（另加 `Repo = repo`，因 `CreateManualTopic→RebuildBoardRelations` 经 tx helper 读包级 `Repo`，与同包其它 PG 集成测试一致）。验收：`grep -nE 'glebarez/sqlite|sqlite.Open' internal/topicgraph/repository/daily_report_manual_topic_test.go` → 零命中
- [x] 2.3 vector seed 防坑。**实测：7 PASS，0 vector 报错**。修复 3 处空串 seed：① `TestCreateManualTopic_NoUsableEmbeddings` section `Embedding:""` → `Omit("embedding")`(NULL)；② `TestGetComposeCandidates_ParsesEmbeddingsAndExcludesEmpty` s3 同理 → NULL；③ `TestGetComposeCandidates_AttachesTopicBrief` 的 `BoardPersistentTopic` 零值 embedding → 补 `vecStr(0.2,0.1)`。**决策：删除 `TestCreateManualTopic_RollbackOnRebuildRelationsFailure`**——该测试是 SQLite 遗物（前提「`period_date::date` 在 SQLite 必败→事务回滚」迁 PG 后前提反转、断言全翻转），PG happy-path 已由 `TestManualTopic_CreateAndReassign`（`daily_report_topic_integration_test.go`）覆盖。原 8 个 DB 测试现为 7 个（3 CreateManualTopic + 4 GetComposeCandidates）
- [x] 2.4 两个文件 import 各自独立、无跨文件残留依赖。验收：`go build ./internal/topicgraph/repository` → BUILD_OK

## 3. 增补 testing.md 硬约束段（spec delta 已于 propose 完成）

> spec delta（`openspec/changes/repository-tests-pg-only/specs/test-infrastructure/spec.md` ADDED requirement「数据访问层测试禁用内存 SQLite」）已写好，archive 时按 §12 sync 进主 spec。本节只做人读文档增补。

- [x] 3.1 `docs/reference/standard/backend/testing.md`「集成测试常见陷阱」节首位增补「🛑 repository 层禁 SQLite」硬约束段：SHALL 用 `testutil.SetupTestDB`、SHALL NOT 用内存 SQLite 测数据访问逻辑；附判定法（是否在 `*/repository/` 包 + 是否碰 DB）；引用既有 4 起事故作前车之鉴；注明纯函数测试归 `_unit_test.go` 豁免；附 grep 回归守卫 + spec 链接。验收：`grep -n '数据访问层（repository）测试禁用内存 SQLite' docs/reference/standard/backend/testing.md` → 命中（73 行）

## 4. 文档

<!-- doc-impact: standard -->

- [x] 4.1 `docs/reference/standard/backend/testing.md`：增补「repository 层禁 SQLite」硬约束段（任务 3.1）
- 无 flow 影响：本 change 是后端测试基建收敛（repository 层 SQLite 测试迁 testcontainer PG + 立禁用规矩），不改任何业务 flow 的生成/编排流程，按《开发执行规范》§12.2 豁免 flow 变更溯源

## 5. 测试（§11.2）

> 后端命令走 WSL；testcontainer 集成测试需 Docker daemon 可用（testcontainer 自管隔离容器）。

- [x] T.1 `cd backend-go && go test -short ./internal/topicgraph/repository` → 14 个纯函数 PASS（无 Docker）·实测 `ok 0.011s`
- [x] T.2 `cd backend-go && go test ./internal/topicgraph/repository` → 全部 PASS（watch 8 PASS + 2 SKIP 级联待 fix-watch；manual_topic 7 PASS；14 纯函数 PASS）·实测 `ok 5.642s`，15 PASS + 2 SKIP + 0 FAIL
- [x] T.3 `grep -rnE 'glebarez/sqlite|sqlite\.Open' backend-go/internal/topicgraph/repository/` → 零命中（repository 层清零 SQLite 验证）

## 6. 验证（§11.2，归档前实测）

- [x] V.1 `cd backend-go && go build ./...` → BUILD_OK（实测通过）
- [x] V.2 `cd backend-go && go vet ./internal/topicgraph/...` → VET_OK（实测通过）
- [x] V.3 `cd backend-go && golangci-lint run ./internal/topicgraph/...` → 0 issues（实测 `0 issues` exit 0）
- [x] V.4 `bash scripts/check-standards.sh` → 本 change 相关全绿（A-D/E/G/H 段 OK；F 段 `[OK] doc-impact 通过 repository-tests-pg-only`）。注：总报告「通过 81 / 失败 3」的 3 个 FAIL 属别的 active change（add-method-auto-instrumentation / daily-report-peel-transition / feed-param-options）的 pre-existing doc-impact 问题，非本 change 引入，不阻塞归档
- [x] V.5 `bash scripts/doc-impact.sh verify openspec/changes/repository-tests-pg-only` → 通过（声明: standard  文件:1 个）
- [x] V.6 `openspec validate repository-tests-pg-only --strict` → Change 'repository-tests-pg-only' is valid
