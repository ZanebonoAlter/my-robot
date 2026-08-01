# 话题锚定可观测（topic-anchor-match-observability）实现计划

> **REQUIRED SUB-SKILL:** 用 executing-plans 技能逐 Task 执行本计划。
> **配套规范:** `docs/reference/开发执行规范.md`（TDD §2、前端 §5、架构体检 §7、归档门禁 §11）。
> **OpenSpec change:** `openspec/changes/topic-anchor-match-observability/`

**Goal:** 给日报 section 补 System 2（section↔持久话题）话题锚定可观测——正文加锚定紧实度徽章（无数字）+ 探究区加话题锚定行（含距离数值），后端零改动。

**Architecture:** 纯前端 change。`topicAnchor.ts`（共享 utils，纯函数）做紧实度分档 → `SectionAnchorBadge.vue`（正文徽章，形态/色彩）+ `SectionQualityExplore.vue`（探针扩展，加顶部锚定行）消费 → `DailyReportTopicSection.vue`（唯一挂载点 line 163-171）挂载并透传 props。复用 `quality-scoring-observability` 的"正文极轻无数字、分数进探究区"展示分层哲学。

**Tech Stack:** Nuxt 4 + Vue 3 `<script setup lang="ts">` + Composition API；Vitest + happy-dom + @vue/test-utils；主题语义 token（`--color-accent` / `--color-match-*`）。

**数据契约（已就绪，`front/app/api/dailyReports.ts`）:**
- `DailyReportSection.topic_match_distance?: number`（line 111）
- `DailyReportSection.topic_match_confidence?: string`（line 112，`'anchor_hit'|'auto_new'|'unmatched'`）
- `DailyReportSection.persistent_topic?: PersistentTopicBrief`（line 113，含 `label: string`）
- `DailyReportSection.cluster_label: string`（line 100，topicLabel 降级源）

**唯一挂载点（`DailyReportTopicSection.vue:163-171`）:** 单个 `v-for section` 循环，head 为 `display:flex; gap:0.4rem`，内含 `<SectionTierBadge>` + `<h4 cluster_label>` + `<SectionQualityExplore :breakdown>`。

---

## 关键设计决策（已钉死，子进程照抄即可）

1. **分档逻辑（spec / design D2）:** confidence 主判据，distance 仅细分 anchor_hit。
   - `auto_new` → 档3（恒定，忽略 distance）
   - `anchor_hit` + `distance≤0.05` → 档0 极紧；`(0.05,0.15]` → 档1 稳锚；`(0.15,0.30]` → 档2 松锚
   - `anchor_hit` + `distance` 缺失/0/null → 档4 未锚定（防御降级，对齐 spec「缺失 topic_match_distance → 空心灰」）
   - `unmatched`/confidence 缺失/任意非 anchor_hit/auto_new → 档4
2. **token 取值:** accent = `var(--color-accent)`；灰 = `var(--color-match-weighted)`（与 SectionTierBadge 灰档一致，徽章家族统一）。
   - 档0 实心 accent 100%；档1 `color-mix(in srgb, var(--color-accent) 55%, transparent)`；档2 `... 30% ...`；档3 空心 accent ring；档4 空心 gray ring。
3. **探针行渲染条件:** `topicConfidence ∈ {anchor_hit, auto_new}` 且 `topicDistance` 为有限正数；否则整行不渲染。
4. **topicLabel 降级:** 组件内 `topicLabel || '未命名话题'`；父组件透传 `section.persistent_topic?.label || section.cluster_label`（cluster_label 降级在父层）。
5. **挂载方式:** head 内用 `<span class="drm-section-card__badges">`（gap:0.25rem）包住 tier + anchor 两徽章，与 title/explore 仍保持 head 的 0.4rem 间隔。
6. **尺寸:** 锚定徽章 0.4rem（< tier 的 0.5rem）。jsdom 不计算 scoped CSS 尺寸，单测不断言 px，靠代码审查 + build 保证（与 SectionTierBadge 测试范式一致）。
7. **无障碍:** 正文徽章无可见文字，但 `aria-label` 读中文紧实度（tier→label 内映射），保证读屏可得语义。

---

## 执行环境约束（必读）

