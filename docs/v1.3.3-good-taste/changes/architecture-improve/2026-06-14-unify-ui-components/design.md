## Architecture Overview

```
┌─────────────────────────────────────────────────────────────────┐
│ Token 三层架构                                                   │
│                                                                  │
│ Layer 1: Primitive (--raw-*)                                     │
│   色值的原始定义，不直接使用                                      │
│   --raw-slate-50 ~ 900, --raw-stone-50 ~ 900, --raw-red-50 ~ 900│
│   --raw-amber-500, --raw-teal-500, --raw-indigo-500             │
│                                                                  │
│ Layer 2: Semantic (--color-*)                                    │
│   跟主题走，组件直接引用这一层                                    │
│   [data-theme="editorial"] { --color-bg-base: var(--raw-stone-50) }
│   [data-theme="dark"]      { --color-bg-base: #080c12 }          │
│                                                                  │
│ Layer 3: Component (--dialog-*, --button-*, etc.)                │
│   可选，复杂组件的局部 token                                      │
│   .app-dialog { --dialog-bg: var(--color-bg-elevated) }          │
│                                                                  │
│ 主题切换: <html data-theme="editorial" | "dark">                 │
│ 控制: useTheme() composable                                      │
└─────────────────────────────────────────────────────────────────┘
```

## Layer 1: Primitive Tokens

原始色值，项目色板的唯一来源。不直接在组件中使用。

```css
:root {
  /* Slate (蓝灰 — 替代旧 ink-*，主文字/边框色系) */
  --raw-slate-50:  #f0f4f8;
  --raw-slate-100: #d9e2ec;
  --raw-slate-200: #bcccdc;
  --raw-slate-300: #9fb3c8;
  --raw-slate-400: #829ab1;
  --raw-slate-500: #627d98;
  --raw-slate-600: #486581;
  --raw-slate-700: #334e68;
  --raw-slate-800: #243b53;
  --raw-slate-900: #102a43;

  /* Stone (暖灰 — 替代旧 paper-*，背景色系) */
  --raw-stone-50:  #faf7f2;
  --raw-stone-100: #f5f0e6;
  --raw-stone-200: #e8dfd1;
  --raw-stone-300: #d4c9b8;
  --raw-stone-400: #b8a994;
  --raw-stone-500: #9a8a74;
  --raw-stone-600: #7d6e5b;
  --raw-stone-700: #5f5344;
  --raw-stone-800: #43392f;
  --raw-stone-900: #2a221b;

  /* Red (红色 — 替代旧 print-red-*，强调色系) */
  --raw-red-50:  #fef2f2;
  --raw-red-100: #fde6e6;
  --raw-red-200: #f9c5c5;
  --raw-red-300: #f29a9a;
  --raw-red-400: #e87070;
  --raw-red-500: #d94a4a;
  --raw-red-600: #c12f2f;
  --raw-red-700: #a41f1f;
  --raw-red-800: #8a1818;
  --raw-red-900: #721515;

  /* Amber (琥珀 — dark 主题强调色) */
  --raw-amber-400: #f0a24b;
  --raw-amber-500: #d4883c;

  /* Blue (蓝色 — 辅助色) */
  --raw-blue-400: #63b3ed;
  --raw-blue-500: #3f7cff;

  /* Teal (青色 — 成功色系) */
  --raw-teal-500: #2d8a7a;
  --raw-teal-600: #3d8a4a;

  /* Indigo (靛蓝 — 信息色系) */
  --raw-indigo-500: #4a5d8a;

  /* Semantic (语义色，不随主题变) */
  --raw-success: #3d8a4a;
  --raw-warning: #c4883c;
  --raw-error:   #c42f3c;
  --raw-info:    #3d7a8a;
}
```

## Layer 2: Semantic Tokens

组件直接引用的语义色。每个主题定义一套完整映射。

### Editorial Theme (印刷厂)

