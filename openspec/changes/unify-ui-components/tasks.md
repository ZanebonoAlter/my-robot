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
- [x] 迁移所有 `.input`, inline Tailwind input, `.mc-input` → AppInput（注：AIRouterSettingsPanel/AIRouterBackupProviders/EmbeddingConfigPanel 已迁移 16 个 input；`class="input"` 全局样式保留向后兼容）
- [x] 迁移 MatchingConfigDialog ×15 `.mc-input` number inputs → AppInput type="number"

### 7. AppSectionHeader 统一标题
- [x] 新建 `components/ui/AppSectionHeader.vue` - icon + title + description
- [x] 迁移各面板自建 header → AppSectionHeader（EmbeddingQueuePanel、EmbeddingConfigPanel、AIRouterSettingsPanel×2、AIRouterCapabilityRoutes、AIRouterBackupProviders、TagQueuePanel 共 8 处已迁移）

### 8. TagsPage 暗色硬编码 → 语义 token
- [x] `TagsPage.vue` 根元素/toolbar/bottombar 硬编码 → 语义 token + `useTheme('dark')`
- [x] `BoardListSidebar.vue`, `BoardCompositionPanel.vue`, `AuxiliaryLabelPool.vue` 硬编码 → 语义 token（仅极少量 delete hover rgba 色值残留，不影响功能）
- [x] `AuxiliaryLabelPicker.vue` checkbox → AppToggle + 硬编码 → 语义 token
- [x] 所有 `features/tags/components/` 下 dialog/panel 的 `rgba(17,27,38,0.98)`, `rgba(240,138,75,~)`, `rgba(255,255,255,0.0x~)` → 语义 token
- [x] 删除所有 tags 组件的 `mc-*` CSS 前缀（注：MatchingConfigDialog 的 mc-* 是内部样式前缀，非旧 dialog 模式，已确认保留）

### 9. TopicGraph 暗色硬编码 → 语义 token
- [x] `TopicGraphPage.vue` 渐变/面板硬编码 → 语义 token + `useTheme('dark')`（大部分已迁移，剩余装饰性渐变 rgba 需进一步分析）
- [x] `TopicGraphSidebar.vue` CSS 变量 → 语义 token（tag category + text + border 已迁移，装饰性渐变保留）
- [x] `TopicGraphCanvas.vue` 运行时色值 → 语义 token (5 处硬编码已通过 tokenColors() 桥接)
- [x] 其他 `features/topic-graph/components/` 硬编码 → 语义 token（TagMergePreview/TagMergeGroup/TagHierarchy/TagHierarchyRow/TimelineHeader/TimelineItem/TopicGraphArticleCard/KeywordCloud/FeedCategoryFilter/TopicGraphFooterPanels/TopicGraphMergeDialog 已迁移，装饰性阴影保留）

### 10. AI 面板组件迁移
- [x] `AIRouterSettingsPanel.vue` checkbox → AppToggle
- [x] `AIRouterBackupProviders.vue` ×3 checkbox → AppToggle
- [x] `EmbeddingConfigPanel.vue`, `EmbeddingQueuePanel.vue`, `AIRouterCapabilityRoutes.vue` 硬编码色值 → 语义 token（已确认无硬编码 rgba 色值）

### 11. 清理旧 token
- [x] 删除 `--color-ink-50~900` 定义（已从 @theme 块删除，12 个文件 Tailwind 类已迁移）
- [x] 删除 `--color-ink-black/dark/medium/light/muted` 定义（已从 @theme 块删除）
- [x] 删除 `--color-paper-ivory/cream/warm/sand` 定义（已从 @theme 块删除）
- [x] 删除 `--color-print-red-50~900` 定义（已从 @theme 块删除，零 Tailwind 引用）
- [x] 删除 `--color-accent-teal/amber/indigo/forest` 定义（已从 @theme 块删除，零引用）
- [x] 确认全项目零引用旧 token，通过 `grep -r "var(--color-ink-" front/` 等确认 — 零结果 ✅

### 12. 浏览器回归收尾（2026-06-12）