- **typecheck / build 必须用 Windows cmd**（WSL 缺 native binding）：`cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"`。
- **lint / test:unit 可在 WSL 跑**（vitest + happy-dom 不需要 Windows）。
- **测试只跑影响范围**：`pnpm test:unit <pattern>`，不全量（除非门禁回归步）。
- **git 身份强制**：仓库 `user.name/email` 为空，commit 必须带 `-c user.name=zanebonoalter -c user.email=380207345@qq.com`。
- **TDD 铁律**：每个测试先红（看到失败原因）后绿，严禁水平切片。

---

## Task 1: 共享工具函数 `topicAnchor.ts`（TDD 红绿）

**Files:**
- Create: `front/app/utils/topicAnchor.ts`
- Test: `front/app/utils/topicAnchor.test.ts`

### Step 1.1 写失败测试 `topicAnchor.test.ts`

```ts
import { describe, expect, it } from 'vitest'
import {
  topicAnchorTier,
  topicAnchorLabel,
  TOPIC_ANCHOR_TIGHT_THRESHOLD,
  TOPIC_ANCHOR_LOOSE_THRESHOLD,
} from './topicAnchor'

describe('topicAnchorTier', () => {
  const cases: Array<[number | undefined | null, string | undefined | null, number]> = [
    // anchor_hit 三档
    [0.02, 'anchor_hit', 0], // 极紧
    [0.05, 'anchor_hit', 0], // 边界 ≤0.05 → 极紧（spec scenario）
    [0.0501, 'anchor_hit', 1], // 稳锚
    [0.10, 'anchor_hit', 1],
    [0.15, 'anchor_hit', 1], // 边界 ≤0.15 → 稳锚（spec scenario）
    [0.1501, 'anchor_hit', 2], // 松锚
    [0.27, 'anchor_hit', 2],
    [0.30, 'anchor_hit', 2],
    // auto_new 恒为档3，忽略 distance
    [0.1, 'auto_new', 3],
    [0.4, 'auto_new', 3],
    [undefined, 'auto_new', 3],
    // unmatched / 缺失 → 档4，忽略 distance
    [0.1, 'unmatched', 4],
    [undefined, undefined, 4],
    [0.1, undefined, 4],
    [null, 'mystery', 4], // 未知 confidence
    // anchor_hit 但 distance 缺失/零值 → 防御降级档4（spec：缺失 distance → 未锚定）
    [undefined, 'anchor_hit', 4],
    [null, 'anchor_hit', 4],
    [0, 'anchor_hit', 4],
  ]
  it.each(cases)('distance=%s confidence=%s → tier %s', (d, c, expected) => {
    expect(topicAnchorTier(d, c)).toBe(expected)
  })

  it('exports the dual thresholds aligned to the 2026-06-26 measurement', () => {
    expect(TOPIC_ANCHOR_TIGHT_THRESHOLD).toBe(0.05)
    expect(TOPIC_ANCHOR_LOOSE_THRESHOLD).toBe(0.15)
  })
})

describe('topicAnchorLabel', () => {
  it('maps each tier to its chinese label', () => {
    expect(topicAnchorLabel(0.02, 'anchor_hit')).toBe('极紧锚定')
    expect(topicAnchorLabel(0.10, 'anchor_hit')).toBe('稳锚定')
    expect(topicAnchorLabel(0.27, 'anchor_hit')).toBe('松锚定')
    expect(topicAnchorLabel(0.1, 'auto_new')).toBe('新话题候选')
    expect(topicAnchorLabel(0.1, 'unmatched')).toBe('未锚定')
  })

  it('falls back to 未锚定 on missing data', () => {
    expect(topicAnchorLabel(undefined, undefined)).toBe('未锚定')
    expect(topicAnchorLabel(0, 'anchor_hit')).toBe('未锚定')
  })
})
```

### Step 1.2 跑红（确认失败）

```
cd front && pnpm test:unit topicAnchor
```
Expected: FAIL（`Failed to resolve import "./topicAnchor"` 或函数未定义）。

### Step 1.3 实现 `topicAnchor.ts`

