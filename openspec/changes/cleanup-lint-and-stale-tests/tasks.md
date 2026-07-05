# Tasks — cleanup-lint-and-stale-tests

> 实现纪律：纯清理 change，按《开发执行规范》§2 完整 TDD（坏测试修复先 RED 确认、再 GREEN）。测试只跑本次修改影响的包；归档门禁跑全量。

## 1. lint 债清理（让 `golangci-lint run ./...` 零失败）

- [x] 1.1 gofmt 自动格式化 5 文件：`cd backend-go && gofmt -w internal/reader/handler/opml.go cmd/verify-cluster-prompt/main.go internal/tagmanagement/service/core/embedding.go internal/tagmanagement/service/core/types.go internal/topicgraph/service/daily_report_merge.go`
      → 验收：`gofmt -l` 这 5 文件零命中
- [x] 1.2 删除 `admin/handler/ai_handler.go` 的 `upsertAISetting` 死函数（已核实全仓无调用方）
      → 验收：`grep -rn upsertAISetting backend-go/` 零命中
- [x] 1.3 `reader/handler/opml.go` RefreshFeed errcheck：补 `if err := feedService.RefreshFeed(...); err != nil { /* best-effort async refresh */ log.Error(...) }`
      → 验收：`golangci-lint run ./internal/reader/...` 0 issues

## 2. pre-existing 坏测试修复（让 `go test ./internal/tagmanagement/service/core` 全 PASS）

- [x] 2.1 `merge_tags_reembedding_test.go`：删除 2 处 `article_feeds` INSERT 行（表在生产代码无 model/迁移建表路径；reembedding 入队真实依赖 `ArticleTopicTag` + `TopicTagEmbedding`，article 已 Create 且 FeedID 已设）
      → 实测：删 article_feeds INSERT 后暴露真根因——`TopicTagEmbedding` 维度不匹配（embedding 列 vector(4096)，原测试 `"[0.100000]" Dim:1` 报 `expected 4096 dimensions`）。最终修法：改用 `makeValidVector(4096)` + `Dimension: 4096` + `EmbeddingType: EmbeddingTypeIdentity`（参考 embedding_test.go）。验收：core 全 PASS
- [x] 2.2 `hard_merge_test.go` `TestHardMergeTags_EmbeddingsDeleted`：`TopicTagEmbedding` 补 `EmbeddingVec: "[0.1,0.2]"`（与 Dimension: 2 一致）+ 必要 `TextHash`
      → 实测：同 2.1 根因（4096 维），补 `EmbeddingVec: makeValidVector(4096)` + `Dimension: 4096`（非 design 预期的简单 `[0.1,0.2]`）。验收：PASS

## 3. 测试

本次 change 影响的测试命令（日常只跑影响包）：

- `cd backend-go && go test ./internal/tagmanagement/service/core` — 3 个坏测试修复
- `cd backend-go && go test ./internal/reader/... ./internal/admin/...` — errcheck/unused 修改包回归
- 归档前全量：`cd backend-go && go test ./...`

## 4. 文档

- 无 flow 影响：本 change 是纯代码清理（lint 格式化 + 删死函数 + 修坏测试），不改任何业务行为 / API / model / flow，按《开发执行规范》§12.2 豁免 flow 变更溯源

## 5. 验证（归档前重跑，每条零失败）

- `cd backend-go && golangci-lint run ./...` → **0 issues（核心目标）**
- `cd backend-go && go vet ./...` → 0 错误
- `cd backend-go && go build ./...` → 编译通过
- `cd backend-go && go test ./internal/tagmanagement/service/core` → 全 PASS
- `cd backend-go && go test ./internal/reader/... ./internal/admin/... ./internal/topicgraph/...` → 修改包回归 PASS
- `cd backend-go && go test ./...` → 全量 PASS（归档门禁）
- `bash scripts/check-standards.sh` → L1 规范验收零失败（§11.4）