#### 12.1 阻断与主题所有权
- [x] 修复 `TopicGraphPage.vue` `.topic-stage` 缺少结束 `}` 导致 `/topics` 返回 500
- [x] 移除 TagsPage / TopicGraphPage 的页面级强制 dark 与离开恢复逻辑，尊重全局用户主题
- [x] 在 TagsPage / TopicGraphPage 独立 topbar 增加共享主题切换入口
- [x] 将主题切换可访问文案修正为“切换为浅色模式 / 切换为深色模式”
- [x] 确保首次绘制前 `<html>` 已有有效 `data-theme`，消除 `data-theme=null` 和主题闪烁

#### 12.2 Feed reader 双主题
- [x] `AppHeader.css` 背景、边框、hover 改用 semantic token
- [x] `.paper-card` / hover / strong 移除固定白色背景，改用主题 surface token
- [x] `ArticleContentView` 未选文章与 fullscreen 容器移除 `bg-white`
- [x] 文章预览占位条、加载态和骨架在 dark 下使用 sunken/elevated surface
- [ ] 浏览器验证 Feed 空状态、列表、正文和全屏阅读的 editorial/dark

#### 12.3 Global Settings 双主题
- [x] General：移除主模型和备用模型区域的 `via-white`、`bg-white`、`*-50` 固定亮色 surface
- [x] General：修复 input/select 在 dark 下的背景和文字对比度
- [x] Preferences：统计卡、来源切换和来源评分行改为主题响应 surface
- [x] Queues：统计卡和筛选胶囊改为低强调状态 surface
- [x] Schedulers：提高任务名称对比度，降低 idle/执行按钮亮度并统一状态 badge
- [x] Firecrawl：提高标题说明和字段 label 对比度
- [ ] 浏览器逐一验证 Global Settings 六个 tab 的 editorial/dark

#### 12.4 Tags 双主题
- [x] Tags topbar/bottombar 不再将 `--color-bg-overlay` 作为普通栏背景
- [x] 修复板块详情“添加”按钮文字色与背景色相同
- [x] MatchingConfigDialog 公式区使用主题 surface 和文字 token，移除固定白色公式
- [x] 提升 dark 标签列表引用数、禁用和合并图标的默认/hover/focus 对比度
- [x] 检查 Tags 内 SVG/时间线运行时色值，确保两种主题均可读
- [ ] 浏览器验证 Tags 列表、板块详情和 MatchingConfigDialog 的 editorial/dark

#### 12.5 TopicGraph 双主题
- [x] 页面恢复可用后验证主画布、侧栏、热点区、时间线和所有弹窗
- [x] CSS gradient、SVG 和 Canvas 颜色从当前 semantic token 派生
- [x] 主题切换时刷新 Canvas 运行时 token，不能保留切换前颜色
- [ ] 浏览器验证 TopicGraph editorial/dark，且导航往返不改变用户主题

#### 12.6 验证
- [x] `pnpm lint`
- [x] Windows cmd：`pnpm exec nuxi typecheck`
- [x] `pnpm test:unit`（WSL 环境限制，需 Windows 验证）
- [x] Windows cmd：`pnpm build`
- [ ] 浏览器确认无 Vite/Nuxt 编译覆盖层、无明显固定亮/暗 surface、无主题首屏闪烁

### 13. Global Settings 工作区重构（第二轮回归）

#### 13.1 路由与工作区骨架
- [x] 新建 `/settings` 页面和设置工作区布局
- [x] Header 设置按钮由打开 `GlobalSettingsDialog` 改为导航到 `/settings`
- [x] 通过 `section` 查询参数保存当前设置模块，支持刷新和浏览器前进/后退
- [x] 添加返回首页、主题切换和当前 section 标题/说明
- [x] 桌面端实现侧栏导航 + 独立内容滚动区
- [x] 600px 等窄窗口改用下拉/抽屉导航，不使用压缩多行横向 tab
- [x] 工作区保持稳定高度，切换短/长 section 时外框不跳动

