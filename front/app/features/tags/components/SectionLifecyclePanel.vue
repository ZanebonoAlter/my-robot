<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionLifecycleNode, type SectionRelation } from '~/api/dailyReports'
import { useDagLayout } from '~/composables/useDagLayout'

const props = defineProps<{
  sectionId: number
}>()

const emit = defineEmits<{
  close: []
  navigate: [node: SectionLifecycleNode]
}>()

const { getSectionLifecycle } = useDailyReportsApi()

const sections = ref<SectionLifecycleNode[]>([])
const relations = ref<SectionRelation[]>([])
const loading = ref(false)
const error = ref(false)

// Layout constants
const NODE_W = 280
const NODE_H = 64
const GAP_X = 24
const GAP_Y = 16
const PADDING = 20

const statusColorMap: Record<string, string> = {
  emerging: '#16a34a',
  continuing: '#2563eb',
  split: '#ea580c',
  merge: '#9333ea',
  ending: '#9ca3af',
}

const statusLabel: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

const weekDays = ['日', '一', '二', '三', '四', '五', '六']

function formatDate(dateStr: string): string {
  const d = new Date(dateStr)
  const month = d.getMonth() + 1
  const day = d.getDate()
  const weekDay = weekDays[d.getDay()]
  return `${month}月${day}日 周${weekDay}`
}

function truncateLabel(label: string, max = 24): string {
  return label.length > max ? label.slice(0, max) + '...' : label
}

const currentSection = computed(() =>
  sections.value.find(s => s.id === props.sectionId),
)

const dagInputNodes = computed(() =>
  sections.value.map(s => ({ ...s, id: s.id as number | string })),
)

const dagInputEdges = computed(() =>
  relations.value.map(r => ({ ...r, from: r.from_id as number | string, to: r.to_id as number | string })),
)

const layout = computed(() => {
  if (dagInputNodes.value.length === 0) return null
  return useDagLayout(dagInputNodes.value, dagInputEdges.value, { direction: 'TB', nodeSize: [1, 1], gap: [1, 1] })
})

const layoutNodeMap = computed(() => {
  const map = new Map<number, { x: number; y: number }>()
  if (layout.value) {
    for (const n of layout.value.nodes) {
      map.set(n.data.id as number, { x: n.x, y: n.y })
    }
  }
  return map
})

const svgWidth = computed(() => {
  if (!layout.value) return 600
  const maxX = Math.max(0, ...layout.value.nodes.map(n => n.x))
  return Math.max(600, (maxX + 1) * (NODE_W + GAP_X) + PADDING)
})
const svgHeight = computed(() => {
  if (!layout.value) return 0
  const maxY = Math.max(0, ...layout.value.nodes.map(n => n.y))
  return (maxY + 1) * (NODE_H + GAP_Y) + PADDING * 2
})

function scaleX(x: number): number {
  return x * (NODE_W + GAP_X) + PADDING
}

function scaleY(y: number): number {
  return y * (NODE_H + GAP_Y) + PADDING
}

function edgePathFromLayout(fromId: string | number, toId: string | number): string {
  const from = layoutNodeMap.value.get(Number(fromId))
  const to = layoutNodeMap.value.get(Number(toId))
  if (!from || !to) return ''
  const x1 = scaleX(from.x) + NODE_W / 2
  const y1 = scaleY(from.y) + NODE_H
  const x2 = scaleX(to.x) + NODE_W / 2
  const y2 = scaleY(to.y)
  const midY = (y1 + y2) / 2
  return `M${x1},${y1} C${x1},${midY} ${x2},${midY} ${x2},${y2}`
}

let fetchId = 0

async function fetchLifecycle() {
  loading.value = true
  error.value = false
  const currentFetch = ++fetchId
  try {
    const res = await getSectionLifecycle(props.sectionId)
    if (currentFetch !== fetchId) return
    if (res.success && res.data) {
      sections.value = res.data.sections || []
      relations.value = res.data.relations || []
    } else {
      error.value = true
    }
  } catch {
    if (currentFetch !== fetchId) return
    error.value = true
  } finally {
    if (currentFetch === fetchId) {
      loading.value = false
    }
  }
}

watch(
  () => props.sectionId,
  () => { fetchLifecycle() },
  { immediate: true },
)

function handleNodeClick(node: SectionLifecycleNode & { id: number | string }) {
  emit('navigate', node as SectionLifecycleNode)
}
</script>

