## 1. Model & 写入层改造

- [x] 1.1 从 `TopicTagEmbedding` struct 移除 `Vector` 字段（`backend-go/internal/domain/models/topic_graph.go:123`）
- [x] 1.2 修改 `GenerateEmbedding`（`embedding.go:134`）：移除 `json.Marshal` 和 `Vector` 赋值，改为返回 `[]float64` 原始浮点数组供调用方使用
- [x] 1.3 修改 `GenerateEmbeddingForText`（`embedding.go:171`）：同上，移除 `Vector` 赋值
- [x] 1.4 调整 `GenerateEmbedding` / `GenerateEmbeddingForText` 的返回类型或签名，使其携带原始 `[]float64`（供 `FindSimilarTags` 使用）

## 2. 查询层改造

- [x] 2.1 修改 `FindSimilarTags`（`embedding.go:222`）：不再从 `embedding.Vector` 反序列化，改为直接使用 `GenerateEmbedding` 返回的 `[]float64` 调用 `floatsToPgVector`
- [x] 2.2 验证 `FindSimilarTags` 的 SQL 查询仍正确使用 `embedding` pgvector 列

## 3. 数据库 Migration

- [x] 3.1 在 `postgres_migrations.go` 中添加 migration：`ALTER TABLE topic_tag_embeddings DROP COLUMN IF EXISTS vector`
- [x] 3.2 启动后端验证 migration 执行成功，`topic_tag_embeddings` 表不再有 `vector` 列

## 4. Datamigrate 清理

- [x] 4.1 `types.go:86`：从 `DefaultTableSpecs` 中 `topic_tag_embeddings` 的 `SampleColumns` 移除 `"vector"`，从 `AllowedMissingTargetColumns` 移除 `"vector"`
- [x] 4.2 `writer_postgres.go`：移除 `includeEmbeddingVector` 相关的 vector 列特殊处理逻辑（约 196 行附近）
- [x] 4.3 `verify.go:316`：移除 `check.SourceVector` 赋值中 vector 列的处理

## 5. 验证

- [x] 5.1 编译通过：`cd backend-go && go build ./...`
- [x] 5.2 测试通过：`cd backend-go && go test ./internal/domain/tagging/... ./internal/platform/database/datamigrate/...`（无新增失败）
- [x] 5.3 数据库空间回收：`VACUUM FULL topic_tag_embeddings`，表大小从 1569 MB 降至 245 MB，数据库总大小从 2659 MB 降至 1337 MB