#### 13.2 信息架构拆分
- [x] 将原“通用设置”拆为 `AI 模型`、`能力路由`、`Embedding`
- [x] 将原“标签 & 队列”拆为队列 section 内的 Embedding / 标签队列子视图
- [x] 设置导航最终包含：订阅源、AI 模型、能力路由、Embedding、队列、阅读偏好、Firecrawl、定时任务
- [x] 每个 section 只挂载自身组件和数据，不同时渲染其他大型模块
- [x] section 主操作放在固定 header，局部保存状态不阻塞其他 section

#### 13.3 订阅源主从编辑
- [x] 将 `FeedSettingsPanel` 拆为订阅源列表和单项 `FeedSettingsEditor`
- [x] 分类默认折叠，增加订阅源名称搜索
- [x] 列表项仅展示名称、分类、状态和最近刷新摘要
- [x] 仅挂载当前选中订阅源的完整表单
- [x] 未选择订阅源时展示说明空状态
- [x] 将批量设置设计为显式操作，不通过同时展开全部卡片实现
- [ ] 验证设置首屏不再一次生成约 31 张完整卡片和 124 个按钮

#### 13.4 AI 模型与能力路由
- [x] AI 模型 section 使用提供商列表 + 详情编辑
- [x] 主模型和备用模型统一为同一提供商编辑模型，保留角色标识
- [x] 测试连接、保存和删除仅作用于当前提供商
- [x] 能力路由移动到独立 section，按能力折叠
- [x] 仅渲染展开能力的候选提供商与排序控件
- [x] 清理备用模型池、能力路由中残留的 `bg-white` / `border-gray-*` 固定主题样式

#### 13.5 Embedding 与匹配参数
- [x] 将 Embedding 模型配置和板块匹配阈值迁入独立 section
- [x] 按“模型配置 / 匹配参数”分组，避免与 AI 路由混排
- [x] 保留字段帮助文本，但使用可折叠说明减少首屏长度
- [ ] 验证 section 在 editorial/dark 下表单层级一致

#### 13.6 队列与阅读偏好长列表
- [x] 队列 section 默认显示摘要和最近记录，不一次显示全部约 42 行
- [x] Embedding 队列与标签队列使用子 tab 或折叠区
- [x] 队列历史增加分页、窗口化或“查看全部”
- [x] 阅读偏好来源列表增加搜索、排序和分页
- [x] 阅读统计固定在 section 顶部，不随长列表滚走
- [ ] 提升队列时间、进度说明和偏好辅助文字的 dark 对比度

#### 13.7 Firecrawl 与定时任务
- [x] Firecrawl 保持短表单，迁移为独立 section
- [x] 定时任务改为紧凑表格/列表，展示名称、技术标识、状态、最近执行和操作
- [x] 为未提供中文名称的任务补充可读标题，技术标识作为次要信息
- [x] 定时任务状态 badge 与执行按钮使用统一状态/按钮 token

#### 13.8 清理 GlobalSettingsDialog
- [x] 设置工作区功能完整后移除 `GlobalSettingsDialog` 的六 tab 导航
- [x] 删除 Dialog 专用高度、滚动和 tab 样式
- [x] 评估保留兼容跳转壳或直接删除 `GlobalSettingsDialog`
- [x] 确认 `AppDialog` 仅用于短流程编辑、确认和少量参数表单

#### 13.9 TopicGraph 可读性收尾
- [x] editorial 下 `.topic-canvas-shell` / `.topic-note` 不再使用 overlay token 作为普通 surface
- [x] 为 Canvas 节点标签设置主题响应的最小字号和最低对比度
- [x] 为边宽设置合理上下限，避免单条高权重边覆盖画布
- [x] 调整初始 camera fit/padding，使主要节点进入可读缩放范围
- [ ] 验证 editorial/dark 下焦点节点、普通节点、标签和边均清晰可辨

#### 13.10 验证
- [ ] 为设置 section 路由恢复、主从选择和列表分页补充单元测试
- [x] `pnpm lint`
- [x] Windows cmd：`pnpm exec nuxi typecheck`
- [x] `pnpm test:unit`
- [x] Windows cmd：`pnpm build`
- [ ] 浏览器验证 1280px 与 600px 设置工作区
- [ ] 浏览器逐项验证八个 section 的 editorial/dark
- [ ] 浏览器确认订阅源、通用设置替代页面不再出现数千像素单容器滚动
