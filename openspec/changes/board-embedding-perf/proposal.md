## Why

版块 embedding 匹配存在严重性能问题和功能缺失：(1) `MatchTopicTag` 每次调用都全量加载所有版块辅助标签和 embedding 到 Go 内存做 O(N×T×B×d) 暴力余弦比较，无缓存、无索引；(2) `semantic_labels.embedding` 的 HNSW 索引被删除后未重建（维度 2560 超过 HNSW 2000 维限制）；(3) 版块升级只支持"发现新版块"模式，缺少"扩充已有版块"方向；(4) backfill 逐条顺序处理无法并行。

## What Changes

- 新增内存缓存层：
  - 版块辅助标签缓存（`board_auxiliaries`）——只在版块 upgrade 后失效
  - 版块 embedding 缓存（`board_embeddings`）——同上
  - AI 配置缓存（`ai_settings` 的 13 行）——TTL 5 分钟
  - 活跃辅助标签缓存（`loadActiveAuxiliaryLabels`）——TTL 5 分钟
- 优化 `MatchTopicTag` 匹配流程：
  - 先通过元数据（ref_count、时间范围）缩小候选版块集
  - 缓存版块 embedding 避免重复加载和 parsePgVector
- 版块升级拆分为两个手动选择的模式：
  - **Mode 1 — 发现新版块**：当前逻辑，查找未分配的辅助标签聚类
  - **Mode 2 — 扩充已有版块**：查找可归入现有版块的辅助标签，生成 `merge_into_existing` 建议
  - 前端增加模式选择 UI
- backfill 批处理优化：
  - 将标签按 batch 分组，每批并行调用 `MatchTopicTag`
  - 可配置并发数（默认 4）
- 次要优化：`clusterAuxiliaryLabels` 的 N² SQL 改为分批 + 缓存

## Capabilities

### New Capabilities

- `board-match-cache`: 版块匹配的内存缓存层，缓存版块辅助标签、embedding 和 AI 配置，支持基于版块变更的缓存失效
- `board-upgrade-expand`: 扩充已有版块的升级模式，查找可归入现有版块的辅助标签并生成 merge_into_existing 建议

### Modified Capabilities

- `tag-to-board-matching`: 匹配流程增加缓存和元数据预过滤，backfill 改为批处理并行
- `board-upgrade`: 拆分为两个方向（发现新版块 / 扩充已有版块），前端增加模式选择
- `tag-embedding-management`: 优化 `parsePgVector` 调用，减少重复解析

## Impact

- `backend-go/internal/domain/tagging/semantic_board_matching.go`：新增缓存 + 优化匹配流程
- `backend-go/internal/domain/tagging/semantic_board_upgrade.go`：拆分双模式，修改 collectCandidates/prompt/filter
- `backend-go/internal/domain/tagging/semantic_board_handler.go`：升级 API 增加 mode 参数
- `backend-go/internal/domain/tagging/semantic_board_backfill.go`：批处理并行
- `backend-go/internal/domain/tagging/auxiliary_label_service.go`：辅助标签缓存
- `front/app/features/tags/components/UpgradeSuggestionPanel.vue`：增加模式选择 UI
- `front/app/api/semanticBoards.ts`：API 适配 mode 参数
- 不影响数据库 schema
