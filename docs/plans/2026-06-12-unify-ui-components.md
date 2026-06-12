# Unify UI Components 实施计划

> **REQUIRED SUB-SKILL:** Use the executing-plans skill to implement this plan task-by-task.

**Goal:** 建立三层 CSS Token 架构 + 双主题系统 + 统一 UI 原子组件，消除前端四套对话框、三种表单控件的 UI 分裂

**Architecture:** Layer 1 Primitive tokens (`--raw-*`) → Layer 2 Semantic tokens (跟主题走) → Layer 3 Component tokens (可选)。通过 `data-theme` 属性切换 editorial/dark 双主题。新建 AppDialog/AppButton/AppToggle/AppInput/AppSectionHeader 五个原子组件替代现有碎片化实现。

**Tech Stack:** Vue 3 + Nuxt 4 + CSS Custom Properties + Tailwind v4 (仅布局)

---

## 执行策略

### 依赖关系分析

```
Phase 1 (Token基础) ─────┐
                         ├──→ Phase 2 (UI原子组件) ──→ Phase 3-11 (迁移)
Phase 1 (useTheme) ──────┘
```

Phase 1 和 Phase 2 是所有后续 phase 的前置依赖。Phase 2 的 5 个组件相互独立，可以并行开发。Phase 3-11 之间大部分独立（不同文件），可以并行，但 Phase 4 (Dialog迁移) 需要在 Phase 2 (AppDialog) 之后，Phase 5-7 需要在对应组件之后。

### 并行分组

**Group A (基础):** Phase 1 → Phase 2（串行，Phase 2 内部可并行）
**Group B (迁移):** Phase 3, 4, 5, 6, 7, 8（并行，依赖各自的 Phase 2 组件）
**Group C (专题页):** Phase 9, 10, 11（并行）
**Group D (清理):** Phase 12（最后执行）

---

## Phase 1: Token 基础架构 (对应 Tasks 1-7)

**Files:**
- Modify: `front/app/assets/css/main.css` (添加 Layer 1 + Layer 2 tokens，迁移通用样式)
- Create: `front/app/composables/useTheme.ts`
- Modify: `front/app/app.vue` (html 默认 data-theme)

### Step 1: 添加 Layer 1 Primitive Tokens

在 `main.css` 的 `@theme {}` 块**之前**（因为 Primitive tokens 不是 Tailwind theme tokens，是纯 CSS 变量），添加 `:root` 块定义所有 `--raw-*` 变量。

**位置:** `main.css` 文件顶部，在 `@import "tailwindcss"` 之后、`@theme {}` 之前

```css
:root {
  /* === Layer 1: Primitive Tokens === */
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

### Step 2: 添加 Layer 2 Semantic Tokens (双主题)

在 `:root` 块之后、`@theme {}` 之前，添加两个主题选择器块：

```css
/* === Layer 2: Semantic Tokens (Editorial) === */
[data-theme="editorial"] {
  --color-bg-base:     var(--raw-stone-50);
  --color-bg-elevated: var(--raw-stone-100);
  --color-bg-sunken:   var(--raw-stone-200);
  --color-bg-overlay:  rgba(26, 26, 26, 0.4);
  --color-bg-hover:    rgba(255, 255, 255, 0.85);
  --color-bg-active:   rgba(255, 255, 255, 0.6);

  --color-text-primary:   #1a1a1a;
  --color-text-secondary: #5a5a5a;
  --color-text-muted:     #8a8a8a;
  --color-text-inverted:  #faf7f2;

  --color-border-subtle: rgba(26, 26, 26, 0.08);
  --color-border-medium: rgba(26, 26, 26, 0.15);
  --color-border-strong: rgba(26, 26, 26, 0.25);

  --color-accent:       var(--raw-red-500);
  --color-accent-hover: var(--raw-red-600);
  --color-accent-subtle: rgba(217, 74, 74, 0.08);

  --color-secondary: var(--raw-amber-500);
  --color-tertiary:  var(--raw-blue-400);

  --shadow-subtle: 0 1px 3px rgba(26, 26, 26, 0.06);
  --shadow-medium: 0 2px 8px rgba(26, 26, 26, 0.08);
  --shadow-strong: 0 4px 16px rgba(26, 26, 26, 0.12);
  --shadow-print:  0 1px 0 rgba(26, 26, 26, 0.1), 0 2px 4px rgba(26, 26, 26, 0.06);

  --color-success: var(--raw-success);
  --color-warning: var(--raw-warning);
  --color-error:   var(--raw-error);
  --color-info:    var(--raw-info);

  --color-input-bg:    rgba(255, 255, 255, 0.6);
  --color-input-border: rgba(26, 26, 26, 0.12);
  --color-input-focus:  var(--raw-red-500);

  --color-dialog-bg:      rgba(255, 255, 255, 0.95);
  --color-dialog-header:  transparent;
  --color-dialog-divider: rgba(26, 26, 26, 0.06);
}