```ts
/**
 * Topic-anchor tightness helpers for the daily-report section surface (System 2:
 * section ↔ persistent topic). Encodes the match tightness as a discrete tier
 * (0..4) plus a chinese label.
 *
 * `confidence` is the primary signal; `distance` only sub-divides `anchor_hit`
 * via the dual thresholds (0.05 / 0.15, aligned to the 2026-06-06 measurement).
 * Colours are NOT resolved here — the badge component derives theme tokens.
 *
 * Pure functions, no side effects, no DOM access.
 */

export const TOPIC_ANCHOR_TIGHT_THRESHOLD = 0.05
export const TOPIC_ANCHOR_LOOSE_THRESHOLD = 0.15

/** Anchor tightness tier. Higher = looser / not anchored. */
export const ANCHOR_TIERS = {
  TIGHT: 0, // anchor_hit, distance ≤ 0.05  → 极紧锚定
  STEADY: 1, // anchor_hit, distance ≤ 0.15  → 稳锚定
  LOOSE: 2, // anchor_hit, distance ≤ 0.30  → 松锚定
  NEW: 3, // auto_new                      → 新话题候选
  UNANCHORED: 4, // unmatched / missing       → 未锚定
} as const

export type AnchorTier = (typeof ANCHOR_TIERS)[keyof typeof ANCHOR_TIERS]

/**
 * Resolve a section's topic-anchor tightness tier (0..4). Confidence is the
 * primary signal: `auto_new` is always tier 3; anything that is not
 * `anchor_hit`/`auto_new` (incl. missing) is tier 4. `distance` only
 * sub-divides `anchor_hit`; a missing/zero distance under `anchor_hit` degrades
 * defensively to tier 4 (spec: missing distance → unanchored).
 */
export function topicAnchorTier(
  distance: number | undefined | null,
  confidence: string | undefined | null,
): AnchorTier {
  if (confidence === 'auto_new') return ANCHOR_TIERS.NEW
  if (confidence !== 'anchor_hit') return ANCHOR_TIERS.UNANCHORED
  if (distance == null || !Number.isFinite(distance) || distance <= 0) {
    return ANCHOR_TIERS.UNANCHORED
  }
  if (distance <= TOPIC_ANCHOR_TIGHT_THRESHOLD) return ANCHOR_TIERS.TIGHT
  if (distance <= TOPIC_ANCHOR_LOOSE_THRESHOLD) return ANCHOR_TIERS.STEADY
  return ANCHOR_TIERS.LOOSE
}

const ANCHOR_LABELS: Record<AnchorTier, string> = {
  0: '极紧锚定',
  1: '稳锚定',
  2: '松锚定',
  3: '新话题候选',
  4: '未锚定',
}

/** Chinese tightness label matching {@link topicAnchorTier}. */
export function topicAnchorLabel(
  distance: number | undefined | null,
  confidence: string | undefined | null,
): string {
  return ANCHOR_LABELS[topicAnchorTier(distance, confidence)]
}
```

### Step 1.4 跑绿

```
cd front && pnpm test:unit topicAnchor
```
Expected: PASS（全部用例绿）。

### Step 1.5 Commit

```bash
git add front/app/utils/topicAnchor.ts front/app/utils/topicAnchor.test.ts
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "feat(daily-report): add topicAnchor tightness util (System 2)"
```

---

## Task 2: 正文锚定徽章 `SectionAnchorBadge.vue`（TDD 红绿）

**Files:**
- Create: `front/app/features/tags/components/daily-report/SectionAnchorBadge.vue`
- Test: `front/app/features/tags/components/daily-report/SectionAnchorBadge.test.ts`

### Step 2.1 写失败测试 `SectionAnchorBadge.test.ts`

