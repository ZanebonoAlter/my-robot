# Tag Embedding Management (Delta)

## Purpose

Delta spec for `tag-embedding-management`: migrate embedding-related tests from SQLite to Postgres, ensuring embedding CRUD, similarity search, and SaveEmbedding cleanup logic are tested against real pgvector behavior.

## Requirements

### Requirement: embedding 测试使用真实 Postgres + pgvector

`embedding_test.go` 中的测试 SHALL 使用 `testutil.SetupTestDB(t)` 替代 SQLite 内存数据库，确保 embedding 写入和相似度查询走 pgvector 代码路径。

#### Scenario: embedding 写入测试验证 vector 列

- **WHEN** 测试通过 `testutil.SetupTestDB` 获取 Postgres 连接并创建一条 `TopicTagEmbedding` 记录
- **THEN** `embedding` (vector) 列被正确写入，查询 `SELECT embedding FROM topic_tag_embeddings` 返回非空结果

#### Scenario: 相似度查询使用 pgvector 距离操作符

- **WHEN** 测试调用 `FindSimilarTags` 或类似相似度搜索函数
- **THEN** 底层 SQL 使用 pgvector `<=>` 操作符（可通过 SQL 日志或 `db.Debug()` 验证），而非 Go 侧遍历比较

### Requirement: auxiliary_label_service 测试使用 Postgres

`auxiliary_label_service_test.go` SHALL 使用 `testutil.SetupTestDB(t)` 替代 SQLite，确保辅助标签匹配逻辑（包括 merge embedding 比较和别名添加）在 Postgres 环境下验证。

#### Scenario: 辅助标签匹配测试在 Postgres 下通过

- **WHEN** 运行 `TestAuxiliaryLabel*` 系列测试
- **THEN** 所有测试通过，包括 embedding 相似度匹配、别名合并、新建标签等场景，且使用 Postgres 数据库

#### Scenario: merge embedding 比较使用 pgvector

- **WHEN** `sqlMergeMatcher` 在测试中被调用（通过 `AuxiliaryLabelService` 间接调用）
- **THEN** 相似度计算使用 pgvector SQL 或 Go 侧优化路径（非 SQLite fallback），行为与生产一致

### Requirement: 优先迁移的 5 个测试文件全部使用 testutil

以下 5 个测试文件 SHALL 替换 `setupXxxTestDB()` 中的 SQLite 初始化为 `testutil.SetupTestDB(t)` 调用：

- `semantic_board_matching_test.go`
- `embedding_test.go`
- `auxiliary_label_service_test.go`
- `semantic_board_upgrade_test.go`
- `semantic_board_handler_test.go`

#### Scenario: 迁移后的测试文件不引用 glebarez/sqlite

- **WHEN** 检查上述 5 个测试文件的 import 声明
- **THEN** 不存在 `"github.com/glebarez/sqlite"` 导入

#### Scenario: 迁移后的测试在 Postgres 下全部通过

- **WHEN** Docker Postgres 运行且执行 `go test ./internal/domain/tagging/...`（不含 `-short`）
- **THEN** 上述 5 个文件中的所有测试通过
