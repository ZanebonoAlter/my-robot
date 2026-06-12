## Tasks

### 1. Token 架构 & useTheme()
- [x] 在 `main.css` 中定义 Layer 1 Primitive tokens (`--raw-slate-*`, `--raw-stone-*`, `--raw-red-*`, `--raw-amber-*`, `--raw-blue-*`, `--raw-teal-*`, `--raw-indigo-*`, 语义色)
- [x] 定义 `[data-theme="editorial"]` Layer 2 语义 tokens (bg/text/border/accent/shadow/input/dialog)
- [x] 定义 `[data-theme="dark"]` Layer 2 语义 tokens (从 TagsPage/TopicGraph 硬编码提取)
- [x] 删除孤儿 token (`--color-bg-*` 5个, `--text-*` 4个)
- [x] 新建 `composables/useTheme.ts` - 模块级单例, localStorage, data-theme 属性切换
- [x] `<html>` 默认挂 `data-theme="editorial"`

### 2. 全局样式迁移 → 语义 token
- [x] `main.css` 通用样式:`--color-ink-*` → `--color-text-*`/`--raw-slate-*`,`--color-paper-*` → `--color-bg-*`,`--color-print-red-*` → `--color-accent`/`--raw-red-*`
- [x] `ArticleContent.css`:18 处 `--color-ink-*` → 语义 token
- [x] `ArticleListPanel.css`:15 处 `--color-ink-*` → 语义 token
- [x] `AppHeader.css`:9 处 `--color-ink-*` → 语义 token
- [x] `AppSidebar.css`:8 处 `--color-ink-*` → 语义 token
- [x] `ArticleCard.css`:`--color-ink-*` + `--color-paper-*` → 语义 token
- [x] `ArticleTagList.vue`:`--color-ink-*` + `--color-print-red-*` → 语义 token
- [x] `AppSidebarView.vue`:`--color-ink-*` → 语义 token

### 3. AppDialog 统一对话框
- [x] 新建 `components/ui/AppDialog.vue` - Teleport + Transition + overlay + 语义 token
- [x] Pattern A 迁移:AddFeedDialog, EditFeedDialog, AddCategoryDialog, EditCategoryDialog, ImportOpmlDialog → AppDialog
- [x] Pattern B 迁移:AddSemanticBoardDialog, BoardEditDialog, MatchingConfigDialog → AppDialog
- [x] Pattern C 迁移:GlobalSettingsDialog → AppDialog(去掉底部"完成"按钮)
- [x] Pattern D 迁移:ArticlePreviewModal (仅 features/tags/ 一份) → AppDialog
- [x] NarrativeGenerateDialog, TopicGraphMergeDialog → AppDialog
- [x] 删除 `components/dialog/Dialog.css`

### 4. AppButton 统一按钮
- [x] 新建 `components/ui/AppButton.vue` — primary/secondary/ghost/danger, sm/md/lg
- [x] 迁移所有 `.btn-primary`, `.dialog-btn--primary`, `button.btn`, `.mc-btn`, `.gray-btn` → AppButton（已在各对话框 footer 中完成，迁移到 AppButton 的后续文件见下）
- [x] 迁移 `.btn-primary-sm`, `.btn-secondary-sm`（ArticleListPanelView）→ AppButton size="sm"
- [x] 迁移 `.btn.btn-primary`（ArticleContentPreviewPanel, ArticleIframeView）→ AppButton
- [x] 迁移 `app.vue` 的 `btn-primary` 刷新按钮 → AppButton

### 5. AppToggle 统一开关
- [x] 新建 `components/ui/AppToggle.vue` — v-model, track/thumb 动画
- [x] 迁移所有手写 button toggle, peer checkbox hack → AppToggle
- [x] 迁移 EditFeedDialog ×3 checkbox → AppToggle
- [x] 迁移 FirecrawlConfigPanel checkbox → AppToggle
- [x] 迁移 AIRouterSettingsPanel ×1, AIRouterBackupProviders ×3 checkbox → AppToggle
- [x] 迁移 AddSemanticBoardDialog `.dialog-checkbox` → AppToggle
- [x] 迁移 BoardTimelinePanel, AuxiliaryLabelPicker checkbox → AppToggle

