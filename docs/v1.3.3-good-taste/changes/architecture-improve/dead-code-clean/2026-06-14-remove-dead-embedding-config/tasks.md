## 1. 后端数据清理 migration

- [x] 1.1 在 `backend-go/internal/platform/database/postgres_migrations.go` 新增 migration：`DELETE FROM embedding_config WHERE key IN ('high_similarity_threshold','low_similarity_threshold','embedding_dimension','embedding_model')`，幂等（DELETE 天然幂等，无需 IF EXISTS）
- [x] 1.2 同一 migration 中追加：`DELETE FROM ai_settings WHERE key = 'narrative_board_embedding_threshold'`
- [x] 1.3 验证 `cluster_*` / `event_cluster_*` 系列 key **不在**删除白名单中（代码 review，确认表与这 5 个 key 保留）

## 2. 后端死接口移除

- [x] 2.1 删除 `backend-go/internal/tagmanagement/handler/embedding_config_handler.go` 整个文件
- [x] 2.2 在 `backend-go/internal/tagmanagement/routes.go` 移除 `embGroup.GET("/config", ...)` 等路由注册（连带 embGroup 若无其他路由则一并清理）
- [x] 2.3 在 `backend-go/internal/tagmanagement/service/core/config_service.go` 删除 `LoadMatchThreshold` 方法（保留 `LoadClusterConfig` 和 `LoadConfig`）
- [x] 2.4 在 `backend-go/internal/tagmanagement/wire.go` 和 `service/service.go` 移除对已删除 handler/service 方法的引用（若有）
- [x] 2.5 在 `backend-go/internal/admin/handler/ai_handler.go` 移除 `SaveSettingsRequest.NarrativeBoardEmbeddingThreshold` 字段及其校验/`upsertAISetting` 调用
- [x] 2.6 在 `backend-go/internal/admin/wire.go` 检查并移除相关 handler 注册引用（若有）
- [x] 2.7 验证：`cd backend-go && go vet ./... && go build ./...` 通过
- [x] 2.8 验证：`cd backend-go && go test ./internal/tagmanagement/... ./internal/admin/...`（仅跑受影响包）

## 3. 前端死组件与 API 移除

- [x] 3.1 删除 `front/app/features/settings/components/SettingsSectionEmbedding.vue`
- [x] 3.2 删除 `front/app/features/ai/components/EmbeddingConfigPanel.vue`
- [x] 3.3 删除 `front/app/api/embeddingConfig.ts`
- [x] 3.4 在 `front/app/api/index.ts` 移除 `useEmbeddingConfigApi` 及 `EmbeddingConfigItem` 类型的导出
- [x] 3.5 在 `front/app/features/ai/composables/useAIRouterSettings.ts` 移除 `embeddingThreshold`、`savingThreshold`、`saveThreshold` 及其加载（`narrative_board_embedding_threshold` 读取）和 `loadData` 中的相关逻辑、return 语句中的导出
- [x] 3.6 在 `front/app/pages/settings.vue` 移除 `SettingsSectionEmbedding` 导入、`sectionComponents` 中的 `'embedding'` 键、模板中的渲染分支
- [x] 3.7 在 `front/app/components/dialog/GlobalSettingsDialog.vue` 移除 embedding 相关 tab/面板引用

## 4. 前端验证

- [x] 4.1 `cd front && pnpm lint`（WSL 可用）通过，无未使用变量/导入残留
- [x] 4.2 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"` 通过（必须 Windows cmd，捕获遗漏的类型引用）
- [x] 4.3 `cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"` 通过

## 5. 文档与收尾

- [x] 5.1 检查 `docs/` 下是否有描述 embedding 配置入口的文档需要同步更新（若有，移除对设置页 embedding 栏的引用）
- [x] 5.2 确认 `openspec/specs/tag-to-board-matching/spec.md` 现有 requirement 无需改动（本次为 ADDED，不触碰现有内容）
- [x] 5.3 全量回归确认：设置页可正常打开且无 embedding 栏；标签管理-匹配规则对话框功能正常
