# Speed Up Testcontainer Setup — Implementation Plan

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 把 `topicgraph/repository` 集成测试从 ~355s 降到 ~60–90s，靠进程级一次性黄金 schema + 测试间 `ResetTestData`（seed 快照恢复），`SetupTestDB` 调用点零改动。

**Architecture:** testutil 引入 `migrateOnce sync.Once`：进程内首次 `SetupTestDB` 跑一次 `runTestMigrations` 建黄金 schema + 拍 seed 快照（`ai_settings`/`embedding_config` 运行时读，非手工副本），后续调用走 `ResetTestData`（`TRUNCATE ... RESTART IDENTITY CASCADE` 所有业务表，跳过 `schema_migrations`，再从快照灌回 seed）。`ReimportTestDB`（DROP+重建）原样留作逃生口。

**Tech Stack:** Go 1.25 + testcontainers-go + GORM + pgvector。测试基建改动，不碰生产代码。

---

## 全局约束（每个 Task 都遵守）

- **Go 走 Windows cmd**：`go` 装在 `D:\tool\Go`，WSL/ctx 沙箱无 go。所有 go 命令必须：
  `cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go <cmd>"`
- **测试只跑影响包**：不跑 `go test ./...`（AGENTS.md 红线），只跑 `internal/platform/testutil` 及当前 Task 影响的包。
- **TDD 强制**：每个 Task 先写红测试看失败，再写实现转绿，再重构（执行规范 §2）。
- **commit author**：`zanebonoalter <380207345@qq.com>`。工作树有无关脏改动（develop 分支），**只 add 本 change 的文件**，不 `git add -A`。
- **DSN 安全红线**（§6.2）：testutil 不得有默认 DSN/读环境变量/连开发库。改动不得引入这些路径。

**权威依据**：
- `openspec/changes/speed-up-testcontainer-setup/design.md`（决策①–⑤，含快照方案）
- `openspec/changes/speed-up-testcontainer-setup/specs/test-infrastructure/spec.md`
- `docs/reference/standard/backend/testing.md`（OID 漂移陷阱、DSN 红线）

**已预核实事实**（apply 时复核）：
- 受影响包（topicgraph/tagmanagement/testutil）`t.Parallel()` 零命中 → 共享 schema 串行假设成立。
- tagmanagement 测试往 `ai_settings` 插自定义键（9 处）→ 必须快照恢复而非跳过表。
- 迁移 seed 仅落 `ai_settings` + `embedding_config` 两张表。

---

## Task 0: 前置审计 + 基线（无代码改动）

**Files:** 无（只读 + 记录）

**Step 1: 复核并发前提**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && findstr /S /N /R /C:"t\.Parallel()" internal\topicgraph\*.go internal\tagmanagement\*.go internal\platform\testutil\*.go"
```
Expected: 零命中（或仅命中非 _test.go）。若有 `_test.go` 命中 → 停下报告，不能进 Task 1。

**Step 2: 量化基线耗时**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository 2>&1 | findstr /R /C:"ok\|FAIL\|---""
```
Expected: `ok ... <NNNN.NN>s`（记录秒数，预期 ~300–355s）。**记下这个数字**，Task 3 用作对照。

**Step 3: 确认 testutil 包当前可编译可测**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./internal/platform/testutil/... && echo BUILD_OK"
```
Expected: `BUILD_OK`

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run TestReimportPreservesVectorInserts -v -count=1"
```
Expected: PASS（现有回归守卫生效）。

**Step 4: 无需 commit**（纯基线）

---

## Task 1: ResetTestData + takeSeedSnapshot（TDD）

**Files:**
- Modify: `backend-go/internal/platform/testutil/testutil.go`（新增包级变量、`takeSeedSnapshot`、`ResetTestData`）
- Test: `backend-go/internal/platform/testutil/reset_test_data_test.go`（新建）

**Step 1: 写红测试**