```css
[data-theme="editorial"] {
  /* 背景 */
  --color-bg-base:     var(--raw-stone-50);        /* #faf7f2 页面底 */
  --color-bg-elevated: var(--raw-stone-100);        /* #f5f0e6 卡片/面板 */
  --color-bg-sunken:   var(--raw-stone-200);        /* #e8dfd1 嵌入区域 */
  --color-bg-overlay:  rgba(26, 26, 26, 0.4);      /* 对话框遮罩 */
  --color-bg-hover:    rgba(255, 255, 255, 0.85);
  --color-bg-active:   rgba(255, 255, 255, 0.6);

  /* 文字 */
  --color-text-primary:   #1a1a1a;
  --color-text-secondary: #5a5a5a;
  --color-text-muted:     #8a8a8a;
  --color-text-inverted:  #faf7f2;

  /* 边框 */
  --color-border-subtle: rgba(26, 26, 26, 0.08);
  --color-border-medium: rgba(26, 26, 26, 0.15);
  --color-border-strong: rgba(26, 26, 26, 0.25);

  /* 强调 */
  --color-accent:       var(--raw-red-500);         /* #d94a4a */
  --color-accent-hover: var(--raw-red-600);
  --color-accent-subtle: rgba(217, 74, 74, 0.08);

  /* 辅助 */
  --color-secondary: var(--raw-amber-500);
  --color-tertiary:  var(--raw-blue-400);

  /* 阴影 */
  --shadow-subtle: 0 1px 3px rgba(26, 26, 26, 0.06);
  --shadow-medium: 0 2px 8px rgba(26, 26, 26, 0.08);
  --shadow-strong: 0 4px 16px rgba(26, 26, 26, 0.12);
  --shadow-print:  0 1px 0 rgba(26, 26, 26, 0.1), 0 2px 4px rgba(26, 26, 26, 0.06);

  /* 语义 (不变，但挂在这里方便组件引用) */
  --color-success: var(--raw-success);
  --color-warning: var(--raw-warning);
  --color-error:   var(--raw-error);
  --color-info:    var(--raw-info);

  /* 输入框 */
  --color-input-bg:    rgba(255, 255, 255, 0.6);
  --color-input-border: rgba(26, 26, 26, 0.12);
  --color-input-focus:  var(--raw-red-500);

  /* 对话框 */
  --color-dialog-bg:      rgba(255, 255, 255, 0.95);
  --color-dialog-header:  transparent;
  --color-dialog-divider: rgba(26, 26, 26, 0.06);

  /* Tag category colors */
  --color-tag-event:          rgba(196, 136, 60, 0.85);
  --color-tag-event-bg:       rgba(196, 136, 60, 0.12);
  --color-tag-event-border:   rgba(196, 136, 60, 0.25);
  --color-tag-person:         rgba(45, 138, 122, 0.85);
  --color-tag-person-bg:      rgba(45, 138, 122, 0.12);
  --color-tag-person-border:  rgba(45, 138, 122, 0.25);
  --color-tag-keyword:        rgba(74, 93, 138, 0.85);
  --color-tag-keyword-bg:     rgba(74, 93, 138, 0.12);
  --color-tag-keyword-border: rgba(74, 93, 138, 0.25);

  /* Interactive/link blue */
  --color-link:            rgba(63, 124, 255, 0.9);
  --color-link-hover:      rgba(63, 124, 255, 1);
  --color-link-subtle:     rgba(63, 124, 255, 0.08);
  --color-link-border:     rgba(63, 124, 255, 0.2);
  --color-link-border-hover: rgba(63, 124, 255, 0.4);
}
```

### Dark Theme (调查风格)

