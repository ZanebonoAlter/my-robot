<script setup lang="ts">
/**
 * 卡片迷你柱图（design D4 / spec「话题卡片 mini-lifeline」）。
 *
 * 替代旧 MiniLifeline.vue 的 CSS 色阶条：
 *  - 无轴无网格迷你柱图，柱高=section_count，空日 0 值占位（日期轴连续）
 *  - hover tooltip 显示「M/D：N 节」
 *  - 主题切换 → readPalette 重建 option
 *
 * emerging 卡片不渲染本组件（节奏信息由总览气泡图承载）。
 */
import { onMounted, watch } from 'vue'
import { useTheme } from '~/composables/useTheme'
import type { LifelinePoint } from '~/api/semanticBoards'
import { useEcharts } from './useEcharts'
import { buildMiniBarOption, readPalette } from './chart-options'

const props = defineProps<{
  points: LifelinePoint[]
}>()

const { theme } = useTheme()
const { elRef, setOption } = useEcharts()

function render() {
  setOption(buildMiniBarOption(props.points, readPalette()))
}

onMounted(render)
watch([() => props.points, theme], render)
</script>

<template>
  <div ref="elRef" class="mlc-chart" :title="`近 ${points.length} 日命中节奏`" />
</template>

<style scoped>
.mlc-chart {
  width: 100%;
  height: 22px;
}
</style>