/* === Layer 2: Semantic Tokens (Dark) === */
[data-theme="dark"] {
  --color-bg-base:     #080c12;
  --color-bg-elevated: rgba(17, 27, 38, 0.98);
  --color-bg-sunken:   #0e161d;
  --color-bg-overlay:  rgba(8, 12, 18, 0.75);
  --color-bg-hover:    rgba(255, 255, 255, 0.06);
  --color-bg-active:   rgba(255, 255, 255, 0.08);

  --color-text-primary:   rgba(255, 255, 255, 0.9);
  --color-text-secondary: rgba(255, 255, 255, 0.6);
  --color-text-muted:     rgba(255, 255, 255, 0.35);
  --color-text-inverted:  #1a1a1a;

  --color-border-subtle: rgba(255, 255, 255, 0.06);
  --color-border-medium: rgba(255, 255, 255, 0.10);
  --color-border-strong: rgba(255, 255, 255, 0.18);

  --color-accent:       rgba(240, 138, 75, 0.85);
  --color-accent-hover: rgba(240, 138, 75, 1);
  --color-accent-subtle: rgba(240, 138, 75, 0.08);

  --color-secondary: rgba(99, 179, 237, 0.85);
  --color-tertiary:  rgba(63, 124, 255, 0.85);

  --shadow-subtle: 0 1px 3px rgba(0, 0, 0, 0.3);
  --shadow-medium: 0 2px 8px rgba(0, 0, 0, 0.4);
  --shadow-strong: 0 4px 16px rgba(0, 0, 0, 0.5);
  --shadow-print:  0 1px 0 rgba(0, 0, 0, 0.2), 0 2px 4px rgba(0, 0, 0, 0.3);

  --color-success: var(--raw-success);
  --color-warning: var(--raw-warning);
  --color-error:   var(--raw-error);
  --color-info:    var(--raw-info);

  --color-input-bg:    rgba(255, 255, 255, 0.04);
  --color-input-border: rgba(255, 255, 255, 0.10);
  --color-input-focus:  rgba(240, 138, 75, 0.85);

  --color-dialog-bg:      rgba(17, 27, 38, 0.98);
  --color-dialog-header:  rgba(255, 255, 255, 0.03);
  --color-dialog-divider: rgba(255, 255, 255, 0.06);
}
```

### Step 3: 删除孤儿 token

在 `@theme {}` 块内，删除以下从未被引用的变量：
- `--color-bg-primary`, `--color-bg-secondary`, `--color-bg-tertiary`, `--color-bg-card`, `--color-bg-hover`
- `--text-primary`, `--text-secondary`, `--text-muted`, `--text-inverted`

> **注意:** `--color-border-*`、`--shadow-*`、`--color-success`/`warning`/`error`/`info` 保留在 `@theme {}` 中（Tailwind 需要这些），但值会被 `[data-theme]` 选择器覆盖。

### Step 4: 迁移 main.css 通用样式中的旧 token

在 `main.css` 的通用样式部分（`.btn-primary`, `.input`, `.paper-card` 等），将所有 `var(--color-ink-*)` 替换为语义 token：
- `--color-ink-black` → `var(--color-text-primary)`
- `--color-ink-dark` → `var(--color-text-secondary)`
- `--color-ink-medium` → `var(--color-text-secondary)`
- `--color-ink-light` → `var(--color-text-muted)`
- `--color-ink-muted` → `var(--color-text-muted)`
- `--color-ink-500` → `var(--color-accent)` (按钮背景)
- `--color-ink-600` → `var(--color-accent-hover)`
- `--color-ink-300` → `var(--color-border-medium)`
- `--color-ink-400` → `var(--color-input-focus)`
- `--color-paper-ivory` → `var(--color-bg-base)`
- `--color-paper-cream` → `var(--color-bg-elevated)`
- `--color-paper-warm` → `var(--color-bg-sunken)`
- `--color-paper-sand` → `var(--color-bg-sunken)`
- `--color-print-red-500` → `var(--color-accent)`

### Step 5: 创建 useTheme composable

**Create:** `front/app/composables/useTheme.ts`

```typescript
import { ref, readonly, computed, onMounted } from 'vue'
import { useHead } from '#imports'

