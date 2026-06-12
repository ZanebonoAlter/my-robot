## Why

前端三大入口页面（主阅读页、标签页、图谱页）和 GlobalSettings 对话框存在严重的 UI 分裂：四套不同的对话框实现、三种 toggle 样式、三种按钮样式、三种输入框样式、两套完全独立的视觉语言。TagsPage 和 TopicGraphPage 的暗色样式全部硬编码（`rgba()` 直接写在 scoped CSS 里），没有走 CSS 变量系统。`main.css` 定义了 53 个 token 但其中 9 个从未被引用，`--text-*` 和 `--color-bg-*` 完全孤儿。组件样式一半用 CSS 变量、一半用 Tailwind、一半硬编码，毫无一致性。

## What Changes

### 主题系统

- **建立三层 Token 架构**：Layer 1 原始色值（`--raw-*`）→ Layer 2 语义色（`--color-*`，跟主题走）→ Layer 3 组件 token（`--dialog-*`、`--button-*`，可选）
- **建立双主题切换**：`data-theme="editorial"`（暖白印刷厂）和 `data-theme="dark"`（深色调查风格），通过 `<html>` 属性切换
- **从现有硬编码提取 dark 主题色值**：TagsPage/TopicGraph 的暗色已经在生产环境验证，提取为语义 token
- **主题切换 API**：`useTheme()` composable，页面级切换 + localStorage 记忆
- **清理孤儿 token**：删除从未引用的 `--color-bg-*`（5个）和 `--text-*`（4个）
- **迁移全部旧 token**：`--color-ink-*` → `--raw-slate-*`，`--color-paper-*` → `--raw-stone-*`，`--color-print-red-*` → `--raw-red-*`

### 统一组件

- **新建 `AppDialog` 统一对话框组件**：Teleport + Transition + overlay + 语义 token，替代现有四套 dialog pattern
- **新建 `AppToggle` 统一开关组件**：响应主题 token，替代自建 button toggle、peer checkbox、原生 checkbox
- **新建 `AppButton` 统一按钮组件**：primary / secondary / ghost 变体，响应主题 token
- **新建 `AppInput` 统一输入框组件**：响应主题 token，替代 `.input`、inline Tailwind、`.mc-input` 三套样式
- **新建 `AppSectionHeader` 统一 section 标题组件**：可选 icon box + 标题 + 描述
- **改造 `GlobalSettingsDialog`**：去掉底部"完成"按钮，各面板自行管理保存；迁移到 `AppDialog` 外壳
- **迁移所有现有对话框**：Pattern A/B/C/D 统一使用 `AppDialog`
- **迁移 TagsPage/TopicGraph 暗色硬编码**：全部替换为语义 token

### 清理

- **删除废弃文件**：`Dialog.css`
- **删除旧 token 定义**：`--color-ink-*`、`--color-paper-*`、`--color-print-red-*`、孤儿 `--color-bg-*`、孤儿 `--text-*`

### 浏览器回归补充（2026-06-12）

顶部全局主题切换加入后，TagsPage 和 TopicGraphPage 不再是固定暗色页面，原设计中的“页面级主题锁定”前提失效。本 change 追加以下收尾范围：

- 三个入口页面和 Global Settings 的全部可见区域必须同时支持 `editorial` / `dark`
- 页面栏、卡片、空状态、表单、统计卡、图表、SVG 和运行时 Canvas 色值均纳入主题适配
- 独立页面保留可访问的主题切换入口，不要求用户返回首页切换
- 修复 TopicGraph 样式编译错误，恢复 `/topics` 页面可用性后再进行双主题验收
- 消除主题首屏闪烁，首次绘制前 `<html>` 必须已有有效 `data-theme`
- 主题切换按钮文案使用“浅色模式 / 深色模式”，不再使用容易误解的“编辑模式”

### 第二轮回归补充：设置工作区

主题适配完成后，Global Settings 仍存在结构性问题：约 `550px` 高的 Dialog 承载了最高 `8652px` 的内容、数十张卡片和上百个交互控件。继续在 Dialog 内微调颜色和间距无法解决导航、性能和编辑效率问题。

- **将 Global Settings 从 Dialog 升级为独立设置工作区**
- **按领域拆分导航**：订阅源、AI 模型、能力路由、Embedding、队列、阅读偏好、Firecrawl、定时任务
- **保留 Header 设置入口**：点击后进入设置路由，而不是打开超长 Dialog
- **订阅源采用主从布局**：可搜索列表 + 单项编辑器，不一次展开所有订阅源表单
- **长列表采用折叠、分页或虚拟化**：队列、阅读偏好和订阅源不得一次渲染全部数据
- **AppDialog 回归轻量职责**：继续承载新增、编辑、确认等短流程，不承载完整管理后台

## Capabilities

### New Capabilities
- `theme-system`: 三层 CSS 变量 token 架构（Primitive → Semantic → Component），`data-theme` 双主题切换，`useTheme()` composable
- `unified-dialog`: 统一对话框外壳组件 AppDialog，含 Teleport、Transition、overlay、关闭行为
- `unified-form-controls`: 统一表单原子组件（AppToggle、AppButton、AppInput、AppSectionHeader），响应主题 token
- `settings-workspace`: 独立设置工作区，提供领域导航、主从编辑、长列表治理和响应式布局

### Modified Capabilities

（无已有 spec 需要修改）

## Impact

- **受影响文件（~35+ 组件）**：
  - `assets/css/main.css`：重写为三层 token 架构
  - `app.vue`：`btn-primary` → AppButton
  - `components/dialog/` 下所有对话框（GlobalSettingsDialog、AddFeedDialog、EditFeedDialog、AddCategoryDialog、EditCategoryDialog、ImportOpmlDialog、FirecrawlConfigPanel）
  - `features/ai/components/` 下 AI 配置面板（AIRouterSettingsPanel、AIRouterBackupProviders）— checkbox → AppToggle
  - `features/articles/components/` 下按钮样式（ArticleContentPreviewPanel、ArticleIframeView）— btn → AppButton
  - `features/shell/components/` 下按钮样式（ArticleListPanelView）— btn-primary-sm / btn-secondary-sm → AppButton
  - `features/tags/components/` 下所有暗色对话框和面板
  - `features/topic-graph/components/` 下对话框和面板
  - `components/article/ArticleContent.css`、`components/layout/ArticleListPanel.css`、`components/layout/AppHeader.css`、`components/layout/AppSidebar.css`：迁移到语义 token
- **删除文件**：`components/dialog/Dialog.css`
- **无后端影响**：纯前端 UI 层变更
- **无 API 变更**：不涉及数据层
