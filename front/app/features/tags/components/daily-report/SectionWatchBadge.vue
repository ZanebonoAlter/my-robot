<script setup lang="ts">
import { computed } from 'vue'

/**
 * 关注物化板块徽标（watch-materialized-topic）。
 *
 * lane_tier = watch_keyword（关键字物化，临时板块）或 watch_sentence（一句话
 * 物化，持久话题线）。纯展示：小圆点 + title 悬浮说明，色彩走主题 token，
 * 与 SectionTierBadge 的视觉语言一致（色点、无数字）。
 */
const props = defineProps<{ laneTier: string }>()

const META: Record<string, { color: string, label: string }> = {
  watch_keyword: { color: 'var(--color-tag-keyword, #b45309)', label: '关键字物化板块' },
  watch_sentence: { color: 'var(--color-accent, #2563eb)', label: '一句话物化话题' },
}

const meta = computed(() => META[props.laneTier] ?? META.watch_keyword!)
</script>

<template>
  <span
    class="section-watch-badge"
    :data-lane-tier="laneTier"
    :style="{ backgroundColor: meta.color }"
    :title="meta.label"
    role="img"
    :aria-label="meta.label"
  />
</template>

<style scoped>
.section-watch-badge {
  display: inline-block;
  flex-shrink: 0;
  width: 0.5rem;
  height: 0.5rem;
  border-radius: 2px; /* 方形角点：与圆形 tier 徽标形成家族区分 */
  vertical-align: middle;
}
</style>
