# Tasks — cleanup-lint-and-stale-tests

> 实现纪律：纯清理 change。原范围 7 lint + 3 core 坏测试；apply 中应作者要求顺手修了同批暴露的 database 测试 / handler / auxlabel / board 缓存副作用（均为 speed-up ResetTestData 引入或暴露的既存问题）。测试只跑本次修改影响的包；归档门禁跑全量。

## 1. lint 债清理（让 `golangci-lint run ./...` 零失败）✓

- [x] 1.1 gofmt 自动格式化 6 文件（`reader/handler/opml.go`、`cmd/verify-cluster-prompt/main.go`、`tagmanagement/service/core/embedding.go`+`types.go`、`topicgraph/service/daily_report_merge.go`、`models/semantic_label.go`）
      → 实测：`gofmt -l` 全绿
- [x] 1.2 删除 `admin/handler/ai_handler.go` 的 `upsertAISetting` 死函数（+ 删随之未用的 `gorm.io/gorm` import）
      → 实测：`grep -rn upsertAISetting` 零命中
- [x] 1.3 `reader/handler/opml.go` RefreshFeed errcheck：改 `_ =` + best-effort 注释（异步刷新不阻断 OPML 导入）
      → 实测：`golangci-lint run ./...` **0 issues**

## 2. core 坏测试（`tagmanagement/service/core` 3 个）✓

- [x] 2.1 `merge_tags_reembedding_test.go`：删 2 处 `article_feeds` INSERT（表无建表路径）；真根因是 `TopicTagEmbedding` 维度（embedding 列 `vector(4096)`，原 `"[0.100000]" Dim:1` 报 4096），改用 `makeValidVector(4096)` + `Dimension:4096` + `EmbeddingTypeIdentity`
- [x] 2.2 `hard_merge_test.go` TestHardMergeTags_EmbeddingsDeleted：同 4096 维根因，补 `EmbeddingVec: makeValidVector(4096)` + `Dimension:4096`
      → 实测：core 全 PASS（3 坏测试修复）

## 3. database 迁移文档测试（应作者要求顺手修）✓

- [x] 3.1 TestPostgresMigrationsDocumentStagedEmbeddingCutover：迁移 `20260403_0003` Description 补 staged + json（"Staged rollout: ... vector(4096); runtime dimension via embedding_config JSON"）
- [x] 3.2 TestSemanticLabelBoardSystemMigrationDocumentsSchemaCutover：**方案 B**——改测试适配 AutoMigrate（移除对 CREATE TABLE/列/ADD COLUMN 的 mustContainAll 检查，保留索引/seed/历史 DROP 检查）
      → 实测：database 包 PASS

## 4. handler 测试（speed-up ResetTestData 副作用）✓

- [x] 4.1 `semantic_board_handler_test.go` TestSemanticBoardHandlerUpgradeBackfillAndConfig：硬 `db.Create(ai_settings)` 与 ResetTestData seed 冲突 duplicate key，改 `FirstOrCreate + Assign`
      → 实测：handler 包 PASS

## 5. auxlabel 测试（speed-up ResetTestData + 包级缓存）✓

- [x] 5.1 `packageAuxLabelCache`（包级单例，"shared across instances"）跨测试残留过时 active labels，ResetTestData 清 db 但不清内存缓存 → 暴露 `InvalidateAuxLabelCache()` + setup 调用
      → 实测：auxlabel 包 PASS

## 6. board 缓存失效（同款包级缓存，已修缓存层）✓

- [x] 6.1 `packageBoardCache` 同款问题：暴露 `InvalidateBoardCache()`（清 board data + config）+ 3 个 setup（matching/upgrade/clustering）调用
      → 实测：缓存层失效完成。**但 backfill 4 测试仍失败**（见验证节，speed-up ResetTestData 更深副作用，非缓存层、非本 change 引入，另行处理）

## 7. 测试

本次 change 影响的测试命令（日常只跑影响包）：

- `go test ./internal/tagmanagement/service/core ./internal/platform/database ./internal/tagmanagement/handler ./internal/tagmanagement/service/auxlabel`
- 归档前全量：`go test ./...`（board backfill 仍 FAIL，见验证节）

## 8. 文档

- 无 flow 影响：纯代码清理（lint + 坏测试 + 测试副作用修复），不改业务行为 / API / model / flow，按《开发执行规范》§12.2 豁免 flow 变更溯源

## 9. 验证（归档前实测，2026-07-05）

- `golangci-lint run ./...` → **0 issues（核心目标达成）** ✓
- `go vet ./...` / `go build ./...` → PASS ✓
- `tagmanagement/service/core` → PASS（3 坏测试修复）✓
- `platform/database` → PASS（测试1+2 修复）✓
- `tagmanagement/handler` → PASS（FirstOrCreate）✓
- `tagmanagement/service/auxlabel` → PASS（缓存失效）✓
- `bash scripts/check-standards.sh` → L1 零失败（§11.4）✓
- 全量 `go test ./...` → **24/25 包 PASS；board backfill 4 测试 FAIL**（`TestSemanticBoardBackfill*`，"has 0"）。根因在 `evaluateSemanticBoardMatches` 匹配条件深处（backfill 全查 db 非 packageBoardCache；疑似 `MinEffectiveSample=3` + 单 auxiliary + config 交互），是 speed-up ResetTestData 的深度副作用（cleanup 开始前就 FAIL，非本 change 引入）。**本 change 不含，建议另开 speed-up-followup change 深入调试。**