```css
[data-theme="dark"] {
  /* 背景 — 从 TagsPage/TopicGraph 硬编码提取 */
  --color-bg-base:     #080c12;
  --color-bg-elevated: rgba(17, 27, 38, 0.98);
  --color-bg-sunken:   #0e161d;
  --color-bg-overlay:  rgba(8, 12, 18, 0.75);
  --color-bg-hover:    rgba(255, 255, 255, 0.06);
  --color-bg-active:   rgba(255, 255, 255, 0.08);

  /* 文字 */
  --color-text-primary:   rgba(255, 255, 255, 0.9);
  --color-text-secondary: rgba(255, 255, 255, 0.6);
  --color-text-muted:     rgba(255, 255, 255, 0.35);
  --color-text-inverted:  #1a1a1a;

  /* 边框 */
  --color-border-subtle: rgba(255, 255, 255, 0.06);
  --color-border-medium: rgba(255, 255, 255, 0.10);
  --color-border-strong: rgba(255, 255, 255, 0.18);

  /* 强调 — 琥珀色 (不是红色) */
  --color-accent:       rgba(240, 138, 75, 0.85);
  --color-accent-hover: rgba(240, 138, 75, 1);
  --color-accent-subtle: rgba(240, 138, 75, 0.08);

  /* 辅助 */
  --color-secondary: rgba(99, 179, 237, 0.85);
  --color-tertiary:  rgba(63, 124, 255, 0.85);

  /* 阴影 — 暗色下阴影微弱 */
  --shadow-subtle: 0 1px 3px rgba(0, 0, 0, 0.3);
  --shadow-medium: 0 2px 8px rgba(0, 0, 0, 0.4);
  --shadow-strong: 0 4px 16px rgba(0, 0, 0, 0.5);
  --shadow-print:  0 1px 0 rgba(0, 0, 0, 0.2), 0 2px 4px rgba(0, 0, 0, 0.3);

  /* 语义 */
  --color-success: var(--raw-success);
  --color-warning: var(--raw-warning);
  --color-error:   var(--raw-error);
  --color-info:    var(--raw-info);

  /* 输入框 */
  --color-input-bg:    rgba(255, 255, 255, 0.04);
  --color-input-border: rgba(255, 255, 255, 0.10);
  --color-input-focus:  rgba(240, 138, 75, 0.85);

  /* 对话框 */
  --color-dialog-bg:      rgba(17, 27, 38, 0.98);
  --color-dialog-header:  rgba(255, 255, 255, 0.03);
  --color-dialog-divider: rgba(255, 255, 255, 0.06);

  /* Tag category colors */
  --color-tag-event:          rgba(245, 158, 11, 0.72);
  --color-tag-event-bg:       rgba(245, 158, 11, 0.18);
  --color-tag-event-border:   rgba(245, 158, 11, 0.32);
  --color-tag-person:         rgba(16, 185, 129, 0.72);
  --color-tag-person-bg:      rgba(16, 185, 129, 0.18);
  --color-tag-person-border:  rgba(16, 185, 129, 0.32);
  --color-tag-keyword:        rgba(99, 102, 241, 0.72);
  --color-tag-keyword-bg:     rgba(99, 102, 241, 0.18);
  --color-tag-keyword-border: rgba(99, 102, 241, 0.32);

  /* Interactive/link blue */
  --color-link:            rgba(99, 179, 237, 0.9);
  --color-link-hover:      rgba(99, 179, 237, 1);
  --color-link-subtle:     rgba(99, 179, 237, 0.1);
  --color-link-border:     rgba(99, 179, 237, 0.25);
  --color-link-border-hover: rgba(99, 179, 237, 0.5);
}
```

## Theme Switching

### useTheme() Composable

```typescript
// composables/useTheme.ts

type Theme = 'editorial' | 'dark'

const STORAGE_KEY = 'syntopica-theme'

// 全局状态，跨组件共享
const currentTheme = ref<Theme>(
  (localStorage.getItem(STORAGE_KEY) as Theme) || 'editorial'
)

export function useTheme() {
  function setTheme(theme: Theme) {
    currentTheme.value = theme
    document.documentElement.setAttribute('data-theme', theme)
    localStorage.setItem(STORAGE_KEY, theme)
  }

  function toggleTheme() {
    setTheme(currentTheme.value === 'editorial' ? 'dark' : 'editorial')
  }

  // 初始化：确保 DOM 和状态同步
  onMounted(() => {
    document.documentElement.setAttribute('data-theme', currentTheme.value)
  })

  return {
    theme: readonly(currentTheme),
    setTheme,
    toggleTheme,
    isDark: computed(() => currentTheme.value === 'dark'),
  }
}
```

### 页面级主题锁定

TagsPage 和 TopicGraph 进入时自动切换到 dark，离开时恢复：