```ts
import { describe, expect, it } from 'vitest'
import { mount } from '@vue/test-utils'
import SectionAnchorBadge from './SectionAnchorBadge.vue'

describe('SectionAnchorBadge', () => {
  it('tier 0 → solid accent dot, no text', () => {
    const w = mount(SectionAnchorBadge, { props: { tier: 0 } })
    const dot = w.find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--solid')
    expect(dot.attributes('data-anchor-tier')).toBe('0')
    expect(dot.attributes('style')).toContain('var(--color-accent)')
    expect(dot.attributes('style')).not.toContain('transparent')
    expect(w.text()).toBe('')
  })

  it('tier 1 → solid accent 55% mix', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 1 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--solid')
    expect(dot.attributes('data-anchor-tier')).toBe('1')
    expect(dot.attributes('style')).toContain('color-mix(in srgb, var(--color-accent) 55%, transparent)')
  })

  it('tier 2 → solid accent 30% mix', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 2 } }).find('.section-anchor-badge')
    expect(dot.attributes('data-anchor-tier')).toBe('2')
    expect(dot.attributes('style')).toContain('color-mix(in srgb, var(--color-accent) 30%, transparent)')
  })

  it('tier 3 (auto_new) → hollow accent ring', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 3 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--hollow')
    expect(dot.classes()).not.toContain('section-anchor-badge--solid')
    expect(dot.attributes('data-anchor-tier')).toBe('3')
    expect(dot.attributes('style')).toContain('transparent')
    expect(dot.attributes('style')).toContain('var(--color-accent)')
  })

  it('tier 4 (unanchored) → hollow gray ring', () => {
    const dot = mount(SectionAnchorBadge, { props: { tier: 4 } }).find('.section-anchor-badge')
    expect(dot.classes()).toContain('section-anchor-badge--hollow')
    expect(dot.attributes('data-anchor-tier')).toBe('4')
    expect(dot.attributes('style')).toContain('transparent')
    expect(dot.attributes('style')).toContain('var(--color-match-weighted)')
  })

  it('never leaks any text / numbers for any tier', () => {
    for (const tier of [0, 1, 2, 3, 4]) {
      expect(mount(SectionAnchorBadge, { props: { tier } }).text()).toBe('')
    }
  })

  it('exposes an aria-label carrying the chinese tightness word', () => {
    const w0 = mount(SectionAnchorBadge, { props: { tier: 0 } })
    expect(w0.find('.section-anchor-badge').attributes('aria-label')).toContain('极紧锚定')
    const w4 = mount(SectionAnchorBadge, { props: { tier: 4 } })
    expect(w4.find('.section-anchor-badge').attributes('aria-label')).toContain('未锚定')
  })
})
```

### Step 2.2 跑红

```
cd front && pnpm test:unit SectionAnchorBadge
```
Expected: FAIL（组件文件不存在）。

### Step 2.3 实现 `SectionAnchorBadge.vue`

```vue
<script setup lang="ts">
import { computed } from 'vue'

/**
 * Topic-anchor tightness badge for a daily-report section (System 2:
 * section ↔ persistent topic). Pure display — no distance numbers or
 * percentages (spec: 正文徽章仅形态/色彩无数字). The shape encodes weight
 * (filled vs hollow) and a single accent token's opacity encodes tightness;
 * the unanchored state falls back to a gray ring. All colours derive from
 * theme tokens so they follow the editorial/dark themes.
 *
 *   tier 0 → solid accent 100%   极紧锚定
 *   tier 1 → solid accent 55%    稳锚定
 *   tier 2 → solid accent 30%    松锚定
 *   tier 3 → hollow accent ring  新话题候选 (auto_new)
 *   tier 4 → hollow gray ring    未锚定
 */
const props = defineProps<{ tier: number }>()

const ACCENT = 'var(--color-accent)'
const GRAY = 'var(--color-match-weighted)'

const ARIA_BY_TIER = ['极紧锚定', '稳锚定', '松锚定', '新话题候选', '未锚定']

const hollow = computed(() => props.tier >= 3)
const fill = computed(() => {
  switch (props.tier) {
    case 0:
      return ACCENT
    case 1:
      return `color-mix(in srgb, ${ACCENT} 55%, transparent)`
    case 2:
      return `color-mix(in srgb, ${ACCENT} 30%, transparent)`
    default:
      return 'transparent'
  }
})
const ring = computed(() => (props.tier === 3 ? ACCENT : GRAY))
const dotStyle = computed(() =>
  hollow.value
    ? { backgroundColor: 'transparent', borderColor: ring.value }
    : { backgroundColor: fill.value },
)
const ariaLabel = computed(() => `话题锚定：${ARIA_BY_TIER[props.tier] ?? '未锚定'}`)
</script>

<template>
  <span
    class="section-anchor-badge"
    :class="hollow ? 'section-anchor-badge--hollow' : 'section-anchor-badge--solid'"
    :data-anchor-tier="tier"
    :style="dotStyle"
    role="img"
    :aria-label="ariaLabel"
  />
</template>

<style scoped>
.section-anchor-badge {
  display: inline-block;
  flex-shrink: 0;
  width: 0.4rem; /* < tier badge 0.5rem — auxiliary signal stays smaller */
  height: 0.4rem;
  border-radius: 50%;
  border: 1.5px solid transparent;
  vertical-align: middle;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}
</style>
```

