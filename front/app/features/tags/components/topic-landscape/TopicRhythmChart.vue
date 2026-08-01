<script setup lang="ts">
/**
 * 话题节奏总览气泡图（design D3 / spec「话题节奏总览气泡图」）。
 *
 * 一张图聚合全部话题的近 N 日命中节奏：
 *  - x=日期（category）、y=话题（category inverse，stance 分组序 + hit_count DESC）
 *  - 气泡大小∝section_count（sqrt clamp 4~18）、颜色=stance，分 5 series + legend
 *  - y 轴 dataZoom（滚轮 + slider），archived series 默认 legend unselected
 *  - 点击气泡 → emit selectTopic（沿用卡片点击链路切「话题总览」聚焦）
 *  - 主题切换 → readPalette 重建 option
 *
 * 挂载位置：VitalityBar 之下、StanceCardWall 之上（TopicLandscapePanel 组装）。
 */
import { computed, onMounted, watch } from 'vue'
import { useTheme } from '~/composables/useTheme'
import type { TopicLandscapeTopic } from '~/api/semanticBoards'
import { useEcharts } from './useEcharts'
import { buildRhythmOption, getRhythmDates, readPalette } from './chart-options'

const props = defineProps<{
  topics: TopicLandscapeTopic[]
}>()

const emit = defineEmits<{
  selectTopic: [topicId: number]
}>()

const { theme } = useTheme()
const { elRef, setOption, on } = useEcharts()

const dates = computed(() => getRhythmDates(props.topics))

function render() {
  setOption(buildRhythmOption(props.topics, dates.value, readPalette()))
}

// useEcharts 的 onMounted 先注册（init），本组件 onMounted 后注册 → setOption 命中已 init 实例
onMounted(render)
watch([() => props.topics, theme], render)

// 点击气泡：value = [xIndex, yIndex, section_count, topicId]
on('click', (params) => {
  const v = params.value
  if (Array.isArray(v) && typeof v[3] === 'number') {
    emit('selectTopic', v[3])
  }
})
</script>

<template>
  <div class="trc">
    <div class="trc-caption">
      <span class="trc-caption-title">话题命中节奏总览</span>
      <span class="trc-caption-hint">滚轮/拖动缩放 · 点击气泡聚焦话题</span>
    </div>
    <div ref="elRef" class="trc-chart" />
  </div>
</template>

<style scoped>
.trc {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  padding: 0.55rem 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
}

.trc-caption {
  display: flex;
  align-items: baseline;
  gap: 0.4rem;
}

.trc-caption-title {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.trc-caption-hint {
  font-size: 0.62rem;
  color: var(--color-text-muted);
}

.trc-chart {
  width: 100%;
  height: 520px;
}
</style>