type Theme = 'editorial' | 'dark'

const STORAGE_KEY = 'syntopica-theme'

// 模块级单例
const currentTheme = ref<Theme>('editorial')

// 客户端初始化标记
let initialized = false

function initTheme() {
  if (initialized) return
  initialized = true
  if (import.meta.client) {
    const stored = localStorage.getItem(STORAGE_KEY) as Theme | null
    if (stored === 'editorial' || stored === 'dark') {
      currentTheme.value = stored
    }
  }
}

export function useTheme() {
  initTheme()

  function setTheme(theme: Theme) {
    currentTheme.value = theme
    if (import.meta.client) {
      localStorage.setItem(STORAGE_KEY, theme)
      document.documentElement.setAttribute('data-theme', theme)
    }
    useHead({ htmlAttrs: { 'data-theme': theme } })
  }

  function toggleTheme() {
    setTheme(currentTheme.value === 'editorial' ? 'dark' : 'editorial')
  }

  // SSR: 通过 useHead 设置 html 属性避免 FOUC
  useHead({ htmlAttrs: { 'data-theme': currentTheme.value } })

  return {
    theme: readonly(currentTheme),
    setTheme,
    toggleTheme,
    isDark: computed(() => currentTheme.value === 'dark'),
  }
}

export type { Theme }
```

### Step 6: app.vue 默认主题

在 `front/app/app.vue` 中，`<html>` 不需要手动设置（useTheme 的 useHead 已经处理）。确保在 app.vue 的 setup 中调用 `useTheme()` 初始化。

实际上更好的做法：在 `nuxt.config.ts` 里已有 `ssr: false`，所以 `useHead` 方式足够。app.vue 只需在合适位置调用一次 `useTheme()`。

### 验证

```bash
# 检查 CSS 变量是否正确定义
cd front && grep -c "raw-slate" app/assets/css/main.css
# 应该有 10 个 (--raw-slate-50 到 900)

# 检查双主题是否定义
grep -c "data-theme" app/assets/css/main.css
# 应该 >= 2