### Step 2.4 跑绿

```
cd front && pnpm test:unit SectionAnchorBadge
```
Expected: PASS。

### Step 2.5 Commit

```bash
git add front/app/features/tags/components/daily-report/SectionAnchorBadge.vue front/app/features/tags/components/daily-report/SectionAnchorBadge.test.ts
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "feat(daily-report): add SectionAnchorBadge (System 2 tightness dot)"
```

---

## Task 3: 挂载锚定徽章到 `DailyReportTopicSection.vue`

**Files:**
- Modify: `front/app/features/tags/components/daily-report/DailyReportTopicSection.vue`（import 区 line 5-6 附近；挂载点 line 164-171）

### Step 3.1 新增 import

在现有 import 区（`SectionTierBadge` / `SectionQualityExplore` 旁）追加：
```ts
import SectionAnchorBadge from './SectionAnchorBadge.vue'
import { topicAnchorTier } from '~/utils/topicAnchor'
```

### Step 3.2 改造挂载点（line 164-171）

把 head 内第一个徽章包进 `.drm-section-card__badges` 并列两徽章：
```vue
<div class="drm-section-card__head">
  <span class="drm-section-card__badges">
    <SectionTierBadge :best-tier="section.best_tier" />
    <SectionAnchorBadge :tier="topicAnchorTier(section.topic_match_distance, section.topic_match_confidence)" />
  </span>
  <h4 v-if="group.sections.length > 1" class="drm-section-card__title">{{ section.cluster_label }}</h4>
  <SectionQualityExplore
    :breakdown="section.quality_breakdown"
    class="drm-section-card__explore"
  />
</div>
```

### Step 3.3 加 badges 容器 CSS

在 `<style scoped>` 的 `.drm-section-card__head` 规则（line 462-469）附近新增：
```css
.drm-section-card__badges {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  flex-shrink: 0;
}
```

### Step 3.4 验证（编译 + 视觉不溢出）

```
cd front && pnpm test:unit DailyReportTopicSection
```
Expected: 既有用例不回归（若有）；无报错。若无既有用例，跳过。
随后 §6 门禁 typecheck 会覆盖此文件。

### Step 3.5 Commit

```bash
git add front/app/features/tags/components/daily-report/DailyReportTopicSection.vue
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "feat(daily-report): mount SectionAnchorBadge beside tier badge"
```

---

## Task 4: 探针扩展 `SectionQualityExplore.vue`（TDD 红绿）

**Files:**
- Modify: `front/app/features/tags/components/daily-report/SectionQualityExplore.vue`
- Test: `front/app/features/tags/components/daily-report/SectionQualityExplore.test.ts`（扩用例）

### Step 4.1 扩写失败测试（在现有 `.test.ts` 末尾追加 describe）