```typescript
// 在 TagsPage.vue / TopicGraphPage.vue 中
const { setTheme } = useTheme()
const previousTheme = ref<string>()

onMounted(() => {
  previousTheme.value = document.documentElement.getAttribute('data-theme') || 'editorial'
  setTheme('dark')
})

onUnmounted(() => {
  setTheme(previousTheme.value as Theme)
})
```

## Layer 3: Component Tokens

组件 token 是可选的。简单组件直接用 Layer 2 语义 token。只有组件内部需要多级色值时才定义 Layer 3。

### AppDialog

```css
.app-dialog {
  --dialog-bg:       var(--color-dialog-bg);
  --dialog-text:     var(--color-text-primary);
  --dialog-border:   var(--color-border-subtle);
  --dialog-accent:   var(--color-accent);
  --dialog-divider:  var(--color-dialog-divider);
  --dialog-header:   var(--color-dialog-header);
  --dialog-radius:   12px;
  --dialog-shadow:   var(--shadow-strong);
  --dialog-overlay:  var(--color-bg-overlay);
}
```

### AppButton

```css
.app-button {
  --button-text:     var(--color-text-inverted);
  --button-bg:       var(--color-accent);
  --button-bg-hover: var(--color-accent-hover);
  --button-radius:   8px;
}

.app-button--secondary {
  --button-text:     var(--color-text-primary);
  --button-bg:       var(--color-bg-hover);
  --button-bg-hover: var(--color-bg-active);
}

.app-button--ghost {
  --button-text:     var(--color-text-secondary);
  --button-bg:       transparent;
  --button-bg-hover: var(--color-bg-hover);
}
```

### AppInput

```css
.app-input {
  --input-bg:        var(--color-input-bg);
  --input-border:    var(--color-input-border);
  --input-focus:     var(--color-input-focus);
  --input-text:      var(--color-text-primary);
  --input-placeholder: var(--color-text-muted);
  --input-radius:    8px;
}
```

## Migration Map

### 旧 Token → 新 Token 映射

```
旧                              → 新
─────────────────────────────────────────────────────
--color-ink-black               → --color-text-primary (editorial)
--color-ink-dark                → --color-text-secondary
--color-ink-medium              → --color-text-secondary
--color-ink-light               → --color-text-muted
--color-ink-muted               → --color-text-muted
--color-ink-50 ~ 900            → --raw-slate-50 ~ 900 (如需直接使用)
--color-paper-ivory             → --color-bg-base
--color-paper-cream             → --color-bg-elevated
--color-paper-warm              → --color-bg-sunken
--color-paper-sand              → --color-bg-sunken
--color-print-red-500           → --color-accent (editorial)
--color-print-red-*             → --raw-red-* (如需直接使用)
--color-bg-primary              → 删除 (孤儿)
--color-bg-secondary            → 删除 (孤儿)
--color-bg-tertiary             → 删除 (孤儿)
--color-bg-card                 → 删除 (孤儿)
--color-bg-hover                → 删除旧定义，由 Layer 2 --color-bg-hover 替代
--text-primary                  → 删除 (孤儿)
--text-secondary                → 删除 (孤儿)
--text-muted                    → 删除 (孤儿)
--text-inverted                 → 删除 (孤儿)
--color-border-subtle           → --color-border-subtle (同名，值由主题决定)
--color-border-medium           → --color-border-medium
--color-border-strong           → --color-border-strong
--shadow-subtle                 → --shadow-subtle (同名)
--shadow-medium                 → --shadow-medium
--shadow-strong                 → --shadow-strong
--shadow-print                  → --shadow-print
--color-success                 → --color-success (同名)
--color-warning                 → --color-warning
--color-error                   → --color-error
--color-info                    → --color-info
--color-accent-teal             → --raw-teal-500
--color-accent-amber            → --raw-amber-500
--color-accent-indigo           → --raw-indigo-500
--color-accent-forest           → --raw-teal-600
```

### 硬编码 → 语义 Token 映射 (TagsPage / TopicGraph)