# 检查 useTheme composable 是否存在
ls app/composables/useTheme.ts
```

---

## Phase 2: UI 原子组件 (对应 Tasks 15, 22, 27, 34, 37)

5 个组件相互独立，**可以并行开发**。

### 2A: AppDialog.vue (Task 15)

**Create:** `front/app/components/ui/AppDialog.vue`

实现要点：
- Props: `modelValue: boolean`, `title?: string`, `width?: string` (默认 '480px'), `closeOnOverlay?: boolean` (默认 true), `closeOnEscape?: boolean` (默认 true), `showClose?: boolean` (默认 true)
- Emits: `update:modelValue`
- Slots: `header`, `default`, `footer`
- 使用 `<Teleport to="body">` + `<Transition name="dialog">`
- 所有色值使用 Layer 2 语义 token
- CSS 参考 design.md 中的 AppDialog 样式

### 2B: AppButton.vue (Task 22)

**Create:** `front/app/components/ui/AppButton.vue`

实现要点：
- Props: `variant?: 'primary' | 'secondary' | 'ghost' | 'danger'`, `size?: 'sm' | 'md' | 'lg'`, `disabled?: boolean`, `loading?: boolean`, `type?: 'button' | 'submit' | 'reset'`
- 使用 Layer 3 组件 token (`--btn-bg`, `--btn-text` 等) 映射到 Layer 2
- loading 状态用 CSS spinner

### 2C: AppToggle.vue (Task 27)

**Create:** `front/app/components/ui/AppToggle.vue`

实现要点：
- Props: `modelValue: boolean`, `disabled?: boolean`, `label?: string`
- Emits: `update:modelValue`
- track/thumb 动画，使用语义 token
- `.is-active` 类控制激活状态

### 2D: AppInput.vue (Task 34)

**Create:** `front/app/components/ui/AppInput.vue`

实现要点：
- Props: `modelValue: string | number`, `type?: string`, `placeholder?: string`, `disabled?: boolean`, `error?: string`, `step?: string | number`, `min?: string | number`, `max?: string | number`
- Emits: `update:modelValue`
- 支持 type="number" + step/min/max 透传
- error 状态显示错误文字

### 2E: AppSectionHeader.vue (Task 37)

**Create:** `front/app/components/ui/AppSectionHeader.vue`

实现要点：
- Props: `title: string`, `description?: string`, `icon?: Component`
- 可选 icon box + 标题 + 描述
- 使用语义 token

### 验证

```bash
# 检查所有组件是否创建
ls front/app/components/ui/
# 应该有: AppDialog.vue AppButton.vue AppToggle.vue AppInput.vue AppSectionHeader.vue
```

---

## Phase 3: 全局样式迁移 (对应 Tasks 8-14)

**所有文件迁移旧 token → 语义 token**

每个文件的任务相同：将 `var(--color-ink-*)`、`var(--color-paper-*)`、`var(--color-print-red-*)` 替换为对应的语义 token。

**迁移映射（参考 design.md Migration Map）：**

| 旧 Token | 新 Token |
|-----------|----------|
| `var(--color-ink-black)` | `var(--color-text-primary)` |
| `var(--color-ink-dark)` | `var(--color-text-secondary)` |
| `var(--color-ink-medium)` | `var(--color-text-secondary)` |
| `var(--color-ink-light)` | `var(--color-text-muted)` |
| `var(--color-ink-muted)` | `var(--color-text-muted)` |
| `var(--color-ink-50)` ~ `var(--color-ink-900)` | 具体分析上下文 |
| `var(--color-paper-ivory)` | `var(--color-bg-base)` |
| `var(--color-paper-cream)` | `var(--color-bg-elevated)` |
| `var(--color-paper-warm)` | `var(--color-bg-sunken)` |
| `var(--color-paper-sand)` | `var(--color-bg-sunken)` |
| `var(--color-print-red-*)` | `var(--color-accent)` 或 `var(--raw-red-*)` |

**涉及文件（可以并行处理）：**
1. `front/app/components/article/ArticleContent.css` (18 处)
2. `front/app/components/layout/ArticleListPanel.css` (15 处)
3. `front/app/components/layout/AppHeader.css` (9 处)
4. `front/app/components/layout/AppSidebar.css` (8 处)
5. `front/app/components/article/ArticleCard.css`
6. `front/app/components/article/ArticleTagList.vue` (在 features/articles 下)
7. `front/app/features/shell/components/AppSidebarView.vue`

### 验证

```bash
# 确认这些文件不再有旧 token 引用
cd front && grep -r "var(--color-ink-" app/components/article/ app/components/layout/
grep -r "var(--color-paper-" app/components/article/ app/components/layout/
# 应该返回空
```

---

## Phase 4: Dialog 迁移 (对应 Tasks 16-21)

### 4A: Pattern A — Editorial Dialogs (Task 16)

**涉及组件（5个）：**
- `components/dialog/AddFeedDialog.vue`
- `components/dialog/EditFeedDialog.vue`
- `components/dialog/AddCategoryDialog.vue`
- `components/dialog/EditCategoryDialog.vue`
- `components/dialog/ImportOpmlDialog.vue`

**迁移步骤（每个组件）：**
1. 将现有 overlay + dialog div 替换为 `<AppDialog v-model="show" title="...">`
2. 注意：Pattern A 现在用 `emit('close')` + 父组件 `v-if` 控制，需要改为 `v-model`
3. 内部 `.input` → `<AppInput>`, `.btn-primary` → `<AppButton>`, checkbox → `<AppToggle>`
4. 删除组件内 scoped 的 overlay/dialog CSS

### 4B: Pattern B — Tags Dark Dialogs (Task 17)

**涉及组件（3个）：**
- `features/tags/components/AddSemanticBoardDialog.vue`
- `features/tags/components/BoardEditDialog.vue`
- `features/tags/components/MatchingConfigDialog.vue`

**迁移步骤同上，额外：**
- 删除所有 `mc-*`/`dialog-*`/`board-edit-*` CSS 前缀的 scoped 样式
- `.mc-input` → `<AppInput>`, `.mc-btn` → `<AppButton>`, `.dialog-checkbox` → `<AppToggle>`

### 4C: Pattern C — GlobalSettingsDialog (Task 18)

**文件:** `components/dialog/GlobalSettingsDialog.vue`

**迁移步骤：**
1. 替换外壳为 `<AppDialog v-model="show" title="设置" width="900px">`
2. 去掉底部"完成"按钮（各面板自行管理保存）
3. Tab 导航保持不变

### 4D: Pattern D — ArticlePreviewModal (Task 19)

**文件:** `features/tags/components/ArticlePreviewModal.vue`

**迁移步骤：**
1. 替换为 `<AppDialog v-model="visible" width="90vw">`
2. 保留 iframe 渲染逻辑

### 4E: TopicGraph Dialogs (Task 20)

**涉及组件（2个）：**
- `features/topic-graph/components/TopicGraphMergeDialog.vue`
- `features/tags/components/NarrativeGenerateDialog.vue`

### 4F: 删除 Dialog.css (Task 21)

```bash
rm front/app/components/dialog/Dialog.css
```

### 验证

```bash
# 确认 Dialog.css 已删除
! test -f front/app/components/dialog/Dialog.css && echo "OK"