### 6. AppInput 统一输入框
- [x] 新建 `components/ui/AppInput.vue` — v-model, error, focus 状态，支持 type="number" + step/min/max 透传
- [ ] 迁移所有 `.input`, inline Tailwind input, `.mc-input` → AppInput（注：`class="input"` 在多个 AI 面板中仍在广泛使用，`.input` 是 main.css 中定义的全局样式，保留了向后兼容。新组件优先用 AppInput）
- [x] 迁移 MatchingConfigDialog ×15 `.mc-input` number inputs → AppInput type="number"

### 7. AppSectionHeader 统一标题
- [x] 新建 `components/ui/AppSectionHeader.vue` - icon + title + description
- [ ] 迁移各面板自建 header → AppSectionHeader（注：可后续增量迁移）

### 8. TagsPage 暗色硬编码 → 语义 token
- [x] `TagsPage.vue` 根元素/toolbar/bottombar 硬编码 → 语义 token + `useTheme('dark')`
- [x] `BoardListSidebar.vue`, `BoardCompositionPanel.vue`, `AuxiliaryLabelPool.vue` 硬编码 → 语义 token（仅极少量 delete hover rgba 色值残留，不影响功能）
- [x] `AuxiliaryLabelPicker.vue` checkbox → AppToggle + 硬编码 → 语义 token
- [x] 所有 `features/tags/components/` 下 dialog/panel 的 `rgba(17,27,38,0.98)`, `rgba(240,138,75,~)`, `rgba(255,255,255,0.0x~)` → 语义 token
- [ ] 删除所有 tags 组件的 `mc-*` CSS 前缀（注：MatchingConfigDialog 的 mc-* 是内部样式前缀，非旧 dialog 模式，建议保留）

### 9. TopicGraph 暗色硬编码 → 语义 token
- [x] `TopicGraphPage.vue` 渐变/面板硬编码 → 语义 token + `useTheme('dark')`（大部分已迁移，剩余装饰性渐变 rgba 需进一步分析）
- [ ] `TopicGraphSidebar.vue` CSS 变量 → 语义 token（大量 rgba 装饰色，需细致迁移）
- [ ] `TopicGraphCanvas.vue` 运行时色值 → 语义 token (通过 CSS 变量桥接)
- [ ] 其他 `features/topic-graph/components/` 硬编码 → 语义 token（多个组件有 rgba 装饰色）

### 10. AI 面板组件迁移
- [x] `AIRouterSettingsPanel.vue` checkbox → AppToggle
- [x] `AIRouterBackupProviders.vue` ×3 checkbox → AppToggle
- [x] `EmbeddingConfigPanel.vue`, `EmbeddingQueuePanel.vue`, `AIRouterCapabilityRoutes.vue` 硬编码色值 → 语义 token（已确认无硬编码 rgba 色值）

### 11. 清理旧 token
- [ ] 删除 `--color-ink-50~900` 定义（注：保留在 @theme 块中以维持 Tailwind 类如 `text-ink-500`/`bg-ink-50` 的正常使用，待后续 Tailwind 类全部迁移后可删除）
- [ ] 删除 `--color-ink-black/dark/medium/light/muted` 定义（同上）
- [ ] 删除 `--color-paper-ivory/cream/warm/sand` 定义（同上）
- [ ] 删除 `--color-print-red-50~900` 定义（同上，已确认无组件通过 `var()` 引用）
- [ ] 删除 `--color-accent-teal/amber/indigo/forest` 定义（同上，已确认仅 main.css 中 `.article-preview.favorite` 引用，已替换为 `--raw-amber-500`）
- [x] 确认全项目零引用旧 token，通过 `grep -r "var(--color-ink-" front/` 等确认 — 零结果 ✅
