## Capability

三层 CSS 变量 token 架构和双主题切换系统。Primitive token 定义原始色值，Semantic token 跟主题走，Component token 由复杂组件按需定义。`useTheme()` composable 管理主题状态。

## API

### useTheme()

```typescript
interface UseThemeReturn {
  /** 当前主题名 */
  theme: Readonly<Ref<Theme>>
  /** 设置主题 */
  setTheme(theme: Theme): void
  /** 切换 editorial ↔ dark */
  toggleTheme(): void
  /** 当前是否 dark */
  isDark: Readonly<Ref<boolean>>
}

type Theme = 'editorial' | 'dark'

function useTheme(): UseThemeReturn
```

**行为**：
- 首次调用时从 `localStorage.getItem('syntopica-theme')` 读取，默认 `'editorial'`
- `setTheme()` 更新 `localStorage` + `useHead({ htmlAttrs: { 'data-theme': theme } })` (SSR-safe，避免 FOUC)
- 模块级单例：所有组件共享同一个 `currentTheme` ref

### 页面级主题锁定

TagsPage 和 TopicGraphPage 进入时自动切换到 dark，离开时恢复：

```typescript
// 由各页面组件自行调用，不在 useTheme 内部自动处理
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

## Token Catalog

### Layer 1: Primitive (`--raw-*`)

只在 Layer 2 定义中引用，不在组件中直接使用（图表等非 UI 场景除外）。

| Family | Tones | 用途 |
|--------|-------|------|
| `--raw-slate-*` | 50~900 | 蓝灰色板，替代旧 `--color-ink-*` |
| `--raw-stone-*` | 50~900 | 暖灰色板，替代旧 `--color-paper-*` |
| `--raw-red-*` | 50~900 | 红色板，替代旧 `--color-print-red-*` |
| `--raw-amber-*` | 400, 500 | 琥珀色，dark 主题强调 |
| `--raw-blue-*` | 400, 500 | 蓝色，辅助 |
| `--raw-teal-*` | 500, 600 | 青色，成功色系 |
| `--raw-indigo-*` | 500 | 靛蓝，信息色系 |
| `--raw-success` | 单值 | 语义绿 |
| `--raw-warning` | 单值 | 语义黄 |
| `--raw-error` | 单值 | 语义红 |
| `--raw-info` | 单值 | 语义蓝 |

### Layer 2: Semantic (`--color-*`, `--shadow-*`)

每个主题定义一套完整映射。组件直接引用这一层。

| Token | Editorial | Dark | 用途 |
|-------|-----------|------|------|
| `--color-bg-base` | `var(--raw-stone-50)` `#faf7f2` | `#080c12` | 页面底色 |
| `--color-bg-elevated` | `var(--raw-stone-100)` `#f5f0e6` | `rgba(17,27,38,0.98)` | 卡片/面板 |
| `--color-bg-sunken` | `var(--raw-stone-200)` `#e8dfd1` | `#0e161d` | 嵌入区域 |
| `--color-bg-overlay` | `rgba(26,26,26,0.4)` | `rgba(8,12,18,0.75)` | 遮罩 |
| `--color-bg-hover` | `rgba(255,255,255,0.85)` | `rgba(255,255,255,0.06)` | 悬停态 |
| `--color-bg-active` | `rgba(255,255,255,0.6)` | `rgba(255,255,255,0.08)` | 激活态 |
| `--color-text-primary` | `#1a1a1a` | `rgba(255,255,255,0.9)` | 主文字 |
| `--color-text-secondary` | `#5a5a5a` | `rgba(255,255,255,0.6)` | 次要文字 |
| `--color-text-muted` | `#8a8a8a` | `rgba(255,255,255,0.35)` | 辅助文字 |
| `--color-text-inverted` | `#faf7f2` | `#1a1a1a` | 反色文字 |
| `--color-border-subtle` | `rgba(26,26,26,0.08)` | `rgba(255,255,255,0.06)` | 微弱边框 |
| `--color-border-medium` | `rgba(26,26,26,0.15)` | `rgba(255,255,255,0.10)` | 中等边框 |
| `--color-border-strong` | `rgba(26,26,26,0.25)` | `rgba(255,255,255,0.18)` | 强边框 |
| `--color-accent` | `var(--raw-red-500)` | `rgba(240,138,75,0.85)` | 强调色 |
| `--color-accent-hover` | `var(--raw-red-600)` | `rgba(240,138,75,1)` | 强调悬停 |
| `--color-accent-subtle` | `rgba(217,74,74,0.08)` | `rgba(240,138,75,0.08)` | 强调淡底 |
| `--color-secondary` | `var(--raw-amber-500)` | `rgba(99,179,237,0.85)` | 辅助色 |
| `--color-tertiary` | `var(--raw-blue-400)` | `rgba(63,124,255,0.85)` | 第三色 |
| `--color-success` | `var(--raw-success)` | `var(--raw-success)` | 成功 |
| `--color-warning` | `var(--raw-warning)` | `var(--raw-warning)` | 警告 |
| `--color-error` | `var(--raw-error)` | `var(--raw-error)` | 错误 |
| `--color-info` | `var(--raw-info)` | `var(--raw-info)` | 信息 |
| `--color-input-bg` | `rgba(255,255,255,0.6)` | `rgba(255,255,255,0.04)` | 输入框底 |
| `--color-input-border` | `rgba(26,26,26,0.12)` | `rgba(255,255,255,0.10)` | 输入框边框 |
| `--color-input-focus` | `var(--raw-red-500)` | `rgba(240,138,75,0.85)` | 输入框聚焦 |
| `--color-dialog-bg` | `rgba(255,255,255,0.95)` | `rgba(17,27,38,0.98)` | 对话框底 |
| `--color-dialog-header` | `transparent` | `rgba(255,255,255,0.03)` | 对话框头 |
| `--color-dialog-divider` | `rgba(26,26,26,0.06)` | `rgba(255,255,255,0.06)` | 对话框分割线 |
| `--shadow-subtle` | `0 1px 3px rgba(26,26,26,0.06)` | `0 1px 3px rgba(0,0,0,0.3)` | 微阴影 |
| `--shadow-medium` | `0 2px 8px rgba(26,26,26,0.08)` | `0 2px 8px rgba(0,0,0,0.4)` | 中阴影 |
| `--shadow-strong` | `0 4px 16px rgba(26,26,26,0.12)` | `0 4px 16px rgba(0,0,0,0.5)` | 强阴影 |
| `--shadow-print` | 印刷阴影 | 暗色印刷阴影 | 特殊 |

