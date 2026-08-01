<script setup lang="ts">
/**
 * 活力顶栏（design / spec「活力顶栏」）。
 *
 * 展示近 N 日 article_count / section_count / active_topic_count + 每日 section 数面积图。
 * feed_active（活跃信息源数）MVP 可为 null —— 此时该子项隐藏，不阻断整栏。
 *
 * 面积图由 ECharts 渲染（替代旧手算 SVG polyline），主题跟随。
 */
import { computed, onMounted, watch } from 'vue'
import { useTheme } from '~/composables/useTheme'
import type { Vitality } from '~/api/semanticBoards'
import { useEcharts } from './useEcharts'
import { buildVitalityOption, readPalette } from './chart-options'

const props = defineProps<{
  vitality: Vitality
}>()

const { theme } = useTheme()
const { elRef, setOption } = useEcharts()

/** trend 至少 2 点才有面积图意义；空/单点 → 不渲染图。 */
const hasTrend = computed(
  () => Array.isArray(props.vitality.trend) && props.vitality.trend.length >= 2,
)

/** 从今天往前推 n 个 MM-DD 日期标签（trend 末位 = 今天，与「近 N 日」语义一致）。 */
function dateLabels(n: number): string[] {
  const labels: string[] = []
  const today = new Date()
  for (let i = n - 1; i >= 0; i--) {
    const d = new Date(today.getFullYear(), today.getMonth(), today.getDate() - i)
    labels.push(
      `${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`,
    )
  }
  return labels
}

function render() {
  if (hasTrend.value) {
    setOption(
      buildVitalityOption(props.vitality.trend, dateLabels(props.vitality.trend.length), readPalette()),
    )
  }
}

onMounted(render)
watch([() => props.vitality.trend, theme], render)
</script>

<template>
  <div class="vb">
    <div class="vb-meta">
      <span class="vb-days">近 {{ vitality.days }} 天</span>
    </div>
    <div class="vb-stats">
      <span class="vb-stat"><b>{{ vitality.article_count }}</b> 文章</span>
      <span class="vb-sep">·</span>
      <span class="vb-stat"><b>{{ vitality.section_count }}</b> 节</span>
      <span class="vb-sep">·</span>
      <span class="vb-stat"><b>{{ vitality.active_topic_count }}</b> 活跃话题</span>
      <!-- feed_active MVP 可空：缺它不阻断整栏，只隐藏该子项 -->
      <template v-if="vitality.feed_active !== null && vitality.feed_active !== undefined">
        <span class="vb-sep">·</span>
        <span class="vb-stat"><b>{{ vitality.feed_active }}</b> 活跃源</span>
      </template>
    </div>
    <!-- 容器始终渲染：挂载时 trend<2 也先 init，父级刷新数据后 watch 触发 render 即出图 -->
    <div ref="elRef" class="vb-chart" aria-hidden="true" />
  </div>
</template>

<style scoped>
.vb {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  flex-wrap: wrap;
  padding: 0.5rem 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
}

.vb-meta {
  flex: 0 0 auto;
}

.vb-days {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--color-text-muted);
}

.vb-stats {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  flex-wrap: wrap;
}

.vb-stat {
  font-size: 0.72rem;
  color: var(--color-text-secondary);
}

.vb-stat b {
  color: var(--color-text-primary);
  font-weight: 700;
}

.vb-sep {
  color: var(--color-text-muted);
  font-size: 0.68rem;
}

.vb-chart {
  flex: 0 0 auto;
  margin-left: auto;
  width: 150px;
  height: 30px;
}
</style>
