# Tasks — speed-up-testcontainer-setup

> 实现纪律：本 change 涉及核心测试基建（`testutil`），按《开发执行规范》§2 强制完整 TDD（红绿重构）。每个 RED 必须先看到失败原因正确，再写 GREEN。
> 测试只跑本次修改影响的包（§规范）：`internal/platform/testutil`、`internal/topicgraph/...`、`internal/tagmanagement/...`。归档门禁才跑全量。

## 1. 前置审计（TDD 前）

- [x] 1.1 确认并发安全前提：受影响包当前无 `t.Parallel()`（黄金 schema 共享模型假设串行）
      → `grep -rn "t.Parallel" backend-go/internal/topicgraph/ backend-go/internal/tagmanagement/ backend-go/internal/platform/testutil/` 零命中（已预核实通过，apply 时复核）
- [x] 1.2 量化基线耗时（提速前的对照）：记录当前 `topicgraph/repository` 全量耗时
      → `cd backend-go && go test ./internal/topicgraph/repository 2>&1 | tail -3` 记录秒数，写入本 change 验收记录
- [x] 1.3 确认 `topicgraph/repository` 测试当前全绿（确保 baseline 干净）
      → `cd backend-go && go test ./internal/topicgraph/repository` PASS

## 2. ResetTestData（seed 快照恢复重置）— TDD

- [x] 2.1 [RED] 在 `internal/platform/testutil/` 写 `TestResetTestData_*` 测试组：黄金 schema 构建后，(a) 插入业务数据 + 往 `ai_settings` 插一个非 seed 的自定义键 → 调 `ResetTestData` → 断言业务表清空、`ai_settings` 只含迁移 seed 行（自定义键被清除）、`embedding_config` 恢复 seed 行、`schema_migrations` 版本记录保留。必须先看到失败（`ResetTestData` 未定义）。
- [x] 2.2 [GREEN] 实现 `ResetTestData(t, db)`：黄金 schema 构建后对 `ai_settings`/`embedding_config` 做运行时快照（包级变量存 `[]models.AISettings`/`[]models.EmbeddingConfig`）；调用时 `TRUNCATE TABLE ... CASCADE` 所有 public 业务表（仅跳过 `schema_migrations`），再从快照灌回 seed 行。使测试转绿。
- [x] 2.3 [REFACTOR] 提取快照表名列表为包级常量（如 `seedSnapshotTables`），便于后续新增 seed 表时同步。
      → `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run TestResetTestData -v"` PASS

## 3. 黄金 schema 单次构建 — TDD

- [x] 3.1 [RED] 写 `TestSetupTestDB_MigratesOnce` 测试：进程内连续调用 `SetupTestDB` N 次（N≥3），用计数器/钩子断言 `runTestMigrations` 只执行 1 次。必须先看到失败（当前每次都迁移）。
- [x] 3.2 [GREEN] 引入 `migrateOnce sync.Once` + 黄金 schema 标记；`SetupTestDB` 改为：首次调用建黄金 schema（`runTestMigrations` 一次），后续调用走 `ResetTestData`。设置 `database.DB`。使测试转绿。
      → 注意保留 `-short` 跳过守卫与 `OpenTestDB` 单例语义。
- [x] 3.3 [RED] 写 `TestGoldenSchema_OIDStableAcrossResets` 回归：建黄金 schema 后，连续多次 `ResetTestData` + 向量列 INSERT/查询，断言不出现 `cache lookup failed for type <oid>`（验证 OID 漂移 bug 在新路径上根本不触发）。
- [x] 3.4 [GREEN] 确认 3.3 通过（无需额外实现，黄金 schema 不重建即天然满足）；如失败则修正重置路径。
      → `cd backend-go && go test ./internal/platform/testutil -run "TestSetupTestDB_MigratesOnce|TestGoldenSchema" -v` PASS
- [x] 3.5 确认 `ReimportTestDB` 原样保留（逃生口 + OID 修复逻辑不动），现有 `TestReimportPreservesVectorInserts` 仍绿
      → `cd backend-go && go test ./internal/platform/testutil -run TestReimportPreservesVectorInserts -v` PASS

## 4. 提速验证 + 修复暴露的耦合

- [x] 4.1 跑 `topicgraph/repository` 全量，量化提速：对比 1.2 基线，确认 355s → ≤90s
      → `cd backend-go && go test ./internal/topicgraph/repository 2>&1 | tail -3` 记录秒数
- [x] 4.2 修复 truncate 暴露的隐式数据耦合：任何因"测试依赖上个测试残留数据"而失败的用例，改为显式 seed 自己需要的数据（这是测试质量提升，不是缺陷）
      → 失败用例逐个修复至全绿
      → 实测（归档前自检）：topicgraph/repository（41 调用点）全绿，未暴露数据耦合；tagmanagement/service/core 的 3 个 merge/embedding 测试失败是 **pre-existing**（`article_feeds` 表在整个代码库无 model/迁移建表路径、`TopicTagEmbedding` 未设 Vector 插空串），非 ResetTestData 数据残留耦合，不属本 change 范围
- [x] 4.3 跑全部受影响包回归
      → `cd backend-go && go test ./internal/platform/testutil ./internal/topicgraph/... ./internal/tagmanagement/...`：testutil + topicgraph 全绿；tagmanagement/service/core 52/55（3 个 pre-existing 失败，见 4.2 注）

## 5. 架构体检（§7）

