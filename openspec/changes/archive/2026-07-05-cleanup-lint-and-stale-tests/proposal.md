## Why

仓库存在既存 lint 债（7 个 `golangci-lint` issue）与 3 个 pre-existing 坏测试，导致全量 `golangci-lint run ./...` 与 `go test ./...` 无法零失败。近期三个 change（`ai-call-logging-schema` / `speed-up-testcontainer-setup` / `manual-topic-lane`）归档时按《开发执行规范》「测试只跑本次修改影响的包」+ AGENTS.md「ignore unrelated dirty-worktree changes」以**定向门禁**绕过，但债仍在，持续阻塞后续 change 的全量归档门禁（§11.1）。需专项清理让仓库回到全量门禁零失败的干净基线。

## What Changes

- **修 7 个既存 lint issue**（让 `golangci-lint run ./...` 零失败）：
  - gofmt ×5：`reader/handler/opml.go`、`cmd/verify-cluster-prompt/main.go`、`tagmanagement/service/core/embedding.go`、`tagmanagement/service/core/types.go`、`topicgraph/service/daily_report_merge.go`（`gofmt -w` 自动格式化，零逻辑变更）
  - unused ×1：`admin/handler/ai_handler.go` 的 `upsertAISetting`（死函数，确认无调用方后删除）
  - errcheck ×1：`reader/handler/opml.go` 的 `RefreshFeed` 返回值未检查（补错误处理或显式忽略并注释理由）
- **修 3 个 pre-existing 坏测试**（`tagmanagement/service/core`）：
  - `TestMergeTagsEnqueuesReembeddingAfterSuccess` / `TestMergeTagsReturnsErrorWhenReembeddingEnqueueFails`：INSERT 不存在的 `article_feeds` 表（整个代码库无 model/迁移建表路径）→ 改测试不依赖该表
  - `TestHardMergeTags_EmbeddingsDeleted`：`TopicTagEmbedding` 未设 `Vector` 字段导致插空串被 PG 拒绝 → 测试 setup 补合法 Vector 值
- **不改任何业务行为**：纯清理，无 API / model / flow 变更，无数据迁移，无 flow 影响。

## Capabilities

### New Capabilities
（无）

### Modified Capabilities
- `test-infrastructure`：补「全量门禁零失败」基线约束——仓库须维持 `golangci-lint run ./...` + `go test ./...` 全量零失败，既存 lint 债与 pre-existing 坏测试须即时清理，不得积压阻塞归档门禁。

### Removed Capabilities
（无）
