## 决策

### D1. gofmt ×5 — `gofmt -w` 自动，零逻辑变更
5 个文件（`reader/handler/opml.go`、`cmd/verify-cluster-prompt/main.go`、`tagmanagement/service/core/embedding.go`、`tagmanagement/service/core/types.go`、`topicgraph/service/daily_report_merge.go`）仅格式问题，`gofmt -w` 自动修复，不碰逻辑。

### D2. upsertAISetting unused — 删除死函数
核实（`grep -rn upsertAISetting` 全仓）：仅定义处 `admin/handler/ai_handler.go:371`，**无任何调用方**（含测试、wire、反射）。死函数，直接删除（含其 docstring）。

### D3. RefreshFeed errcheck — best-effort 异步，补日志
上下文：`reader/handler/opml.go:155` 在 goroutine 里循环批量 refresh feed（OPML 导入后），`feedService.RefreshFeed(...)` 返回值未检查。这是 fire-and-forget 异步刷新（不阻断 OPML 导入主流程响应）。修法：补 `if err := ...; err != nil { log.Error("best-effort async refresh failed", ...) }`，保留 best-effort 语义同时满足 errcheck。

### D4. article_feeds 坏测试 ×2 — 删除多余的 INSERT
核实：`article_feeds` 表在整个代码库**无 model / 无迁移建表路径**（生产代码不存在）。`merge_tags_reembedding_test.go` 的 2 个测试 INSERT 该表是测试作者误解——reembedding 入队的真实依赖是 `ArticleTopicTag`（article↔tag 关联）+ `TopicTagEmbedding`（tag 向量），article 本身已 `db.Create` 且 `FeedID` 已设。修法：删除 2 处 `article_feeds` INSERT 行。**不新增 ArticleFeed model/迁移**（那是另一个功能，超出清理范围）。apply 时验证：删除后 `MergeTags` 入队断言仍成立；若入队逻辑实际依赖 article_feeds join（预期不依赖），则深入看 MergeTags。

### D5. TestHardMergeTags_EmbeddingsDeleted vector 空串 — 补合法 EmbeddingVec
核实：`hard_merge_test.go` 创建 `TopicTagEmbedding{TopicTagID, Dimension: 2, Model: "test"}` 未设 `EmbeddingVec`，PG vector 类型拒绝空串（`invalid input syntax for type vector: ""`）。参考同文件 reembedding 测试格式 `EmbeddingVec: "[0.100000]"`。修法：补 `EmbeddingVec: "[0.1,0.2]"`（与 `Dimension: 2` 一致）+ 必要的 `TextHash`。

## 验证
- `golangci-lint run ./...` → 0 issues（核心目标）
- `go test ./internal/tagmanagement/service/core` → 全 PASS（3 个坏测试修复）
- `go test ./internal/reader/... ./internal/admin/... ./internal/topicgraph/...` → 修改包回归 PASS
- `go test ./...` → 全量 PASS
