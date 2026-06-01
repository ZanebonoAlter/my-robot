## Context

`topic_tag_embeddings` 表当前有两个列存储同一份 embedding 数据：

1. `vector` (text) — 以 JSON 数组存储 2560 维 float64 全精度文本，每行 ~53 KB
2. `embedding` (vector(2560)) — pgvector 二进制格式，每行 ~10 KB

`vector` 列已被标记 `// Deprecated`，所有相似度查询已使用 `embedding` 列的 pgvector 距离运算。但写入时仍同时填充两列，且 `FindSimilarTags` 仍从 `vector` 列反序列化来构建查询向量。

当前数据量：20,159 行，`vector` 列总计 ~1052 MB（存在 TOAST 中），占数据库总大小的 39%。

约束：
- 不需要适配 SQLite 迁移（datamigrate 路径中移除 vector 相关处理即可）
- 单用户系统，无在线迁移压力，可直接 DDL + VACUUM

## Goals / Non-Goals

**Goals:**
- 彻底移除 `vector` text 列，回收 ~1052 MB 空间
- 消除 `FindSimilarTags` 对 `vector` 列的读取依赖
- Go model 和数据库 schema 保持干净，无 deprecated 遗留

**Non-Goals:**
- 不优化 embedding 维度或模型选择
- 不改变 embedding_type 的种类或数量
- 不优化 pgvector 索引策略
- 不适配 SQLite → PostgreSQL 迁移路径中的 vector 列

## Decisions

### D1: FindSimilarTags 直接用 GenerateEmbedding 返回的原始 []float64

**现状**: `FindSimilarTags` 调用 `GenerateEmbedding` → 拿到 `TopicTagEmbedding{Vector: "[...]"}` → `json.Unmarshal(Vector)` → `floatsToPgVector()` 构建 pgvector 查询字符串。

**改为**: `GenerateEmbedding` 返回新类型或修改返回值，包含原始 `[]float64`，`FindSimilarTags` 直接用 `floatsToPgVector(floats)` 构建查询。

**备选**: 让 `GenerateEmbedding` 返回时直接附带 pgvector 格式字符串。但这样会增加返回类型复杂度，不如让调用方按需转换。

**理由**: 消除一个完整的 序列化→持久化→反序列化→再序列化 循环，逻辑更清晰。

### D2: GenerateEmbedding 不再写入 Vector 字段

**现状**: 写入 `Vector: string(vectorJSON)` 和 `EmbeddingVec: pgVecStr`。

**改为**: 只写 `EmbeddingVec`，`Vector` 字段从 model 中移除。

### D3: 数据库变更用 GORM AutoMigrate + 手动 DROP COLUMN

GORM AutoMigrate 不会自动删除列。需要：
1. 从 Go model 移除 `Vector` 字段
2. 添加一条 migration：`ALTER TABLE topic_tag_embeddings DROP COLUMN IF EXISTS vector`
3. 执行 `VACUUM FULL topic_tag_embeddings` 回收空间（可选，VACUUM FULL 会锁表，单用户系统可接受）

### D4: datamigrate 中直接移除 vector 相关处理

`DefaultTableSpecs` 中 `topic_tag_embeddings` 的 `SampleColumns` 移除 `"vector"`，`AllowedMissingTargetColumns` 移除 `"vector"`。`writer_postgres.go` 和 `verify.go` 中的 vector 逻辑一并清理。

## Risks / Trade-offs

- **[VACUUM FULL 锁表]** → 单用户系统，可在维护窗口执行或用 `pg_repack` 替代。最坏情况只是暂时不可用。
- **[datamigrate 旧 SQLite 备份不兼容]** → 用户明确不需要适配 SQLite 迁移，旧备份中若有 vector 列数据将被忽略。
- **[回滚]** → 如果需要回滚，`vector` 列数据已不可恢复（DROP COLUMN）。但该列是 deprecated 冗余数据，embedding 列有完整数据，重新生成也可行。