<template>
  <div class="slp-inline">
    <!-- Header bar -->
    <div class="slp-header">
      <button type="button" class="slp-back" @click="$emit('close')">
        <Icon icon="mdi:arrow-left" width="16" />
        <span>返回日报</span>
      </button>
      <span v-if="currentSection" class="slp-header-title">{{ currentSection.cluster_label }}</span>
      <span class="slp-header-label">话题生命周期</span>
    </div>

    <!-- Loading skeleton -->
    <div v-if="loading" class="slp-body">
      <div v-for="i in 3" :key="i" class="slp-skeleton-node">
        <div class="slp-skeleton-dot" />
        <div class="slp-skeleton-lines">
          <div class="slp-skeleton-line slp-skeleton-short" />
          <div class="slp-skeleton-line slp-skeleton-long" />
        </div>
      </div>
    </div>

    <!-- Error -->
    <div v-else-if="error" class="slp-body">
      <div class="slp-error">加载失败</div>
    </div>

    <!-- Empty state -->
    <div v-else-if="sections.length === 0" class="slp-body">
      <div class="slp-empty">独立话题，无演化记录</div>
    </div>

    <!-- DAG view -->
    <div v-else class="slp-body slp-dag-scroll">
      <svg
        :width="svgWidth"
        :height="svgHeight"
        :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
        xmlns="http://www.w3.org/2000/svg"
        class="slp-dag-svg"
      >
        <!-- Edges -->
        <g class="slp-edges">
          <path
            v-for="(edge, i) in (layout?.edges ?? [])"
            :key="'e' + i"
            :d="edgePathFromLayout(edge.from, edge.to)"
            fill="none"
            stroke="rgba(80,60,30,0.18)"
            stroke-width="1.5"
          />
        </g>

        <!-- Nodes -->
        <g
          v-for="posNode in (layout?.nodes ?? [])"
          :key="'n' + posNode.data.id"
          class="slp-dag-node"
          :class="{
            'slp-dag-node-current': posNode.data.id === sectionId,
            'slp-dag-node-ending': posNode.data.status === 'ending',
          }"
          :transform="`translate(${scaleX(posNode.x)}, ${scaleY(posNode.y)})`"
          @click="handleNodeClick(posNode.data)"
        >
          <!-- Background rect -->
          <rect
            :width="NODE_W"
            :height="NODE_H"
            rx="6"
            :fill="posNode.data.id === sectionId ? 'rgba(80,60,20,0.08)' : 'rgba(255,255,255,0.5)'"
            :stroke="posNode.data.status === 'ending' ? 'rgba(120,120,120,0.3)' : 'rgba(80,60,30,0.15)'"
            :stroke-dasharray="posNode.data.status === 'ending' ? '4,3' : 'none'"
            stroke-width="1"
          />

          <!-- Status dot -->
          <circle
            cx="14"
            :cy="NODE_H / 2"
            r="4"
            :fill="statusColorMap[posNode.data.status] || '#9ca3af'"
          />

          <!-- Date -->
          <text
            x="26"
            y="16"
            fill="rgba(80,60,30,0.5)"
            font-size="9"
            font-family="system-ui, sans-serif"
          >{{ formatDate(posNode.data.period_date) }}</text>

          <!-- Cluster label -->
          <text
            x="26"
            y="32"
            fill="rgba(40,30,10,0.85)"
            font-size="12"
            font-weight="500"
            font-family="system-ui, sans-serif"
          >{{ truncateLabel(posNode.data.cluster_label) }}</text>

          <!-- Meta -->
          <text
            x="26"
            y="50"
            fill="rgba(80,60,30,0.45)"
            font-size="9"
            font-family="system-ui, sans-serif"
          >{{ posNode.data.article_count }} 篇 · {{ posNode.data.thread_count }} 条线索</text>

          <!-- Status badge -->
          <rect
            :x="NODE_W - 46"
            y="23"
            width="38"
            height="18"
            rx="4"
            :fill="statusColorMap[posNode.data.status] + '18'"
            :stroke="statusColorMap[posNode.data.status] + '40'"
            stroke-width="0.5"
          />
          <text
            :x="NODE_W - 27"
            y="35"
            :fill="statusColorMap[posNode.data.status]"
            font-size="8"
            font-weight="500"
            font-family="system-ui, sans-serif"
            text-anchor="middle"
          >{{ statusLabel[posNode.data.status] || posNode.data.status }}</text>
        </g>
      </svg>
    </div>
  </div>
</template>

<style scoped>
.slp-inline {
  display: flex;
  flex-direction: column;
  height: 100%;
  min-height: 0;
}

/* Header */
.slp-header {
  display: flex;
  align-items: center;
  gap: 0.75rem;
  padding-bottom: 0.8rem;
  border-bottom: 1px solid rgba(80, 60, 30, 0.12);
  margin-bottom: 0.8rem;
  flex-shrink: 0;
}

.slp-back {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.6rem;
  border: 1px solid rgba(80, 60, 30, 0.15);
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.4);
  color: rgba(60, 40, 10, 0.7);
  font-size: 0.75rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.slp-back:hover {
  background: rgba(255, 255, 255, 0.6);
  color: rgba(40, 20, 0, 0.9);
}

.slp-header-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: rgba(40, 30, 10, 0.85);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slp-header-label {
  font-size: 0.7rem;
  color: rgba(80, 60, 30, 0.4);
  flex-shrink: 0;
}

/* Body */
.slp-body {
  flex: 1;
  min-height: 0;
}

.slp-dag-scroll {
  overflow: auto;
}

.slp-dag-svg {
  display: block;
}

/* DAG node interaction */
.slp-dag-node {
  cursor: pointer;
  transition: opacity 0.12s ease;
}

.slp-dag-node:hover rect {
  fill: rgba(255, 255, 255, 0.7);
}

.slp-dag-node-ending {
  opacity: 0.5;
}

/* Empty */
.slp-empty {
  text-align: center;
  padding: 3rem 0;
  font-size: 0.8rem;
  color: rgba(80, 60, 30, 0.35);
}

/* Error */
.slp-error {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2rem 0;
  font-size: 0.8rem;
  color: rgba(80, 60, 30, 0.4);
}

/* Loading skeleton */
.slp-skeleton-node {
  display: flex;
  gap: 0.75rem;
  padding: 0.6rem 0;
}

.slp-skeleton-dot {
  flex-shrink: 0;
  width: 10px;
  height: 10px;
  border-radius: 50%;
  background: rgba(80, 60, 30, 0.08);
  margin-top: 4px;
}

.slp-skeleton-lines {
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  flex: 1;
}

.slp-skeleton-line {
  height: 10px;
  border-radius: 4px;
  background: rgba(80, 60, 30, 0.06);
  animation: slpPulse 1.5s ease-in-out infinite;
}

.slp-skeleton-short { width: 40%; }
.slp-skeleton-long { width: 85%; }

@keyframes slpPulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}
</style>
