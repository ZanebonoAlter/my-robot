## Why

`topic_tag_embeddings` 表占 1569 MB（数据库 2659 MB 的 59%），其中废弃的 `vector` text 列独占约 1052 MB。该列以全精度 JSON 文本（17 位小数）存储 embedding 向量，与实际在用的 `embedding` pgvector 列完全冗余。代码中已标注 `// Deprecated`，查询已迁移到 pgvector，但旧列从未清理。

## What Changes

- **删除 `vector` text 列**：从 Go model、数据库 schema、写入逻辑中彻底移除
- **修复 `FindSimilarTags`**：当前从 DB 读取 `vector` 文本再反序列化为 `[]float64` 来构建查询向量，改为直接使用 `GenerateEmbedding` 返回的原始浮点数组，消除对 `vector` 列的读取依赖
- **清理 datamigrate**：移除 `topic_tag_embeddings` 迁移配置中的 `vector` 相关字段
- **数据库 migration**：`ALTER TABLE topic_tag_embeddings DROP COLUMN vector`，回收约 1052 MB 空间

## Capabilities

### New Capabilities

（无）

### Modified Capabilities

- `tagging-domain`：embedding 模型移除 `Vector` 字段，`FindSimilarTags` 改为直接使用原始浮点数组构建 pgvector 查询字符串

## Impact

- **数据库**：`topic_tag_embeddings` 表从 ~1569 MB 缩减至 ~500 MB，数据库总大小从 ~2659 MB 缩减至 ~1600 MB
- **Go 代码**：`backend-go/internal/domain/tagging/embedding.go`（读写两处）、`backend-go/internal/domain/models/topic_graph.go`（model 定义）、`backend-go/internal/platform/database/datamigrate/`（迁移配置 3 处）
- **无 API 变化**：所有变更均为内部实现，不涉及外部 API
- **无前端影响**：`Vector` 字段已通过 `json:"-"` 或仅在内部使用