创建 `backend-go/internal/platform/testutil/reset_test_data_test.go`：

```go
package testutil

import (
	"testing"

	"github.com/stretchr/testify/require"
	"gorm.io/gorm"
	"syntopica-backend/internal/models"
)

// ResetTestData must: (a) truncate all business tables, (b) restore migration
// seed rows from the golden-schema snapshot, (c) clear custom ai_settings keys
// that tests inserted, (d) preserve schema_migrations.
func TestResetTestData_ClearsBusinessTables(t *testing.T) {
	db := SetupTestDB(t)
	takeSeedSnapshot(db) // establish snapshot from the clean post-migration state

	require.NoError(t, db.Create(&models.TopicTag{Name: "tt-1"}).Error)
	var n int64
	db.Model(&models.TopicTag{}).Count(&n)
	require.EqualValues(t, 1, n)

	ResetTestData(t, db)

	db.Model(&models.TopicTag{}).Count(&n)
	require.EqualValues(t, 0, n, "ResetTestData must clear business tables")
}

func TestResetTestData_RestoresSeedAndClearsCustomKeys(t *testing.T) {
	db := SetupTestDB(t)
	takeSeedSnapshot(db)

	// Simulate a test inserting a non-seed custom ai_settings key
	// (mirrors tagmanagement/service/board tests).
	require.NoError(t, db.Create(&models.AISettings{
		Key: "semantic_board_match_sim_threshold", Value: "0.6",
	}).Error)

	ResetTestData(t, db)

	var custom models.AISettings
	err := db.Where("key = ?", "semantic_board_match_sim_threshold").First(&custom).Error
	require.ErrorIs(t, err, gorm.ErrRecordNotFound,
		"custom (non-seed) ai_settings key must be cleared by ResetTestData")

	// Seed rows must still be present (restored from snapshot).
	var seedCount int64
	db.Model(&models.AISettings{}).Count(&seedCount)
	require.Greater(t, seedCount, int64(0), "migration seed ai_settings rows must be restored")
}

func TestResetTestData_PreservesSchemaMigrations(t *testing.T) {
	db := SetupTestDB(t)
	takeSeedSnapshot(db)

	ResetTestData(t, db)

	var n int64
	db.Raw("SELECT count(*) FROM schema_migrations").Scan(&n)
	require.Greater(t, n, int64(0), "schema_migrations must survive ResetTestData")
}

func TestResetTestData_IdempotentAcrossCalls(t *testing.T) {
	db := SetupTestDB(t)
	takeSeedSnapshot(db)

	ResetTestData(t, db)
	ResetTestData(t, db)
	ResetTestData(t, db)

	var seedCount int64
	db.Model(&models.AISettings{}).Count(&seedCount)
	require.Greater(t, seedCount, int64(0))
}
```