```
硬编码                              → 语义 token
──────────────────────────────────────────────────
#080c12                            → --color-bg-base
rgba(17, 27, 38, 0.98)             → --color-bg-elevated
#0e161d                            → --color-bg-sunken
rgba(8, 12, 18, 0.75)              → --color-bg-overlay
rgba(240, 138, 75, 0.85)           → --color-accent
rgba(240, 138, 75, 0.08)           → --color-accent-subtle
rgba(99, 179, 237, ~)              → --color-secondary
rgba(63, 124, 255, ~)              → --color-tertiary
rgba(255, 255, 255, 0.9)           → --color-text-primary
rgba(255, 255, 255, 0.6)           → --color-text-secondary
rgba(255, 255, 255, 0.35)          → --color-text-muted
rgba(255, 255, 255, 0.04~0.06)     → --color-border-subtle
rgba(255, 255, 255, 0.10~0.12)     → --color-border-medium
rgba(255, 255, 255, 0.06)          → --color-bg-hover
```

## File Changes

### 修改文件清单

```
main.css                          → 重写为三层 token，删除旧 token
app.vue                           → btn-primary → AppButton
Dialog.css                        → 删除
GlobalSettingsDialog.vue          → 迁移到 AppDialog + Tailwind → token
FirecrawlConfigPanel.vue          → checkbox → AppToggle
ArticleContent.css                → ink-*/paper-* → 语义 token
ArticleListPanel.css              → ink-* → 语义 token，btn-primary-sm / btn-secondary-sm 样式迁移到 AppButton
AppHeader.css                     → ink-* → 语义 token
AppSidebar.css                    → ink-* → 语义 token
ArticleCard.css                   → ink-*/paper-* → 语义 token
ArticleTagList.vue                → ink-*/print-red-* → 语义 token
AppSidebarView.vue                → ink-* → 语义 token
ArticleContentPreviewPanel.vue    → btn → AppButton
ArticleIframeView.vue             → btn → AppButton
ArticleListPanelView.vue          → btn-primary-sm / btn-secondary-sm → AppButton
AIRouterSettingsPanel.vue         → checkbox → AppToggle
AIRouterBackupProviders.vue       → checkbox → AppToggle
ArticleContentPreviewPanel.vue    → btn → AppButton
ArticleIframeView.vue             → btn → AppButton
ArticleListPanelView.vue          → btn-primary-sm / btn-secondary-sm → AppButton
AIRouterSettingsPanel.vue         → checkbox → AppToggle
AIRouterBackupProviders.vue       → checkbox → AppToggle

TagsPage.vue                      → 硬编码 → 语义 token + useTheme('dark')
所有 tags/components/*.vue        → 硬编码 → 语义 token
TopicGraphPage.vue                → 硬编码 → 语义 token + useTheme('dark')
所有 topic-graph/components/*.vue → 硬编码 → 语义 token
```

### 新建文件清单

```
composables/useTheme.ts           → 主题切换 composable
components/ui/AppDialog.vue       → 统一对话框
components/ui/AppButton.vue       → 统一按钮
components/ui/AppToggle.vue       → 统一开关
components/ui/AppInput.vue        → 统一输入框
components/ui/AppSectionHeader.vue → 统一标题
```

## Constraints

- `data-theme` 挂在 `<html>` 上，所有 CSS 变量自动级联
- 不使用 Tailwind 的 `dark:` 前缀，因为 editorial/dark 不是简单的明暗翻转
- Primitive token (`--raw-*`) 只在 Layer 2 定义中引用，不在组件中直接使用
- 组件中如果需要非语义的特定色值（如图表颜色），允许直接引用 `--raw-*`
- `useTheme()` 是全局单例（模块级 ref），不是 per-component 实例
- 主题切换不需要 transition 动画（会闪烁），直接切换即可
- Nuxt SSR 兼容：`useTheme()` 初始化应通过 `useHead({ htmlAttrs: { 'data-theme': ... } })` 设置 html 属性，避免 FOUC；`onMounted` 做客户端同步
- GlobalSettingsDialog 的面板迁移在本 change 中完成，但各面板的内部逻辑不改动

## Browser Regression Follow-up (2026-06-12)

### Updated Theme Model

新增顶部主题切换后，主题由用户全局选择，不再由页面强制决定：

```text
localStorage / SSR default
          |
          v
<html data-theme="editorial|dark">
          |
          +--> Feed reader
          +--> Tags
          +--> Topic graph
          +--> Global Settings
```

