<script setup lang="ts">
import { computed } from 'vue'

/**
 * Colour-only tier badge for a daily-report section.
 *
 * Pure display — no scores, percentages or match-method words (spec:
 * "正文徽章仅色彩无数字"). The shape encodes weight (filled vs hollow) and the
 * colour encodes the best match tier, both derived from theme tokens so they
 * follow the editorial/dark themes:
 *
 *   best_tier 0 → filled, direct-hit (green)
 *   best_tier 1 → filled, hit-rate (blue)
 *   best_tier 2 → filled, max-sim (orange)
 *   best_tier 3 → hollow,  weighted (gray)
 *
 * best_tier is an independent frozen field, so historical sections
 * (quality_breakdown = null) still render their badge.
 */
const props = defineProps<{ bestTier: number }>()

const TOKEN_BY_TIER: Record<number, string> = {
  0: 'var(--color-match-direct-hit)',
  1: 'var(--color-match-hit-rate)',
  2: 'var(--color-match-max-sim)',
  3: 'var(--color-match-weighted)',
}

const token = computed(() => TOKEN_BY_TIER[props.bestTier] ?? TOKEN_BY_TIER[3])
const hollow = computed(() => props.bestTier >= 3)
const dotStyle = computed(() => (
  hollow.value
    ? { backgroundColor: 'transparent', borderColor: token.value }
    : { backgroundColor: token.value }
))
</script>

<template>
  <span
    class="section-tier-badge"
    :class="hollow ? 'section-tier-badge--hollow' : 'section-tier-badge--solid'"
    :data-tier="bestTier"
    :style="dotStyle"
    role="img"
    :aria-label="`质量等级 ${bestTier}`"
  />
</template>

<style scoped>
.section-tier-badge {
  display: inline-block;
  flex-shrink: 0;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 50%;
  border: 1.5px solid transparent;
  vertical-align: middle;
  transition: background-color 0.2s ease, border-color 0.2s ease;
}
</style>