**Step 2: 运行测试，验证失败**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run TestResetTestData -v -count=1"
```
Expected: 编译失败 `undefined: takeSeedSnapshot` 和 `undefined: ResetTestData`（红，失败原因正确）。

**Step 3: 写最小实现**

在 `testutil.go` 改动：

3a. 包级变量区（在现有 `var (` 块内追加）：

```go
	// Golden-schema state. migrateOnce builds the schema once per process;
	// every later SetupTestDB call resets via ResetTestData (fast path).
	migrateOnce      sync.Once // runs runTestMigrations once per process
	goldenSchemaErr  error     // captures first-build error, surfaced on every call
	seedSnapshotMu   sync.Mutex
	aiSettingsSeed   []models.AISettings
	embeddingCfgSeed []models.EmbeddingConfig
```

在 `var (` 块下方、`OpenTestDB` 之前，加：

```go
// seedSnapshotTables names the tables whose migration seed rows must survive
// ResetTestData. Content is read at runtime from the golden schema (never
// hardcoded), so it cannot drift from production migrations. New seed tables
// added by a future migration MUST be registered here too.
var seedSnapshotTables = []string{"ai_settings", "embedding_config"}
```

3b. 新增 `takeSeedSnapshot` 和 `ResetTestData`（放在 `TruncateAllTables` 之后）：

```go
// takeSeedSnapshot reads the current seed rows of ai_settings/embedding_config
// from the golden schema into package-level slices. MUST be called immediately
// after runTestMigrations, before any test mutates those tables. Content comes
// from the DB at runtime — there is no hardcoded seed copy in testutil.
func takeSeedSnapshot(db *gorm.DB) error {
	aiSettingsSeed = nil
	embeddingCfgSeed = nil
	if err := db.Find(&aiSettingsSeed).Error; err != nil {
		return fmt.Errorf("snapshot ai_settings: %w", err)
	}
	if err := db.Find(&embeddingCfgSeed).Error; err != nil {
		return fmt.Errorf("snapshot embedding_config: %w", err)
	}
	return nil
}

// ResetTestData resets the golden schema to the "fresh production startup"
// state for the next test: TRUNCATE all business tables (RESTART IDENTITY
// CASCADE, skipping schema_migrations) then restore the migration seed rows
// snapshotted right after the golden schema was built.
//
// This is the fast-path reset used between tests once the golden schema exists.
// TruncateAllTables (no snapshot restore) is retained for callers that
// deliberately want a truly empty DB including seeds.
func ResetTestData(t *testing.T, db *gorm.DB) {
	t.Helper()

	seedSnapshotMu.Lock()
	defer seedSnapshotMu.Unlock()

	if aiSettingsSeed == nil && embeddingCfgSeed == nil {
		t.Fatal("ResetTestData: seed snapshot not taken; golden schema must be built first")
	}

	var tables []string
	if err := db.Raw(`SELECT table_name FROM information_schema.tables
		WHERE table_schema = 'public' AND table_type = 'BASE TABLE'
		AND table_name <> 'schema_migrations'`).Scan(&tables).Error; err != nil {
		t.Fatalf("list tables: %v", err)
	}
	if len(tables) > 0 {
		if err := db.Exec(fmt.Sprintf("TRUNCATE TABLE %s RESTART IDENTITY CASCADE",
			strings.Join(tables, ", "))).Error; err != nil {
			t.Fatalf("truncate tables: %v", err)
		}
	}

	// Restore migration seed rows from the golden-schema snapshot.
	if len(aiSettingsSeed) > 0 {
		if err := db.Create(&aiSettingsSeed).Error; err != nil {
			t.Fatalf("restore ai_settings seed: %v", err)
		}
	}
	if len(embeddingCfgSeed) > 0 {
		if err := db.Create(&embeddingCfgSeed).Error; err != nil {
			t.Fatalf("restore embedding_config seed: %v", err)
		}
	}

	database.DB = db
}
```

> 注意：`models` 和 `database` 包已在文件 import 中（`syntopica-backend/internal/platform/database`），`models` 需确认是否已 import；若未 import，加 `"syntopica-backend/internal/models"`。

**Step 4: 运行测试，验证通过**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run TestResetTestData -v -count=1"
```
Expected: 4 个测试全 PASS。

**Step 5: 回归（确认没破坏现有 testutil 测试）**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -v -count=1"
```
Expected: 含 `TestReimportPreservesVectorInserts` 在内全部 PASS。

**Step 6: Commit**

```
cd /mnt/d/project/Syntopica
git add backend-go/internal/platform/testutil/testutil.go backend-go/internal/platform/testutil/reset_test_data_test.go
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "test(testutil): add ResetTestData with seed snapshot restore

TDD: ResetTestData truncates business tables (RESTART IDENTITY CASCADE,
skipping schema_migrations) and restores ai_settings/embedding_config seed
rows from a runtime snapshot taken after golden-schema build. Foundation
for the fast reset path in SetupTestDB (next task).

Refs: openspec/changes/speed-up-testcontainer-setup design decision ③"
```

---

## Task 2: 黄金 schema 单次构建 migrateOnce（TDD）

**Files:**
- Modify: `backend-go/internal/platform/testutil/testutil.go`（`SetupTestDB` 改造 + 可观测计数器）
- Test: `backend-go/internal/platform/testutil/golden_schema_test.go`（新建）

**Step 1: 写红测试**

创建 `backend-go/internal/platform/testutil/golden_schema_test.go`：

```go
package testutil

import (
	"sync/atomic"
	"testing"

	"github.com/stretchr/testify/require"
	"syntopica-backend/internal/models"
)

// SetupTestDB must build the golden schema (runTestMigrations) only ONCE per
// process; subsequent calls reset via ResetTestData without rebuilding.
func TestSetupTestDB_MigratesOncePerProcess(t *testing.T) {
	before := atomic.LoadInt64(&migrationsRunCount)

	SetupTestDB(t)
	SetupTestDB(t)
	SetupTestDB(t)

	after := atomic.LoadInt64(&migrationsRunCount)
	require.Equal(t, before, after,
		"runTestMigrations must not re-run after the golden schema is built; got delta=%d", after-before)
}

// After the golden schema is built, repeated ResetTestData cycles must NOT
// trigger the pgvector OID drift bug (no DROP SCHEMA => stable OID).
func TestGoldenSchema_OIDStableAcrossResets(t *testing.T) {
	db := SetupTestDB(t)

	for cycle := 1; cycle <= 5; cycle++ {
		require.NoError(t, db.Create(&models.SemanticLabel{
			Label:     "oid-stability-probe",
			Slug:      "oid-stability-probe",
			LabelType: "board",
			Status:    "active",
		}).Error, "cycle %d: vector-bearing insert must succeed", cycle)

		// Vector cast must resolve (OID not broken).
		var v string
		require.NoError(t, db.Raw("SELECT '[0]'::vector::text").Row().Scan(&v),
			"cycle %d: vector cast failed => OID drift", cycle)

		ResetTestData(t, db)
	}
}
```

**Step 2: 运行测试，验证失败**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run "TestSetupTestDB_MigratesOncePerProcess|TestGoldenSchema" -v -count=1"
```
Expected: 编译失败 `undefined: migrationsRunCount`（红）。

**Step 3: 写最小实现**

3a. 加可观测计数器（包级变量区）：

```go
	// migrationsRunCount is a test-only observable: how many times
	// runTestMigrations executed in this process. SetupTestDB's migrateOnce
	// must ensure this stops incrementing after the first call.
	migrationsRunCount int64
```

3b. `runTestMigrations` 顶部加计数（在函数体第一行 `db.Exec("CREATE EXTENSION...")` 之前）：

```go
func runTestMigrations(db *gorm.DB) error {
	atomic.AddInt64(&migrationsRunCount, 1)
	// Enable pgvector extension (mirrors the first production migration).
	...（原内容不变）
}
```

> 需在 import 加 `"sync/atomic"`。

3c. 改造 `SetupTestDB`（替换整个函数体）：

```go
// SetupTestDB is the single entry point for integration tests.
// It:
//  1. Skips when running with -short flag.
//  2. Starts (or reuses) the isolated pgvector container.
//  3. On the FIRST call of the process, builds the golden schema once
//     (runTestMigrations) and snapshots migration seed rows.
//  4. On every LATER call, resets via the fast path (ResetTestData) instead of
//     rebuilding the schema — this is the ~6x speedup.
//  5. Sets database.DB for production code compatibility.
//
// Every integration test should start with: db := testutil.SetupTestDB(t)
func SetupTestDB(t *testing.T) *gorm.DB {
	t.Helper()

	if testing.Short() {
		t.Skip("requires Postgres (testcontainers)")
	}

	db := OpenTestDB(t)

	migrateOnce.Do(func() {
		if err := runTestMigrations(db); err != nil {
			goldenSchemaErr = fmt.Errorf("build golden schema: %w", err)
			return
		}
		if err := takeSeedSnapshot(db); err != nil {
			goldenSchemaErr = fmt.Errorf("snapshot seed: %w", err)
			return
		}
	})
	if goldenSchemaErr != nil {
		t.Fatalf("%v", goldenSchemaErr)
	}

	// Whether this was the first call (just built) or a later one: ensure a
	// clean seed-only state. On first call the schema is already fresh from
	// runTestMigrations; ResetTestData on an already-clean DB is a cheap
	// no-op-ish truncate that also sets database.DB. To avoid an unnecessary
	// truncate on the very first call, detect first-call via the counter.
	if atomic.AddInt64(&setupCallCount, 1) == 1 {
		database.DB = db
		return db
	}
	ResetTestData(t, db)
	return db
}
```

> 需在包级变量区加 `setupCallCount int64`。

**Step 4: 运行测试，验证通过**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -run "TestSetupTestDB_MigratesOncePerProcess|TestGoldenSchema" -v -count=1"
```
Expected: 2 个测试 PASS。

> ⚠️ 注意：`migrationsRunCount` 在测试进程里可能因其他测试先触发过 migrateOnce 而 >1。本测试只验证"三次 SetupTestDB 不增加计数"，所以 `before == after` 即正确，与初始值无关。

**Step 5: 全 testutil 包回归**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/platform/testutil -v -count=1"
```
Expected: 全部 PASS（含 Task 1 的 4 个 + 现有 `TestReimportPreservesVectorInserts` + Task 2 的 2 个）。

**Step 6: Commit**

```
cd /mnt/d/project/Syntopica
git add backend-go/internal/platform/testutil/testutil.go backend-go/internal/platform/testutil/golden_schema_test.go
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "perf(testutil): build golden schema once via migrateOnce

SetupTestDB now runs runTestMigrations exactly once per process (building
the golden schema + seed snapshot); every subsequent call resets via
ResetTestData instead of DROP+rebuild. Eliminates the pgvector OID-drift
reconnect dance on the hot path. ReimportTestDB retained as escape hatch.

Refs: openspec/changes/speed-up-testcontainer-setup design decision ①④⑤"
```

---

## Task 3: 提速验证 + 修复 truncate 暴露的耦合

**Files:** 可能 Modify 若干 `internal/topicgraph/**/*_test.go`、`internal/tagmanagement/**/*_test.go`（仅在测试因隐式耦合失败时；这是测试质量提升，非缺陷）

**Step 1: 跑 topicgraph/repository，量化提速**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository 2>&1 | findstr /R /C:"ok\|FAIL\|---""
```
Expected: `ok ... <NN>s`，且 **NN 远小于 Task 0 基线**（目标 ≤90s）。

**Step 2: 若有 FAIL —— 修复隐式数据耦合**

如果某个测试失败，最可能原因是它偷偷依赖上个测试残留的数据。修复方式：让该测试在开头显式 seed 自己需要的数据（用 `seedTestBoard`/`seedTestReport` 等现有 helper），不依赖残留。

- 每个失败用例单独修，修完立即重跑该包确认绿。
- **不要**为了"让测试过"去改 ResetTestData 语义（快照恢复是正确的）。
- 记录修复的用例（commit message 说明）。

Run（反复，直到全绿）:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/repository -run <失败的Test> -v"
```

**Step 3: 全受影响包回归**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./internal/topicgraph/... ./internal/tagmanagement/... ./internal/platform/testutil"
```
Expected: 全部 PASS。

**Step 4: Commit（若有耦合修复）**

```
cd /mnt/d/project/Syntopica
git add backend-go/internal/topicgraph/<改动的测试文件> backend-go/internal/tagmanagement/<改动的测试文件>
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "test: fix implicit cross-test data coupling exposed by ResetTestData

Golden-schema + truncate reset surfaced tests that depended on residue
from a previous test. Each now seeds its own data explicitly.

Refs: openspec/changes/speed-up-testcontainer-setup design risk #1"
```

> 若无任何耦合修复（全部一次绿），跳过 commit，记录"零耦合暴露"。

---

## Task 4: 文档同步 + 收尾门禁

**Files:**
- Modify: `docs/reference/standard/backend/testing.md`
- Modify: `docs/reference/开发执行规范.md`（§6.0 如需）
- Modify: `openspec/changes/speed-up-testcontainer-setup/README.md`

**Step 1: 更新 testing.md**

在 testing.md 的「`testutil.SetupTestDB` 使用模式」小节后，新增「黄金 schema 模式」子节，内容：

- 进程级 `migrateOnce`：首次 `SetupTestDB` 跑一次迁移建黄金 schema + 拍 seed 快照，后续走 `ResetTestData`（truncate + 快照恢复）。
- `ResetTestData(t, db)`：测试间重置，清业务表、保留 `schema_migrations`、从快照恢复 `ai_settings`/`embedding_config` seed。
- `TruncateAllTables`：清空一切（含 seed），原义保留。
- `ReimportTestDB`：DROP+重建逃生口，OID 修复逻辑不删。
- **约束①**：新增迁移 seed 表 MUST 同步加入 `seedSnapshotTables`（testutil.go）。
- **约束②**：共享 schema 模型下不得对集成测试加 `t.Parallel()`。

**Step 2: 修正 README.md 措辞**

把 `openspec/changes/speed-up-testcontainer-setup/README.md` 里"每次 SetupTestDB 调用都重起 pgvector testcontainer"改为"每次 SetupTestDB 调用都重建 schema（容器已单例复用）"，与 proposal 根因一致。

**Step 3: 质量门禁（执行规范 §4.1）**

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && golangci-lint run ./... 2>&1"
```
Expected: 0 错误。

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go vet ./... && echo VET_OK"
```
Expected: `VET_OK`

Run:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go build ./... && echo BUILD_OK"
```
Expected: `BUILD_OK`

Run（全量，归档门禁）:
```
cmd.exe /C "cd /d D:\project\Syntopica\backend-go && go test ./... 2>&1 | findstr /R /C:"FAIL\|--- FAIL""
```
Expected: 零命中（无 FAIL）。注意全量较慢（所有包），这是归档前的一次性确认。

**Step 4: 架构体检（执行规范 §7.1）**

Run:
```
codegraph_codegraph_impact(symbol="SetupTestDB")
codegraph_codegraph_impact(symbol="ResetTestData")
codegraph_codegraph_impact(symbol="ReimportTestDB")
```
Expected: 无 HIGH/CRITICAL 被忽略。若有 → 暂停报告。

**Step 5: Commit**

```
cd /mnt/d/project/Syntopica
git add docs/reference/standard/backend/testing.md docs/reference/开发执行规范.md openspec/changes/speed-up-testcontainer-setup/README.md
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "docs(testing): document golden-schema mode + ResetTestData

Add golden-schema/reset conventions to backend testing.md; fix change
README wording (container is singleton, not restarted per call).

Refs: openspec/changes/speed-up-testcontainer-setup tasks §6"
```

---

## 验收清单（全部完成后）

- [ ] Task 0 基线秒数已记录
- [ ] Task 1：ResetTestData 4 测试 PASS
- [ ] Task 2：migrateOnce 2 测试 PASS，OID 稳定测试 PASS
- [ ] Task 3：topicgraph/repository 提速到 ≤90s，受影响包全绿
- [ ] Task 4：lint/vet/build/test 全量 0 失败，testing.md 已更新
- [ ] 提速倍数 = Task0基线 / Task3结果，写入 change 验证记录
- [ ] 设计决策③（快照方案）已反映在 design.md 和 spec.md（controller 已完成）
