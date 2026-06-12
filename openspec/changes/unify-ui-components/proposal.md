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

## Capabilities

### New Capabilities
- `theme-system`: 三层 CSS 变量 token 架构（Primitive → Semantic → Component），`data-theme` 双主题切换，`useTheme()` composable
- `unified-dialog`: 统一对话框外壳组件 AppDialog，含 Teleport、Transition、overlay、关闭行为
- `unified-form-controls`: 统一表单原子组件（AppToggle、AppButton、AppInput、AppSectionHeader），响应主题 token

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
