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
