## Why

设置页 "Embedding" 栏（高/低相似度阈值、embedding 维度、板块匹配阈值滑块）写入数据库后**没有任何运行时消费点**——这些值既不影响 embedding 生成（走 airouter capability-routes），也不影响标签匹配（走 `semantic_board_match_*` 系列，入口在标签管理-匹配规则）。更糟的是 `EmbeddingConfigService.UpdateConfig` 对 `embedding_model` 会打一条 warning，让用户误以为改了它会切换模型导致旧 embedding 失效，实际上什么都不会发生。这是一次没清理干净的重构遗留：UI、表、CRUD、migration seed 都还在，但运行时已绕过它们。

## What Changes

- **移除前端 "Embedding" 设置栏**：`SettingsSectionEmbedding.vue`、`EmbeddingConfigPanel.vue`、`embeddingConfig.ts`（含 `useEmbeddingConfigApi`）、`settings.vue` 的 `'embedding'` section、`GlobalSettingsDialog.vue` 的 embedding tab、`useAIRouterSettings` 中的 `embeddingThreshold`/`saveThreshold`/`savingThreshold` 及其加载/保存逻辑。
- **移除后端死接口**：`embedding_config_handler.go`、`EmbeddingConfigService.LoadMatchThreshold`（写了从未被调用）、`ai_handler.go` 中 `NarrativeBoardEmbeddingThreshold` 字段及其校验/upsert、对应路由注册。
- **清理死数据行**：`embedding_config` 表中 `high_similarity_threshold` / `low_similarity_threshold` / `embedding_dimension` / `embedding_model` 四行；`ai_settings` 表中 `narrative_board_embedding_threshold` 一行。
- **保留 `embedding_config` 表与 `cluster_*` 系列**：`cluster_max_tags` 等 5 个 key 被 `tag_clustering.go:193` 的 `LoadClusterConfig()` 真实读取，仅是无 UI 入口、依赖 seed 默认值，不在本次清理范围。
- **BREAKING**（仅对内部 API）：`GET/PUT /embedding/config`、`ai_settings.narrative_board_embedding_threshold` 写入字段移除。这些接口本就不影响任何运行时行为，无外部消费者。

## Capabilities

### New Capabilities
<!-- 无新增能力，纯清理 -->

### Modified Capabilities
- `tag-to-board-matching`: 新增负向 requirement，明确系统 **SHALL NOT** 提供独立的 embedding 阈值/维度/模型配置入口；tag-board 匹配参数的唯一用户可调入口为 `semantic_board_match_*` 系列（即标签管理-匹配规则对话框）。固化"配置入口收敛"的契约，防止死配置复活。

## Impact

- **前端**：`front/app/features/settings/components/`、`front/app/features/ai/components/`、`front/app/features/ai/composables/useAIRouterSettings.ts`、`front/app/pages/settings.vue`、`front/app/components/dialog/GlobalSettingsDialog.vue`、`front/app/api/embeddingConfig.ts`、`front/app/api/index.ts`（导出清理）。
- **后端**：`backend-go/internal/tagmanagement/handler/embedding_config_handler.go`（删）、`backend-go/internal/tagmanagement/service/core/config_service.go`（删 `LoadMatchThreshold`，保留 `LoadClusterConfig`）、`backend-go/internal/tagmanagement/routes.go`、`backend-go/internal/admin/handler/ai_handler.go`、`backend-go/internal/admin/wire.go`。
- **数据**：一次性的数据清理 migration，删除上述死 key 行；表结构不变。
- **零运行时行为变化**：所有真正生效的匹配逻辑（`semantic_board_matching.go`）、聚类逻辑（`tag_clustering.go`）、embedding 生成（`embedding.go` → `airouter`）均不受影响。