```ts
describe('SectionQualityExplore — topic anchor line', () => {
  it('renders the anchor line above breakdown when anchor data present', () => {
    const w = mount(SectionQualityExplore, {
      props: {
        breakdown: [entry()],
        topicLabel: '霍尔木兹海峡',
        topicDistance: 0.03,
        topicConfidence: 'anchor_hit',
      },
    })
    const anchor = w.find('.section-quality-explore__anchor')
    expect(anchor.exists()).toBe(true)
    // 锚定行在 chip 列表之前（DOM 顺序）
    expect(w.element.querySelector('.section-quality-explore__anchor + .section-quality-explore__list')).toBeTruthy()
    expect(anchor.text()).toContain('霍尔木兹海峡')
    expect(anchor.text()).toContain('0.03')
    expect(anchor.text()).toContain('极紧锚定')
  })

  it('distinguishes 稳锚 / 松锚 labels', () => {
    const steady = mount(SectionQualityExplore, {
      props: { topicDistance: 0.1, topicConfidence: 'anchor_hit' },
    })
    expect(steady.find('.section-quality-explore__anchor').text()).toContain('稳锚定')
    const loose = mount(SectionQualityExplore, {
      props: { topicDistance: 0.27, topicConfidence: 'anchor_hit' },
    })
    expect(loose.find('.section-quality-explore__anchor').text()).toContain('松锚定')
  })

  it('shows 新话题候选 with distance for auto_new', () => {
    const w = mount(SectionQualityExplore, {
      props: { topicDistance: 0.2, topicConfidence: 'auto_new' },
    })
    const t = w.find('.section-quality-explore__anchor').text()
    expect(t).toContain('新话题候选')
    expect(t).toContain('0.20')
  })

  it('does not render the anchor line for unmatched', () => {
    const w = mount(SectionQualityExplore, {
      props: { breakdown: [entry()], topicDistance: 0.1, topicConfidence: 'unmatched' },
    })
    expect(w.find('.section-quality-explore__anchor').exists()).toBe(false)
    expect(w.text()).toContain('AI芯片') // breakdown 仍正常
  })

  it('does not render the anchor line when distance missing/zero', () => {
    const a = mount(SectionQualityExplore, { props: { topicConfidence: 'anchor_hit' } })
    expect(a.find('.section-quality-explore__anchor').exists()).toBe(false)
    const b = mount(SectionQualityExplore, { props: { topicDistance: 0, topicConfidence: 'anchor_hit' } })
    expect(b.find('.section-quality-explore__anchor').exists()).toBe(false)
  })

  it('falls back to 未命名话题 when topicLabel missing', () => {
    const w = mount(SectionQualityExplore, {
      props: { topicDistance: 0.1, topicConfidence: 'anchor_hit' },
    })
    expect(w.find('.section-quality-explore__anchor').text()).toContain('未命名话题')
  })

  it('historical section (no breakdown, no anchor) shows 无质量明细 and no anchor line', () => {
    const w = mount(SectionQualityExplore, { props: { breakdown: null } })
    expect(w.find('.section-quality-explore__anchor').exists()).toBe(false)
    expect(w.text()).toContain('无质量明细')
  })
})
```

> 注：原有 `entry()` helper 与既有用例保留不动。旧用例 `props: { breakdown: null }` 因未传 anchor props，anchorLine 为 null，仍命中"无质量明细"——不破坏。

### Step 4.2 跑红

```
cd front && pnpm test:unit SectionQualityExplore
```
Expected: FAIL（`.section-quality-explore__anchor` 不存在 / props 未声明）。

### Step 4.3 改造 `SectionQualityExplore.vue`

完整替换 `<script setup>` 与 `<template>`（style 末尾追加锚定行样式）：

