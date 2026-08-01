# Tasks: repository 层清零 SQLite + 立禁用规矩

> 垂直切片，每切片独立可交付、可验证。推荐顺序：1 迁移 topic_watch（最直接，model 已在 schema）→ 2 拆分 manual_topic（含 vector seed 坑）→ 3 增补 testing.md 硬约束（spec delta 已于 propose 阶段写好）。尾部遵循《开发执行规范》§11 归档门禁。
>
> ⚠️ 前置：`testutil.SetupTestDB` 黄金 schema 模式已成熟（同包 9 文件在用），`BoardTopicWatch`/`TopicWatchHit` 已在 `daily_report_models.go` 且纳入生产 AutoMigrate——迁移无需额外 seed 配置。后端命令走 WSL；testcontainer 集成测试需 Docker daemon 可用。

## 1. 迁移 topic_watch_repository_test.go 到 testcontainer PG

- [x] 1.1 `setupWatchTestDB` 改造：删 `sqlite.Open(...)` + 手动 `AutoMigrate(&BoardTopicWatch{}, &TopicWatchHit{})`，改为 `db := testutil.SetupTestDB(t); return NewTopicGraphRepository(db)`。验收：`grep -nE 'glebarez/sqlite|sqlite.Open' internal/topicgraph/repository/topic_watch_repository_test.go` → 零命中
- [x] 1.2 import 块调整：移除 `"github.com/glebarez/sqlite"`，新增 `syntopica-backend/internal/platform/testutil`；清理因迁移产生的未用 import。验收：`go build ./internal/topicgraph/repository` → BUILD_OK
- [ ] 1.3 10 个 CRUD 测试在真 PG 下跑通，重点核查级联删除（`TestDeleteWatchCascadesHits`）与复合唯一去重（`TestTopicWatchHit_UpsertDedup`）—— SQLite 外键/约束语义与 PG 不同，确认断言在 PG 下仍成立。验收：`go test ./internal/topicgraph/repository -run 'Watch' -v` → 10/10 PASS  ⚠️ **部分完成 / 需主线程决策**：实测 8/10 PASS + 2 SKIP。`UpsertDedup`/`DuplicateRejected`（唯一索引）通过；但 2 个级联删除测试（`TestDeleteWatchCascadesHits`、`TestDeleteWatchDoesNotAffectOtherWatchHits`）在 PG 黄金 schema 下**断言不成立**——根因：生产/黄金 schema 对 `topic_watch_hits.watch_id` **无 FK ON DELETE CASCADE**（AutoMigrate `DisableForeignKeyConstraintWhenMigrating=true` 不建 FK，版本化迁移仅加了 status CHECK + 复合唯一索引，无 FK；对比 `topic_tags` 在 `postgres_migrations.go:595` 显式加了 CASCADE）。即 `DeleteWatch` 在生产 PG 上会留孤儿 hits 行——这是本次迁移暴露的**潜在生产 bug**（正是本 change 要消灭的「SQLite 全绿」温床）。两个测试已 `t.Skip` 暂存（断言未改）。选项：(a) 加版本化迁移 `ALTER TABLE topic_watch_hits ADD CONSTRAINT ... FOREIGN KEY (watch_id) REFERENCES board_topic_watches(id) ON DELETE CASCADE`（推荐，对齐 model tag 意图）；(b) `DeleteWatch` 内手动级联删 hits；(c) 改断言为「留孤儿」（不推荐，掩盖 bug）。(a)/(b) 均超出本 change 3 文件/不动产品代码范围，交主线程定夺
- [x] 1.4 不加 `t.Parallel()`（黄金 schema 是进程级共享态，禁止并发 truncate 串台，见 testing.md 约束②）。验收：`grep -n 't.Parallel' internal/topicgraph/repository/topic_watch_repository_test.go` → 零命中

## 2. 拆分 daily_report_manual_topic_test.go

