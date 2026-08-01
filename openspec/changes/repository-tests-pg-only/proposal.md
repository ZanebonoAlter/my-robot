## Why

`topicgraph/repository` 是直接发 SQL 的数据访问层，却仍有 2 个测试文件用内存 SQLite 测 DB 行为——`topic_watch_repository_test.go`（10 个 CRUD 测试整文件）与 `daily_report_manual_topic_test.go` 中的 8 个 DB 测试。SQLite 对 GROUP BY 语义、JSONB/vector 类型、约束都宽松，PostgreSQL 严格，二者语义不一致是「SQLite 全绿、生产 PG 炸」的直接温床。`standard/backend/testing.md` 已记录至少 4 起同类事故（GROUP BY 缺列、vector 空串、JSONB 空串、迁移 NOT NULL）。

同包其余 9 个文件早已迁到 `testutil.SetupTestDB`（testcontainer PG + 黄金 schema），这最后一片半是残留缺口。本次补齐迁移，并立一条硬规矩（repository 包禁 SQLite）防止日后回退——否则今天清零、明天又有人写回假 DB。

## What Changes

- **迁移 `topic_watch_repository_test.go`**：删 `sqlite.Open` + 手动 `AutoMigrate`，改用 `testutil.SetupTestDB(t)`。10 个 CRUD 测试（含级联删除、复合唯一去重）在真 PG 下验证。`BoardTopicWatch`/`TopicWatchHit` 已在生产 schema，黄金 schema 自动建表，无额外 seed 配置。
- **拆分 `daily_report_manual_topic_test.go`**：14 个纯函数测试（AggregateEmbeddings×6、DetectOutliers×4、FloatsToPgVector×3、PureFunctionDetection）拆到新文件 `daily_report_manual_topic_unit_test.go`（`-short` 可跑、零 DB）；8 个 DB 测试（CreateManualTopic×4、GetComposeCandidates×4）迁到 PG。
- **立规矩（治本）**：`test-infrastructure` spec 新增 requirement——`*/repository/` 包下的测试 SHALL 使用 `testutil.SetupTestDB`（testcontainer PG），SHALL NOT 使用内存 SQLite 测数据访问逻辑；`standard/backend/testing.md` 同步增补该硬约束。纯算法/纯函数测试不受此约束（本就该是 unit）。
- **不动**：`daily_report_assignment_test.go`、`daily_report_matching_test.go` 经核实为零 DB 依赖的纯算法测试（CosineDistance、匈牙利分配、Phase1/2），属健康的 unit，不迁。

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `test-infrastructure`: 新增 requirement「repository 层测试禁 SQLite，必须用 testcontainer PG」——把 `standard/backend/testing.md` 已有的事故教训从「改 SQL 时才补 PG 用例」升级为「repository 包常驻禁 SQLite」。

## Impact

- **代码**：`backend-go/internal/topicgraph/repository/topic_watch_repository_test.go`（改）、`backend-go/internal/topicgraph/repository/daily_report_manual_topic_test.go`（拆分，仅留 8 个 DB 测试并迁 PG）、新增 `daily_report_manual_topic_unit_test.go`（14 个纯函数）。
- **文档**：`docs/reference/standard/backend/testing.md`（增补硬约束段）。
- **spec**：`openspec/specs/test-infrastructure/spec.md`（delta 加 requirement）。
- **依赖**：无新增。复用现有 `testutil.SetupTestDB` + 黄金 schema；model `BoardTopicWatch`/`TopicWatchHit` 已在 `daily_report_models.go` 且纳入生产 AutoMigrate。
- **无产品行为变化 / 无 API 变化 / 无 schema 变化**——纯测试基础设施与规范收敛。