```vue
<script setup lang="ts">
import { computed } from 'vue'
import { matchReasonColor, matchInfoLabel } from '~/utils/matchQuality'
import { topicAnchorLabel } from '~/utils/topicAnchor'
import type { DailyReportQualityEntry } from '~/api/dailyReports'

/**
 * Quality probe panel for a daily-report section. Renders two lineage bands:
 *   1. (top) topic-anchor line — System 2 (section ↔ persistent topic):
 *      topic name + cosine distance + chinese tightness label.
 *   2. (below) per-tag quality_breakdown chips — System 1 (tag ↔ board).
 *
 * The anchor line renders only when anchor data is present
 * (confidence ∈ {anchor_hit, auto_new} and a finite positive distance);
 * otherwise it is omitted and the probe falls back to the per-tag list or the
 * "无质量明细" placeholder. Pure display: the parent reveals it on hover/focus.
 */
const props = defineProps<{
  breakdown?: DailyReportQualityEntry[] | null
  topicLabel?: string
  topicDistance?: number
  topicConfidence?: string
}>()

const showAnchor = computed(
  () =>
    (props.topicConfidence === 'anchor_hit' || props.topicConfidence === 'auto_new')
    && typeof props.topicDistance === 'number'
    && Number.isFinite(props.topicDistance)
    && props.topicDistance > 0,
)
const anchorLine = computed(() => {
  if (!showAnchor.value) return null
  const d = props.topicDistance as number
  return {
    label: props.topicLabel || '未命名话题',
    distance: d.toFixed(2),
    tier: topicAnchorLabel(d, props.topicConfidence),
  }
})
const hasBreakdown = computed(() => !!props.breakdown && props.breakdown.length > 0)
</script>

<template>
  <div class="section-quality-explore">
    <p v-if="anchorLine" class="section-quality-explore__anchor">
      🔗 话题锚定 · <span class="section-quality-explore__anchor-label">{{ anchorLine.label }}</span>
      · 距离 {{ anchorLine.distance }} · {{ anchorLine.tier }}
    </p>
    <ul v-if="hasBreakdown" class="section-quality-explore__list">
      <li
        v-for="entry in breakdown"
        :key="entry.tag_id"
        class="section-quality-explore__chip"
        :class="{ 'section-quality-explore__chip--downgraded': entry.downgraded }"
        :data-tag-id="entry.tag_id"
        :style="{ borderColor: matchReasonColor(entry.match_reason, entry.downgraded) }"
      >
        <span class="section-quality-explore__name">{{ entry.label }}</span>
        <span class="section-quality-explore__meta">{{ matchInfoLabel(entry) }}</span>
      </li>
    </ul>
    <p v-else-if="!anchorLine" class="section-quality-explore__empty">无质量明细</p>
  </div>
</template>
```

在 `<style scoped>` 末尾追加（与现有 chip/empty 样式同风格）：
```css
.section-quality-explore__anchor {
  display: flex;
  flex-wrap: wrap;
  align-items: baseline;
  gap: 0.25rem;
  margin: 0 0 0.4rem;
  padding-bottom: 0.35rem;
  border-bottom: 1px solid var(--color-border-medium);
  color: var(--color-text-secondary);
  font-size: 0.7rem;
  line-height: 1.4;
  font-variant-numeric: tabular-nums;
}

.section-quality-explore__anchor-label {
  color: var(--color-text-primary);
  font-weight: 500;
}
```

### Step 4.4 跑绿

```
cd front && pnpm test:unit SectionQualityExplore
```
Expected: PASS（新用例 + 旧用例全绿）。

### Step 4.5 Commit

```bash
git add front/app/features/tags/components/daily-report/SectionQualityExplore.vue front/app/features/tags/components/daily-report/SectionQualityExplore.test.ts
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "feat(daily-report): add topic anchor line to SectionQualityExplore probe"
```

---

## Task 5: 透传锚定 props 到探针（`DailyReportTopicSection.vue`）

**Files:**
- Modify: `front/app/features/tags/components/daily-report/DailyReportTopicSection.vue`（Task 3 改过的 SectionQualityExplore 调用处）

### Step 5.1 给 `<SectionQualityExplore>` 补 props

```vue
<SectionQualityExplore
  :breakdown="section.quality_breakdown"
  :topic-label="section.persistent_topic?.label || section.cluster_label"
  :topic-distance="section.topic_match_distance"
  :topic-confidence="section.topic_match_confidence"
  class="drm-section-card__explore"
/>
```

### Step 5.2 跑测试不回归

```
cd front && pnpm test:unit SectionQualityExplore DailyReportTopicSection
```
Expected: PASS。

### Step 5.3 Commit

```bash
git add front/app/features/tags/components/daily-report/DailyReportTopicSection.vue
git -c user.name=zanebonoalter -c user.email=380207345@qq.com commit -m "feat(daily-report): wire topic-anchor props into SectionQualityExplore"
```

---

## Task 6: 架构体检（§7 强制）

### Step 6.1 codegraph 调用面核验