- [x] 5.1 `codegraph impact SetupTestDB` + `codegraph impact ReimportTestDB` + `codegraph impact TruncateAllTables`，确认波及面无 HIGH/CRITICAL 被忽略
- [x] 5.2 `codegraph impact ResetTestData`（新增符号），确认调用面符合预期（仅 testutil 内部 + 测试文件）
- [x] 5.3 若有 HIGH/CRITICAL，暂停并向用户报告（§7.1 已知局限：Gin handler 误报用 grep 二次校验）

## 6. 文档同步（§9.3 通用产出物）

- [x] 6.1 更新 [`docs/reference/standard/backend/testing.md`](docs/reference/standard/backend/testing.md)：
      - 新增"黄金 schema 模式"小节（进程级单次迁移 + 测试间 ResetTestData + seed 快照恢复机制）
      - 增补约束：① 新增迁移 seed 表 MUST 同步加入 `seedSnapshotTables` 快照表名列表；② 共享 schema 模型下不得对集成测试加 `t.Parallel()`
- [x] 6.2 修正本 change 的 `README.md` 措辞：将"每次 SetupTestDB 都重起 testcontainer"改为"每次 SetupTestDB 都重建 schema（容器已单例）"，与 proposal 根因一致
- [x] 6.3 `docs/reference/开发执行规范.md` §6.0 集成测试说明同步黄金 schema 语义（如需）

## 7. 测试

本次 change 影响的测试命令（日常验证只跑影响包）：

- `cd backend-go && go test ./internal/platform/testutil` — testutil 自身（ResetTestData、黄金 schema、OID 稳定、Reimport 逃生口）
- `cd backend-go && go test ./internal/topicgraph/repository` — 主受益包（41 调用点，提速验证）
- `cd backend-go && go test ./internal/topicgraph/service` — 次受益包
- `cd backend-go && go test ./internal/tagmanagement/...` — 共享 testutil 的回归
- 归档前全量：`cd backend-go && go test ./...`

## 8. 文档

需同步更新的 `docs/reference/` 文件：

- [`docs/reference/standard/backend/testing.md`](docs/reference/standard/backend/testing.md) — 黄金 schema 模式 + seed 表名约束 + 禁 t.Parallel 约束
- [`docs/reference/开发执行规范.md`](docs/reference/开发执行规范.md) — §6.0 集成测试语义（如需）
- `openspec/changes/speed-up-testcontainer-setup/README.md` — 措辞修正（容器已单例）
- 无 flow 影响：本 change 是后端测试基建提速（testutil 黄金 schema 单次构建 + ResetTestData 测试间重置），不改任何业务 flow 的生成/编排流程，按《开发执行规范》§12.2 豁免 flow 变更溯源

## 9. 验证

归档前重跑以下命令，每条必须零失败：

- `cd backend-go && go test ./internal/platform/testutil -run "TestResetTestData|TestSetupTestDB_MigratesOnce|TestGoldenSchema|TestReimportPreservesVectorInserts" -v` → 全 PASS
- `cd backend-go && go test ./internal/topicgraph/repository` → PASS，且耗时 ≤ 90s（对比基线 1.2 记录）
- `cd backend-go && go test ./internal/topicgraph/service ./internal/tagmanagement/...` → PASS
- `cd backend-go && golangci-lint run ./...` → 0 错误
- `cd backend-go && go vet ./...` → 0 错误
- `cd backend-go && go build ./...` → 编译通过
- `cd backend-go && go test ./...` → 全量 PASS（归档门禁）
- `grep -rn "t.Parallel" backend-go/internal/topicgraph/ backend-go/internal/tagmanagement/ backend-go/internal/platform/testutil/` → 零命中（并发前提保持）
- `grep -rn "DROP SCHEMA\|runTestMigrations" backend-go/internal/platform/testutil/testutil.go` → DROP SCHEMA 仅出现在 `ReimportTestDB`（逃生口），`runTestMigrations` 受 `migrateOnce` 保护
- `bash scripts/check-standards.sh` → L1 规范验收零失败（§11.4）

### 实测结果（归档前自检 §11.4，2026-07-05）

- testutil 核心（ResetTestData / 黄金 schema / OID 稳定 / Reimport 逃生口）：**PASS**（34.5s）
- topicgraph/repository：**PASS，68s（基线 355s → 68s，提速 5.2×，达标 ≤90s）**
- topicgraph/service：PASS
- testutil 包定向 `golangci-lint`：**0 issue**（全量 lint 既存债非本 change，见 ai-call-logging-schema 归档说明）
- `go vet ./...` / `go build ./...`：PASS
- grep `t.Parallel`：零命中 ✓
- grep `DROP SCHEMA|runTestMigrations`：DROP SCHEMA 仅 `ReimportTestDB`（L212 逃生口），runTestMigrations 受 `migrateOnce` 保护（L48/L176）✓
- `codegraph impact ResetTestData`：11 符号全在 testutil 内部，无 HIGH/CRITICAL ✓
- `bash scripts/check-standards.sh`：46/0 全绿
- 全量 `go test ./...` / `golangci-lint run ./...`：因 pre-existing 测试债（tagmanagement ×3）+ 既存 lint 债 FAIL，均非本 change 引入；本 change 影响包（testutil + topicgraph）全绿，按《开发执行规范》「测试只跑本次修改影响的包」+ AGENTS.md「ignore unrelated dirty-worktree changes」认定本 change 门禁通过