### Layer 3: Component (`--dialog-*`, `--button-*`, `--input-*`, `--toggle-*`)

由组件内部定义，映射到 Layer 2。详见各组件 spec。

## Migration

### 删除的旧 Token

```
--color-bg-primary      (孤儿，直接删除)
--color-bg-secondary    (孤儿，直接删除)
--color-bg-tertiary     (孤儿，直接删除)
--color-bg-card         (孤儿，直接删除)
--color-bg-hover        (旧定义删除，由 Layer 2 同名语义 token 替代)
--text-primary          (孤儿，直接删除)
--text-secondary        (孤儿，直接删除)
--text-muted            (孤儿，直接删除)
--text-inverted         (孤儿，直接删除)
```

### 替换的旧 Token

```
--color-ink-50~900       → --raw-slate-50~900 (或语义 token)
--color-ink-black        → --color-text-primary
--color-ink-dark         → --color-text-secondary
--color-ink-medium       → --color-text-secondary
--color-ink-light        → --color-text-muted
--color-ink-muted        → --color-text-muted
--color-paper-ivory      → --color-bg-base
--color-paper-cream      → --color-bg-elevated
--color-paper-warm       → --color-bg-sunken
--color-paper-sand       → --color-bg-sunken
--color-print-red-50~900 → --raw-red-50~900 (或语义 token)
--color-accent-teal      → --raw-teal-500
--color-accent-amber     → --raw-amber-500
--color-accent-indigo    → --raw-indigo-500
--color-accent-forest    → --raw-teal-600
```

### 保留的 Token (同名，值由主题决定)

```
--color-border-subtle
--color-border-medium
--color-border-strong
--shadow-subtle
--shadow-medium
--shadow-strong
--shadow-print
--color-success
--color-warning
--color-error
--color-info
```

## Constraints

- `data-theme` 必须挂在 `<html>` 元素上，确保全局级联
- 不使用 Tailwind `dark:` 前缀
- Primitive token 只在 Layer 2 的 `[data-theme]` 块内引用，不在组件 `.vue`/`.css` 中直接使用
- `useTheme()` 是模块级单例，不是 composable 实例级状态
- 主题切换不需要 CSS transition（直接切换）
- `localStorage` key: `'syntopica-theme'`
- Nuxt SSR 兼容：使用 `useHead({ htmlAttrs })` 设置 html 属性，避免 FOUC