- 删除 TagsPage / TopicGraphPage 的进入时强制 `setTheme('dark')` 和离开时恢复逻辑
- TagsPage、TopicGraphPage 提供与主 Header 一致的主题切换入口
- 主题值只由 `useTheme()` 维护，页面组件只消费，不覆盖用户选择

### Findings and Solutions

| 区域 | 浏览器发现 | 解决方案 |
|---|---|---|
| TopicGraph | `/topics` 因 `.topic-stage` 缺少 `}` 返回 500 | 先修复 CSS 语法并增加页面烟雾验证；页面可加载是后续主题验收前置条件 |
| Feed Header | dark 下仍是白色半透明 Header | 背景、边框、hover 全部改用 `--color-bg-elevated`、`--color-border-*`、`--color-bg-hover` |
| Feed article cards | dark 下文章卡仍使用 `rgba(255,255,255,0.7)` | `.paper-card` / hover / strong 使用语义 surface token；必要时增加 `--color-bg-card` 语义 token，不复用 overlay |
| Feed empty/fullscreen | 未选文章和全屏容器使用 `bg-white` | 改为 `--color-bg-base` / `--color-bg-elevated`，标题和说明使用语义文字 token |
| Feed preview | 文章顶部占位条在 dark 下近白 | 骨架、占位和加载态使用 `--color-bg-sunken` 与 `--color-border-subtle` |
| Theme toggle | dark 下提示“切换为编辑模式” | aria-label/title 统一为“切换为浅色模式”或“切换为深色模式” |
| Global Settings / General | 主模型区域保留亮白渐变，表单层级失真 | 删除 `from-*-50/via-white/to-*` 主题色 Tailwind；卡片、header、input、button 改用语义 token / 统一组件 |
| Global Settings / Preferences | 统计卡和来源评分卡仍为亮色 | 为统计卡定义主题响应的状态 surface；来源行使用 elevated/sunken surface，保留状态色但降低 dark 下亮度 |
| Global Settings / Queues | 统计卡和筛选胶囊偏亮 | 使用状态色的低透明背景 + 语义文字；筛选器使用 AppButton 或统一 chip token |
| Global Settings / Schedulers | 任务名过暗，idle/执行按钮过亮 | 任务名使用 primary/secondary text；状态 badge 使用状态 token；执行按钮使用 secondary/ghost 变体 |
| Global Settings / Firecrawl | 辅助文字和字段标签对比度不足 | 标签至少使用 `--color-text-secondary`，说明使用 `--color-text-muted`，避免额外 opacity 叠加 |
| Tags top bar | editorial 下使用 overlay token 导致整栏发灰 | 普通栏背景使用 elevated/半透明 surface；`--color-bg-overlay` 仅用于模态遮罩 |
| Tags board detail | “添加”按钮文字与背景同色 | primary 按钮文字必须使用 `--color-text-inverted`，增加 computed-style / 视觉回归检查 |
| MatchingConfigDialog | editorial 下公式区灰底白字，对比度不足 | 公式容器使用 sunken surface，公式文字使用 primary/secondary token；数学渲染不得固定白色 |
| Tags dark list | 引用数和右侧操作图标过暗 | 提升到 muted/secondary token；交互图标 hover/focus 必须有可辨识反馈 |
| Tags navigation | 页面内无主题切换入口 | 在独立页 topbar 放置共享 ThemeToggle，不复制主题状态逻辑 |
| Theme initialization | 首次加载时 `data-theme` 短暂为 null | SSR/head 阶段设置默认主题；客户端在首绘前读取持久值并同步，禁止等待普通 `onMounted` 才设置 |

### Token Usage Rules

- `--color-bg-overlay` 仅表示覆盖页面内容的模态遮罩，不得作为 header、toolbar、card 或 panel 背景。
- 页面 surface 按层级使用 `base -> elevated -> sunken`；hover/active 只用于短暂交互状态。
- 状态卡可以保留蓝、绿、黄、红色相，但背景必须使用低透明度状态 token，不能直接使用 Tailwind `*-50`。
- dark 主题中的普通内容面不得出现接近纯白的大面积背景；反色白底仅允许明确的媒体内容或第三方 iframe。
- SVG、Canvas 和 CSS gradient 的颜色必须由当前主题 token 派生；切换主题后应刷新运行时色值。

