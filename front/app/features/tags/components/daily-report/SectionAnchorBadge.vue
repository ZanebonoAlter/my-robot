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
