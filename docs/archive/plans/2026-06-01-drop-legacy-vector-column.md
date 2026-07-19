# Drop Legacy Vector Column 实施计划

> **REQUIRED SUB-SKILL:** 使用 subagent-driven-development 执行此计划。

**Goal:** 从 `topic_tag_embeddings` 彻底移除废弃的 `vector` text 列，回收 ~1052 MB 空间。

**Architecture:** 修改 Go model 移除 Vector 字段 → 改写写入/查询代码消除对 vector 的依赖 → 添加 DB migration DROP COLUMN → 清理 datamigrate 代码。

**Tech Stack:** Go (GORM), PostgreSQL (pgvector), SQL migration

---

## Group 1: Model & 写入层改造

### Task 1: 移除 TopicTagEmbedding.Vector 字段 + 改写写入函数 + 修改 FindSimilarTags

**Files:**
- Modify: `backend-go/internal/domain/models/topic_graph.go:123` (移除 Vector 字段)
- Modify: `backend-go/internal/domain/tagging/embedding.go` (3 处改动)

**Step 1: 移除 Vector 字段**

在 `backend-go/internal/domain/models/topic_graph.go` 第 123 行，删除：
```go
Vector        string    `gorm:"type:text;not null" json:"vector"` // Deprecated: legacy JSON text payload. Use EmbeddingVec for pgvector.
```

**Step 2: 改写 GenerateEmbedding (embedding.go 约第 127-150 行)**

移除 `json.Marshal` 和 `Vector` 赋值。函数签名不变，返回的 TopicTagEmbedding 不再包含 Vector 字段（已删除）。

```go
// 删除这两行:
// vectorJSON, err := json.Marshal(result.Embeddings[0])
// ... err check ...

// embedding 赋值中删除:
// Vector:        string(vectorJSON),
```

需要保留 `result.Embeddings[0]` 用于 `floatsToPgVector`，所以核心逻辑变为：
```go
pgVecStr := floatsToPgVector(result.Embeddings[0])
embedding := &models.TopicTagEmbedding{
    TopicTagID:    tag.ID,
    EmbeddingType: embeddingType,
    EmbeddingVec:  pgVecStr,
    Dimension:     result.Dimensions,
    Model:         result.Model,
    TextHash:      textHash,
}
```

同时移除文件顶部不再需要的 `"encoding/json"` import（如果只有此处用到——但注意 `json.Unmarshal` 在 FindSimilarTags 也用了，那个也要删，见 Step 4）。`json.Marshal` 在 `embedding.go` 中只用于 vector，改完后可检查是否还有其他 json 用法。`buildTagEmbeddingText` 中有 `json.Unmarshal` 用于 aliases，所以 `"encoding/json"` import 要保留。

**Step 3: 改写 GenerateEmbeddingForText (embedding.go 约第 155-185 行)**

同 Step 2 模式：
```go
// 删除 json.Marshal 和 Vector 赋值
pgVecStr := floatsToPgVector(result.Embeddings[0])
embedding := &models.TopicTagEmbedding{
    TopicTagID:    tagID,
    EmbeddingType: embeddingType,
    EmbeddingVec:  pgVecStr,
    Dimension:     result.Dimensions,
    Model:         result.Model,
    TextHash:      textHash,
}
```

**Step 4: 改写 FindSimilarTags (embedding.go 约第 220-230 行)**

当前代码：
```go
var vector []float64
if err := json.Unmarshal([]byte(embedding.Vector), &vector); err != nil {
    return nil, fmt.Errorf("failed to parse embedding vector: %w", err)
}
pgVecStr := floatsToPgVector(vector)
```

改为：让 `GenerateEmbedding` 直接返回原始浮点数组。最简洁的方式是修改 `GenerateEmbedding` 返回值类型，返回一个包含 `*models.TopicTagEmbedding` 和 `[]float64` 的结构。

或者更简单：在 `FindSimilarTags` 中直接调用 `floatsToPgVector`，不需要从 Vector 字段反序列化。问题是 `GenerateEmbedding` 内部已有 `result.Embeddings[0]`，但没有暴露出来。

**推荐方案：** 修改 `GenerateEmbedding` 的返回，增加 `[]float64` 返回值：

```go
func (s *EmbeddingService) GenerateEmbedding(ctx context.Context, tag *models.TopicTag, embeddingType string, opts ...EmbeddingTextOptions) (*models.TopicTagEmbedding, []float64, error) {
```

调用方（FindSimilarTags）：
```go
embedding, rawFloats, err := s.GenerateEmbedding(ctx, tag, embeddingType)
if err != nil {
    return nil, fmt.Errorf("failed to generate embedding: %w", err)
}
pgVecStr := floatsToPgVector(rawFloats)
```

其他调用 `GenerateEmbedding` 的地方需要适配（忽略第三个返回值或用 `_`）。需要检查所有调用点：
- `FindSimilarTags` — 用 rawFloats
- `TagMatch` — 通过 `FindSimilarTags` 间接调用，不直接调用 `GenerateEmbedding`
- 其他可能的调用点 — 用 `_` 忽略

同样 `GenerateEmbeddingForText` 也要增加 `[]float64` 返回值。