# 确认所有对话框都引用 AppDialog
grep -r "AppDialog" front/app/components/dialog/ front/app/features/tags/components/ front/app/features/topic-graph/components/
```

---

## Phase 5: 按钮迁移 (对应 Tasks 23-26)

**涉及文件：**
- `ArticleListPanelView.vue` — `.btn-primary-sm` / `.btn-secondary-sm` → `<AppButton size="sm">`
- `ArticleContentPreviewPanel.vue` — `.btn.btn-primary` → `<AppButton>`
- `ArticleIframeView.vue` — `.btn.btn-primary` → `<AppButton>`
- `app.vue` — 刷新按钮 `.btn-primary` → `<AppButton>`

### 验证

```bash
# 确认不再有旧按钮 class 引用
cd front && grep -r "btn-primary\|btn-secondary\|mc-btn\|gray-btn" app/ --include="*.vue"
# 只应在 main.css 的 class 定义中出现，不在 template 中
```

---

## Phase 6: Toggle 迁移 (对应 Tasks 28-33)

**涉及组件：**
- `EditFeedDialog.vue` — 3 个 checkbox → `<AppToggle>`
- `FirecrawlConfigPanel.vue` — checkbox → `<AppToggle>`
- `AIRouterSettingsPanel.vue` — 1 个 checkbox → `<AppToggle>`
- `AIRouterBackupProviders.vue` — 3 个 checkbox → `<AppToggle>`
- `AddSemanticBoardDialog.vue` — `.dialog-checkbox` → `<AppToggle>`
- `BoardTimelinePanel.vue` — checkbox → `<AppToggle>`
- `AuxiliaryLabelPicker.vue` — checkbox → `<AppToggle>`

### 验证

```bash
# 确认 checkbox 已迁移（允许非 toggle 场景保留原生 checkbox）
grep -r 'type="checkbox"' front/app/app/ --include="*.vue" | grep -v AppToggle
# 应该只剩非 toggle 场景（如全选框等）
```

---

## Phase 7: Input 迁移 (对应 Tasks 35-36)

**涉及组件：**
- 所有使用 `.input` class 的对话框 → `<AppInput>`
- 所有使用 `.mc-input` 的组件 → `<AppInput>`
- `MatchingConfigDialog.vue` — 15 个 `.mc-input` number inputs → `<AppInput type="number">`

### 验证

```bash
grep -r 'class="input\b\|class="mc-input"' front/app/ --include="*.vue"
# 应该返回空
```

---

## Phase 8: SectionHeader 迁移 (对应 Task 38)

**涉及：各面板的自建 header → `<AppSectionHeader>`**

逐个检查各 panel/dialog 的 header 区域，替换为 `<AppSectionHeader>`。

---

## Phase 9: TagsPage 硬编码迁移 (对应 Tasks 39-43)

**涉及文件：**
- `features/tags/components/TagsPage.vue` — 页面级 `useTheme('dark')` + 硬编码色值迁移
- `features/tags/components/BoardListSidebar.vue`
- `features/tags/components/BoardCompositionPanel.vue`
- `features/tags/components/AuxiliaryLabelPool.vue`
- `features/tags/components/AuxiliaryLabelPicker.vue`
- 所有 `features/tags/components/` 下 dialog/panel

**核心替换映射：**
```
#080c12                       → var(--color-bg-base)
rgba(17, 27, 38, 0.98)        → var(--color-bg-elevated)
rgba(240, 138, 75, 0.85)      → var(--color-accent)
rgba(240, 138, 75, 0.08)      → var(--color-accent-subtle)
rgba(255, 255, 255, 0.9)      → var(--color-text-primary)
rgba(255, 255, 255, 0.6)      → var(--color-text-secondary)
rgba(255, 255, 255, 0.35)     → var(--color-text-muted)
rgba(255, 255, 255, 0.06)     → var(--color-border-subtle)
rgba(255, 255, 255, 0.10)     → var(--color-border-medium)
```

**TagsPage.vue 添加主题锁定：**
```typescript
const { setTheme, theme } = useTheme()
let saved: Theme