```
codegraph impact topicAnchorTier
codegraph callers SectionAnchorBadge
codegraph callers SectionQualityExplore
```
Expected:
- `topicAnchorTier` 命中 `DailyReportTopicSection.vue`（徽章挂载）。
- `topicAnchorLabel` 命中 `SectionQualityExplore.vue`（探针行）+ `SectionAnchorBadge`（aria-label 同源——注：aria 用组件内映射，非 topicAnchorLabel，故 topicAnchorLabel 仅探针引用）。
- `SectionAnchorBadge` 调用点 = `DailyReportTopicSection.vue`，无遗漏。
- `SectionQualityExplore` 调用点 = `DailyReportTopicSection.vue`（唯一），props 扩展未漏调用点。
- 若 impact 返回 HIGH/CRITICAL → 暂停上报（执行规范 §7.1）。

### Step 6.2 架构合理性人工确认

- 新 utils 落在共享层 `app/utils/`，与 `matchQuality.ts` 同层 ✓
- 两徽章并列无循环依赖（徽章不反向依赖挂载点）✓
- 探针 props 全可选，旧调用点（无此 change 时）不破坏 ✓
- 无新增 lint 警告（由 §Task 7 lint 覆盖）

---

## Task 7: 文档更新（§9 / §12）

### Step 7.1 日报业务链路文档

检索 `docs/reference/flow/` 下日报相关文档（含 section 可视化/质量段落者），补充 System 2 锚定可观测语义：
- 正文锚定紧实度点（无数字，形态五档）+ 探究区话题锚定行（话题名/距离/中文标签）。
- 与 System 1（标签↔板块，quality-scoring）并列说明两套独立维度。
- 若无现成日报可视化 flow 文档，在最近的日报 flow 文件加一节，或记一笔到 `docs/reference/architecture/map.md`（Step 7.2）。

### Step 7.2 索引地图

更新 `docs/reference/architecture/map.md`：日报域新增「话题锚定可观测」入口，指向 `SectionAnchorBadge.vue` / `SectionQualityExplore.vue` / `utils/topicAnchor.ts`（若已有质量可视化条目，补并列条目）。

### Step 7.3 前端规范（仅在确有新约定时）

检查 `docs/reference/standard/frontend/` 是否需补「双徽章并列 / 共享 utils 落位」约定。若现有规范已覆盖（共享 utils 在 `app/utils/`、徽章并列属组件自由），则不动并在此记录"已评估，无需新增"。

---

## Task 8: 质量门禁验收（§5.1 / §11.2 归档门禁）

> 这些命令由「验收子进程」执行（实现子进程完成后）。逐条跑，记录实际输出。

### Step 8.1 lint（WSL 可跑）
```
cd front && pnpm lint
```
Expected: 零 error。

### Step 8.2 typecheck（必须 Windows cmd）
```
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm exec nuxi typecheck"
```
Expected: 零 error。

### Step 8.3 build（必须 Windows cmd）
```
cmd.exe /C "cd /d D:\project\Syntopica\front && pnpm build"
```
Expected: 构建成功。

### Step 8.4 全量单测回归
```
cd front && pnpm test:unit
```
Expected: 全绿（无既有用例回归）。

### Step 8.5 grep 一致性（证明 System 2 字段不再零展示）
```
grep -rn "topic_match_distance\|topic_match_confidence" front/app --include=*.vue --include=*.ts | grep -v "\.test\.ts"
```
Expected: 命中 `DailyReportTopicSection.vue`（徽章 tier 计算 + 探针 props 透传）、`dailyReports.ts`（类型声明）。证明字段已消费。

### Step 8.6 双徽章并列挂载校验
```
grep -rn "SectionTierBadge\|SectionAnchorBadge" front/app --include=*.vue
```
Expected: 两徽章并列挂载于 `DailyReportTopicSection.vue`。

### Step 8.7 L1 规范验收
```
bash scripts/check-standards.sh
```
Expected: 零失败。

---

## 完成定义（DoD）

- [x] Task 1-6 全部 commit，topicAnchor 工具函数 + SectionAnchorBadge + SectionQualityExplore 扩展 + 挂载/透传就绪。
- [x] Task 7 文档更新落地（或记录"已评估无需改"）。
- [x] Task 8 七条门禁命令实测零失败。
- [x] tasks.md 对应 checkbox 全部勾选（1.1-7.7）。
- [x] codegraph 无 HIGH/CRITICAL 风险遗留。
→ 满足归档门禁 §11，可进入 `openspec archive`（归档前再跑一遍 Task 8 验证节）。