**Step 5: 编译验证**

Run: `cd backend-go && go build ./...`
Expected: 编译成功，无错误

**Step 6: Commit**

```bash
git add backend-go/internal/domain/models/topic_graph.go backend-go/internal/domain/tagging/embedding.go
git commit -m "refactor: remove deprecated Vector text field from TopicTagEmbedding"
```

---

## Group 2: 数据库 Migration

### Task 2: 添加 DROP COLUMN migration

**Files:**
- Modify: `backend-go/internal/platform/database/postgres_migrations.go` (在最后一条 migration 之后添加新 migration)

**Step 1: 添加 migration**

在 `postgresMigrations()` 的 return 数组最后一条之后（即最后一个 `},` 之后、`}` 之前）添加：

```go
{
    Version:     "20260601_0001",
    Description: "Drop deprecated vector text column from topic_tag_embeddings.",
    Up: func(db *gorm.DB) error {
        return db.Exec("ALTER TABLE topic_tag_embeddings DROP COLUMN IF EXISTS vector").Error
    },
},
```

**Step 2: 启动验证**

启动后端 `go run cmd/server/main.go`，检查日志确认 migration 执行成功。

**Step 3: Commit**

```bash
git add backend-go/internal/platform/database/postgres_migrations.go
git commit -m "feat: add migration to drop deprecated vector column"
```

---

## Group 3: Datamigrate 清理

### Task 3: 清理 datamigrate 中 vector 相关代码

**Files:**
- Modify: `backend-go/internal/platform/database/datamigrate/types.go:87`
- Modify: `backend-go/internal/platform/database/datamigrate/writer_postgres.go` (~行 150, 196, 340-355)
- Modify: `backend-go/internal/platform/database/datamigrate/verify.go` (~行 316)

**Step 1: types.go 第 87 行**

```go
// 当前:
{Name: "topic_tag_embeddings", PrimaryKey: "id", SampleColumns: []string{"topic_tag_id", "dimension", "model", "text_hash", "vector"}, AllowedMissingTargetColumns: []string{"vector"}},

// 改为:
{Name: "topic_tag_embeddings", PrimaryKey: "id", SampleColumns: []string{"topic_tag_id", "dimension", "model", "text_hash"}},
```

**Step 2: writer_postgres.go 清理 includeEmbeddingVector 逻辑**

删除以下代码（约第 196 行）：
```go
includeEmbeddingVector := spec.Name == "topic_tag_embeddings" && targetSet["embedding"] && contains(sourceColumns, "vector")
```

修改 `resolveColumns` 函数签名，不再返回 `bool`：
```go
// 当前: return shared, includeEmbeddingVector, nil
// 改为: return shared, nil
```

同时修改所有调用 `resolveColumns` 的地方，适配新签名。

删除 `WriteRows` 中使用 `includeEmbeddingVector` 的代码块（约第 155-160 行）：
```go
if includeEmbeddingVector {
    args = append(args, row["vector"])
}
```

删除 `LoadEmbeddingChecks` 中 `legacyVectorExpr` 相关逻辑（约第 340-355 行）：
```go
// 删除:
legacyVectorExpr := "''"
if targetSet["vector"] {
    legacyVectorExpr = "COALESCE(vector, '')"
}
```
以及 query 中对 `legacyVectorExpr` 的引用，只保留 `embeddingExpr`。

**Step 3: verify.go 第 316 行**

```go
// 当前:
check.SourceVector = fmt.Sprintf("%v", sourceRow["vector"])

// 删除这一行，并检查 EmbeddingVectorCheck 结构体是否需要调整
```

查看 `EmbeddingVectorCheck` 结构体定义，如果有 `SourceVector` 字段且不再需要，也一并移除。

**Step 4: 编译验证**

Run: `cd backend-go && go build ./...`
Expected: 编译成功

**Step 5: 运行 datamigrate 相关测试**

Run: `cd backend-go && go test ./internal/platform/database/datamigrate/...`
Expected: 全部通过

**Step 6: Commit**

```bash
git add backend-go/internal/platform/database/datamigrate/
git commit -m "refactor: remove vector column handling from datamigrate"
```

---

## Group 4: 最终验证

### Task 4: 编译 + 测试 + 数据库空间确认

**Step 1: 全量编译**

Run: `cd backend-go && go build ./...`
Expected: PASS

**Step 2: 受影响的包测试**

Run: `cd backend-go && go test ./internal/domain/tagging/... ./internal/platform/database/datamigrate/...`
Expected: 全部通过

**Step 3: lint**

Run: `cd backend-go && golangci-lint run ./...`
Expected: 无新增 warning

**Step 4: 启动后端，验证 migration 执行**

Run: `cd backend-go && go run cmd/server/main.go`

验证数据库：
```sql
\d topic_tag_embeddings
-- 应不再有 vector 列
```

**Step 5: VACUUM 回收空间**

```sql
VACUUM FULL topic_tag_embeddings;
SELECT pg_size_pretty(pg_total_relation_size('topic_tag_embeddings')) as total_size;
-- 预期: ~500 MB (从 ~1569 MB 缩减)
```