onMounted(() => {
  saved = theme.value
  setTheme('dark')
})
onUnmounted(() => {
  setTheme(saved)
})
```

---

## Phase 10: TopicGraph 硬编码迁移 (对应 Tasks 44-47)

**涉及文件：**
- `features/topic-graph/components/TopicGraphPage.vue` — 页面级 `useTheme('dark')` + 硬编码迁移
- `features/topic-graph/components/TopicGraphSidebar.vue` — CSS 变量迁移
- `features/topic-graph/components/TopicGraphCanvas.client.vue` — 运行时色值通过 CSS 变量桥接
- 其他 `features/topic-graph/components/` 下组件

**TopicGraphCanvas 特殊处理：**
Canvas 运行时色值无法直接用 CSS 变量，需要通过 `getComputedStyle` 读取 CSS 变量值：
```typescript
const style = getComputedStyle(document.documentElement)
const accentColor = style.getPropertyValue('--color-accent').trim()
// 传给 Canvas 渲染
```

---

## Phase 11: AI 面板迁移 (对应 Tasks 48-50)

**涉及文件：**
- `features/ai/components/AIRouterSettingsPanel.vue` — checkbox → AppToggle
- `features/ai/components/AIRouterBackupProviders.vue` — 3 个 checkbox → AppToggle
- `features/ai/components/EmbeddingConfigPanel.vue` — 如有硬编码色值 → 语义 token
- `features/ai/components/EmbeddingQueuePanel.vue` — 同上
- `features/ai/components/AIRouterCapabilityRoutes.vue` — 同上

---

## Phase 12: 旧 Token 清理 (对应 Tasks 51-56)

**最后执行，确保所有引用已迁移完毕。**

### Step 1: 删除旧 token 定义

在 `main.css` 的 `@theme {}` 块中删除：
- `--color-ink-50` ~ `--color-ink-900` (10个)
- `--color-ink-black`, `--color-ink-dark`, `--color-ink-medium`, `--color-ink-light`, `--color-ink-muted` (5个)
- `--color-paper-ivory`, `--color-paper-cream`, `--color-paper-warm`, `--color-paper-sand` (4个)
- `--color-print-red-50` ~ `--color-print-red-900` (10个)
- `--color-accent-teal`, `--color-accent-amber`, `--color-accent-indigo`, `--color-accent-forest` (4个)

### Step 2: 全项目零引用确认

```bash
cd front
grep -r "var(--color-ink-" app/ --include="*.vue" --include="*.css"
grep -r "var(--color-paper-" app/ --include="*.vue" --include="*.css"
grep -r "var(--color-print-red-" app/ --include="*.vue" --include="*.css"
grep -r "var(--text-primary)" app/ --include="*.vue" --include="*.css"
grep -r "var(--text-secondary)" app/ --include="*.vue" --include="*.css"
grep -r "var(--color-bg-primary)" app/ --include="*.vue" --include="*.css"
# 全部应该返回空
```

### Step 3: 更新 tasks.md 中的 checkbox

将所有 `- [ ]` 改为 `- [x]`。

---

## 全局验证

```bash
# 1. Lint 检查
cd front && pnpm lint

# 2. Typecheck（必须通过 Windows cmd）
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"

# 3. Build（必须通过 Windows cmd）
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```

---

## 风险与注意事项

1. **`@theme {}` 与 `:root` / `[data-theme]` 的关系：** Tailwind v4 的 `@theme {}` 生成 `:root` 下的 CSS 变量。我们需要确保 Layer 1 和 Layer 2 的定义不会与 `@theme {}` 冲突。策略：Layer 1 在 `@theme {}` 之前定义（纯 `:root`），Layer 2 在 `@theme {}` 之后定义（用 `[data-theme]` 选择器覆盖）。

2. **SSR 兼容：** 项目已设置 `ssr: false`（SPA 模式），所以 `useHead` 方式足够，无需复杂的水合处理。

3. **Canvas 色值：** TopicGraphCanvas 使用 JavaScript 运行时色值，需要通过 `getComputedStyle` 读取 CSS 变量。

4. **旧 token 兼容期：** Phase 3-11 迁移过程中，旧 token 定义仍然存在（在 `@theme {}` 中），所以不会出现中间态断裂。Phase 12 最后统一清理。

5. **GlobalSettingsDialog 去掉"完成"按钮：** 这是一个行为变更，需要确认各面板的保存逻辑已独立。