### Verification Matrix

浏览器验收覆盖以下组合：

| 页面/区域 | editorial | dark |
|---|---:|---:|
| Feed 空状态、文章列表、文章正文、全屏阅读 | required | required |
| Global Settings 六个 tab | required | required |
| Tags 列表、板块详情、MatchingConfigDialog | required | required |
| TopicGraph 主画布、侧栏、时间线、弹窗 | required | required |

每个组合检查：

1. 页面可加载且无 Vite/Nuxt 编译覆盖层。
2. 无非媒体用途的大面积固定白色或固定深色 surface。
3. 文字、图标、按钮和输入框在静态、hover、focus、disabled 状态下可辨识。
4. 主题切换后页面不跳回另一主题，刷新后选择保持。
5. 首次绘制时 `<html>` 已包含有效 `data-theme`，无明显主题闪烁。

## Settings Workspace Follow-up (2026-06-12)

### Decision

Global Settings 不再继续扩展 `AppDialog`。设置入口迁移到独立路由，建议使用 `/settings`，并通过查询参数或子路由保存当前位置：

```text
/settings?section=feeds
/settings?section=ai-providers
/settings?section=capability-routes
/settings?section=embedding
/settings?section=queues
/settings?section=preferences
/settings?section=firecrawl
/settings?section=schedulers
```

选择查询参数可以在不拆散现有设置 composable 的前提下完成迁移；如果后续需要每个模块独立加载，再升级为 `/settings/<section>` 子路由。

### Layout

```text
┌──────────────────────────────────────────────────────────────┐
│ Settings Header                         Theme     Back/Home   │
├──────────────────┬───────────────────────────────────────────┤
│ 订阅源           │ Section title / description / actions     │
│ AI 模型          ├───────────────────────────────────────────┤
│ 能力路由         │                                           │
│ Embedding        │             Section content               │
│ 队列             │                                           │
│ 阅读偏好         │                                           │
│ Firecrawl        │                                           │
│ 定时任务         │                                           │
└──────────────────┴───────────────────────────────────────────┘
```

- 桌面端使用固定宽度侧栏 + 独立内容滚动。
- 窄屏使用顶部 section selector 或抽屉导航，不压缩八个横向 tab。
- 页面容器保持稳定高度，不因 Firecrawl 等短内容切换而跳动。
- section 名称、说明和主操作固定在内容顶部；长内容仅滚动内容区。

### Information Architecture

| Section | 内容 | 迁移策略 |
|---|---|---|
| 订阅源 | 分类、搜索、单项刷新与抓取配置 | 左侧分类/订阅源列表，右侧选中订阅源编辑器；默认不渲染所有表单 |
| AI 模型 | 主模型、备用模型池 | 提供商列表 + 详情编辑；保存和连接测试属于当前详情 |
| 能力路由 | 各能力的模型降级顺序 | 独立 section，按能力折叠；拖拽/排序仅渲染展开项 |
| Embedding | 模型配置与匹配阈值 | 合并相关配置，拆成“模型”和“匹配参数”两个卡片组 |
| 队列 | Embedding 队列、标签打标队列 | 两个独立子 tab；默认显示摘要和最近记录，完整记录分页 |
| 阅读偏好 | 统计、来源/分类评分 | 统计固定顶部；列表支持搜索、排序和分页 |
| Firecrawl | 服务地址、密钥、抓取参数 | 保持单页短表单 |
| 定时任务 | 任务状态与手动执行 | 紧凑表格/列表；展示名称、技术标识、状态、最近执行和操作 |

### Feed Settings Master-detail

原订阅源配置一次渲染约 31 张卡片、124 个按钮，内容高度约 `8652px`。改为：

```text
Category/search list -> selected feed id -> FeedSettingsEditor
```

- 列表项只展示图标、名称、分类、启用状态和最近刷新状态。
- 编辑器只挂载当前选中的订阅源。
- 分类默认折叠，并提供搜索。
- 批量配置作为独立显式操作，不通过同时展开全部表单实现。
- 未选择订阅源时展示说明空状态。

