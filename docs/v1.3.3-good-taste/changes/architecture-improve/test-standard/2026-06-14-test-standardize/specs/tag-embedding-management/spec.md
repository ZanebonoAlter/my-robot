# Tag Embedding Management (Delta)

## Purpose

Delta spec for `tag-embedding-management`: migrate all tagmanagement embedding-related tests from SQLite to Postgres, ensuring embedding CRUD, similarity search, and SaveEmbedding cleanup logic are tested against real pgvector behavior.

## Requirements

### Requirement: 所有 tagmanagement 测试迁移到 Postgres

`internal/tagmanagement/` 下全部 14 个测试文件 SHALL 使用 `testutil.SetupTestDB(t)` 替代各自的 SQLite setup 函数。其中 13 个直接 import `glebarez/sqlite`，另 1 个（`semantic_board_backfill_test.go`）间接复用即将删除的 `setupSemanticBoardMatchingTestDB`。迁移文件列表：

- `repository/tagger_embedding_test.go`
- `repository/tag_job_queue_test.go`
- `service/core/embedding_test.go`
- `service/core/quality_score_test.go`
- `service/core/metadata_test.go`
- `service/core/hard_merge_test.go`
- `service/core/merge_tags_reembedding_test.go`
- `service/auxlabel/auxiliary_label_service_test.go`
- `service/board/semantic_board_matching_test.go`
- `service/board/semantic_board_upgrade_test.go`
- `service/board/semantic_board_backfill_test.go`
- `service/board/tag_clustering_test.go`
- `handler/semantic_board_handler_test.go`
- `handler/merge_reembedding_queue_test.go`

#### Scenario: 迁移后的测试文件不引用 glebarez/sqlite

- **WHEN** 检查上述文件的 import 声明
- **THEN** 不存在 `"github.com/glebarez/sqlite"` 导入

#### Scenario: 迁移后的测试在 Postgres 下全部通过

- **WHEN** Docker Postgres 运行且执行 `go test ./internal/tagmanagement/... -v`
- **THEN** 上述所有文件中的测试通过

### Requirement: embedding 测试使用真实 Postgres + pgvector

`embedding_test.go` 和 `tagger_embedding_test.go` 中的测试 SHALL 使用 `testutil.SetupTestDB(t)`，确保 embedding 写入和相似度查询走 pgvector 代码路径。

#### Scenario: embedding 写入测试验证 vector 列

- **WHEN** 测试通过 `testutil.SetupTestDB` 获取 Postgres 连接并创建一条 `TopicTagEmbedding` 记录
- **THEN** `embedding` (vector) 列被正确写入，查询 `SELECT embedding FROM topic_tag_embeddings` 返回非空结果

#### Scenario: 相似度查询使用 pgvector 距离操作符

- **WHEN** 测试调用 `FindSimilarTags` 或类似相似度搜索函数
- **THEN** 底层 SQL 使用 pgvector `<=>` 操作符，而非 Go 侧遍历比较

### Requirement: auxiliary_label_service 测试使用 Postgres

`auxiliary_label_service_test.go` SHALL 使用 `testutil.SetupTestDB(t)`，确保辅助标签匹配逻辑在 Postgres 环境下验证。

#### Scenario: 辅助标签匹配测试在 Postgres 下通过

- **WHEN** 运行 `TestAuxiliaryLabel*` 系列测试
- **THEN** 所有测试通过，包括 embedding 相似度匹配、别名合并、新建标签等场景

#### Scenario: merge embedding 比较使用与生产一致的路径

- **WHEN** `sqlMergeMatcher` 在测试中被调用
- **THEN** 相似度计算行为与生产一致（pgvector SQL 或保留的 Go 侧优化路径）