- [x] 2.1 新建 `internal/topicgraph/repository/daily_report_manual_topic_unit_test.go`，搬入 14 个纯函数测试（`AggregateEmbeddings`×6 / `DetectOutliers`×4 / `FloatsToPgVector`×3 / `PureFunctionDetection`）；移除其 DB setup 与 import，纯内存逻辑。验收：`go test -short ./internal/topicgraph/repository -run 'AggregateEmbeddings|DetectOutliers|FloatsToPgVector|PureFunction' -v` → 14/14 PASS（无 Docker）
- [x] 2.2 原 `daily_report_manual_topic_test.go` 仅留 8 个 DB 测试（`CreateManualTopic`×4 / `GetComposeCandidates`×4）；`setupManualTopicTestDB` 改为 `testutil.SetupTestDB(t)` + `NewTopicGraphRepository(db)`（另加 `Repo = repo`，因 `CreateManualTopic→RebuildBoardRelations` 经 tx helper 读包级 `Repo`，与同包其它 PG 集成测试一致）。验收：`grep -nE 'glebarez/sqlite|sqlite.Open' internal/topicgraph/repository/daily_report_manual_topic_test.go` → 零命中
- [ ] 2.3 vector seed 防坑：逐一核查 8 个 DB 测试的 seed 数据，凡涉及 embedding/vector 列 MUST 用 `repository.FloatsToPgVector(...)` 填合法值，不留非指针 string 零值空串（防 `SQLSTATE 22P02`，testing.md 已有先例）。验收：`go test ./internal/topicgraph/repository -run 'CreateManualTopic|GetComposeCandidates' -v` → 8/8 PASS，无 `invalid input syntax for type vector`  ⚠️ **vector seed 已全部修复 / 8/8 未达**：实测 7/8 PASS + 1 SKIP，**已无任何 `22P02` vector 报错**。修复三处空串 seed：① `TestCreateManualTopic_NoUsableEmbeddings` 的 section `Embedding:""` → `Omit("embedding")`(NULL)；② `TestGetComposeCandidates_ParsesEmbeddingsAndExcludesEmpty` 的 s3 同理 → NULL；③ `TestGetComposeCandidates_AttachesTopicBrief` 的 `BoardPersistentTopic` 零值 embedding → 补 `vecStr(0.2,0.1)`。8/8 未达的唯一原因是 `TestCreateManualTopic_RollbackOnRebuildRelationsFailure` 被 `t.Skip`——该测试是 **SQLite 专用**（前提「RebuildBoardRelations 用 PG 专 SQL 在 SQLite 必败」），迁 PG 后前提**反转**（PG 下成功），所有 `assert.Error/Nil` 断言翻转，非 vector 问题。PG happy-path 已由 `TestManualTopic_CreateAndReassign`（`daily_report_topic_integration_test.go`）覆盖。选项：(a) 删除该测试（SQLite 遗物，PG 下失效）；(b) 改造为 PG 上真实中事务失败触发回滚；(c) 转为 happy-path（与 TestManualTopic_CreateAndReassign 重复）。交主线程定夺
- [x] 2.4 两个文件 import 各自独立、无跨文件残留依赖。验收：`go build ./internal/topicgraph/repository` → BUILD_OK

## 3. 增补 testing.md 硬约束段（spec delta 已于 propose 完成）

> spec delta（`openspec/changes/repository-tests-pg-only/specs/test-infrastructure/spec.md` ADDED requirement「数据访问层测试禁用内存 SQLite」）已写好，archive 时按 §12 sync 进主 spec。本节只做人读文档增补。

- [ ] 3.1 `docs/reference/standard/backend/testing.md`「集成测试常见陷阱」节增补一条「repository 层禁 SQLite」硬约束：SHALL 用 `testutil.SetupTestDB`、SHALL NOT 用内存 SQLite 测数据访问逻辑；附判定法（是否在 `*/repository/` 包 + 是否碰 DB）；引用既有 4 起事故作前车之鉴；注明纯函数测试归 `_unit_test.go` 豁免。验收：`grep -n 'repository' docs/reference/standard/backend/testing.md` → 含新约束段关键词

## 4. 文档

<!-- doc-impact: standard -->

- [ ] 4.1 `docs/reference/standard/backend/testing.md`：增补「repository 层禁 SQLite」硬约束段（任务 3.1）

## 5. 测试（§11.2）

> 归档前重跑，确认零失败。后端命令走 WSL；testcontainer 集成测试需 Docker daemon 可用（`docker compose -f docker-compose.pg.yml` 不必起，testcontainer 自管隔离容器）。

- [ ] T.1 `cd backend-go && go test -short ./internal/topicgraph/repository` → 纯函数测试 PASS（14 个，无 Docker 依赖）
- [ ] T.2 `cd backend-go && go test ./internal/topicgraph/repository` → 全部 PASS（含迁移后 10 watch + 8 manual_topic DB 测试，真 PG + 黄金 schema）
- [ ] T.3 `grep -rnE 'glebarez/sqlite|sqlite\.Open' backend-go/internal/topicgraph/repository/` → 零命中（repository 层清零 SQLite 验证）

## 6. 验证（§11.2，归档前实测）

- [ ] V.1 `cd backend-go && go build ./...` → BUILD_OK
- [ ] V.2 `cd backend-go && go vet ./internal/topicgraph/...` → VET_OK
- [ ] V.3 `cd backend-go && golangci-lint run ./internal/topicgraph/...` → 0 issues
- [ ] V.4 `bash scripts/check-standards.sh` → A-D 段零失败（E 段归档后校验，F 段 doc-impact verify、G 段死链检查归档前零失败）
- [ ] V.5 `bash scripts/doc-impact.sh verify openspec/changes/repository-tests-pg-only` → 对账零失败（声明 standard 域 ↔ 实际改动 testing.md 一致）
- [ ] V.6 `openspec validate repository-tests-pg-only --strict`（若 CLI 支持）→ proposal/design/specs/tasks 四件套校验通过