### Performance and State

- section 路由状态写入 URL，刷新和浏览器返回后可恢复。
- 各 section 按需挂载，离开后不保留不必要的大型 DOM。
- 队列和偏好列表必须分页、窗口化或限制默认条数。
- 保存状态由各 section 自行管理，避免一个全局 saving 阻塞无关模块。
- 从旧 `GlobalSettingsDialog` 迁移时复用现有 API/composable，不改变后端契约。

### AppDialog Boundary

`AppDialog` 继续用于：

- 新增/编辑订阅源
- 新增/编辑分类
- 新增/编辑模型提供商
- 删除确认
- 少量参数确认

`AppDialog` 不用于：

- 多 section 设置导航
- 超过一个视口的管理列表
- 需要 URL 定位或浏览器前进/后退的工作流
- 同时包含几十个表单控件的配置中心

### TopicGraph Readability Follow-up

第二轮浏览器回归确认 TopicGraph 已可加载并支持双主题，但仍需视觉可读性收尾：

- editorial 下 `.topic-canvas-shell` 和 `.topic-note` 不得使用 overlay token 作为普通 surface。
- Canvas 节点标签需设置最小字号/对比度，并随缩放维持可读范围。
- 边宽应按权重设上下限，避免单条连线覆盖主要画布。
- 初始 camera fit 应包含合理 padding，避免节点过小或焦点被推到画布边缘。
- 两种主题均需验证焦点节点、普通节点、边和标签的对比度。

## Third Browser Regression Follow-up (2026-06-12)

### Confirmed Improvements

- `/settings` 已替代超长 Dialog，桌面端工作区高度稳定。
- `section` 查询参数可在刷新后恢复当前模块。
- 600px 窄屏下侧栏隐藏并通过 240px 抽屉导航，无横向溢出。
- 订阅源已采用列表 + 单项编辑器，不再同时挂载全部完整表单。
- Feed reader、Tags 和设置工作区未发现大面积固定亮色 surface。

### Remaining Findings and Solutions

#### AI Section Duplication

`SettingsSectionAiProviders` 仍直接挂载完整的 `AIRouterSettingsPanel`，因此 `AI 模型` 页面同时出现主模型、备用模型池、能力路由和板块匹配阈值。独立的 `能力路由` 与 `Embedding` section 又重复渲染相同内容。

解决方案：

- 将提供商管理提取为独立组件，只在 `AI 模型` section 挂载。
- `AIRouterCapabilityRoutes` 只由 `能力路由` section 挂载。
- Embedding 模型配置和板块匹配阈值只由 `Embedding` section 挂载。
- 共享数据通过 composable/store 复用，不通过复用聚合 UI 组件实现。

#### Long-list Governance

浏览器实测队列 section 内容高度约 `2276px`，阅读偏好约 `2044px`，说明分页、折叠或默认条数限制尚未真正作用于渲染结果。

解决方案：

- 队列默认只挂载摘要与最近一页记录，Embedding / 标签队列切换时卸载非活动列表。
- 阅读偏好按服务端或客户端分页展示，并让搜索、排序作用于完整数据集。
- 验收不仅检查控件是否存在，还检查首屏 DOM 行数和 scroll height。

#### Scheduler Presentation

`preference_update`、`tag_quality_score`、`log_cleanup`、`daily_report` 仍直接作为标题，状态仍显示 `idle` / `running`。

解决方案：

- 使用任务标识到中文标题的集中映射，未知任务回退到技术标识。
- 状态使用统一映射，例如“空闲”“执行中”“失败”“已停用”。
- 技术标识保留为次要文本，状态文字在 dark 下不得只使用最低强调色。

#### TopicGraph Canvas Verification

外围文本和 surface 的双主题色值已正确更新，但 WebGL Canvas 在自动截图时超时，因此不能仅凭 DOM token 判定图谱节点可读。

解决方案：

- 增加 Canvas 颜色映射的单元测试或可导出的测试渲染模式。
- 人工或 E2E 截图分别覆盖 editorial/dark 的焦点节点、普通节点、标签和高权重边。
