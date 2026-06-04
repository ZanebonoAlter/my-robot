<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { useDailyReportsApi, type SectionLifecycleNode, type SectionRelation } from '~/api/dailyReports'
import { useDagLayout } from '~/composables/useDagLayout'

const props = defineProps<{
  sectionId: number
  visible: boolean
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
const NODE_W = 260
const NODE_H = 56
const GAP_X = 20
const GAP_Y = 14
const PADDING = 16

const statusColorMap: Record<string, string> = {
  emerging: '#34d399',
  continuing: '#60a5fa',
  split: '#fb923c',
  merge: '#c084fc',
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

function truncateLabel(label: string, max = 20): string {
  return label.length > max ? label.slice(0, max) + '...' : label
}

// Compute DAG layout
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

// Map layout nodes by id for quick lookup
const layoutNodeMap = computed(() => {
  const map = new Map<number, { x: number; y: number }>()
  if (layout.value) {
    for (const n of layout.value.nodes) {
      map.set(n.data.id as number, { x: n.x, y: n.y })
    }
  }
  return map
})

// SVG canvas dimensions
const svgWidth = computed(() => {
  if (!layout.value) return 320
  const maxX = Math.max(0, ...layout.value.nodes.map(n => n.x))
  return Math.max(320, (maxX + 1) * (NODE_W + GAP_X) + PADDING)
})
const svgHeight = computed(() => {
  if (!layout.value) return 0
  const maxY = Math.max(0, ...layout.value.nodes.map(n => n.y))
  return (maxY + 1) * (NODE_H + GAP_Y) + PADDING * 2
})

// Coordinate scaling
function scaleX(x: number): number {
  return x * (NODE_W + GAP_X) + PADDING
}

function scaleY(y: number): number {
  return y * (NODE_H + GAP_Y) + PADDING
}

// Edge path generator — cubic bezier, vertical-first, from node positions
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
  () => [props.visible, props.sectionId] as const,
  ([vis]) => {
    if (vis) {
      fetchLifecycle()
    }
  },
  { immediate: true },
)

function handleNodeClick(node: SectionLifecycleNode & { id: number | string }) {
  emit('navigate', node as SectionLifecycleNode)
}
</script>

<template>
  <Transition name="slp-panel">
    <div v-if="visible" class="slp-panel">
      <div class="slp-header">
        <span class="slp-title">话题生命周期</span>
        <button type="button" class="slp-close" @click="$emit('close')">&#10005;</button>
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
        <div class="slp-empty">独立话题</div>
      </div>

      <!-- DAG view -->
      <div v-else class="slp-body slp-dag-scroll">
        <svg
          :width="svgWidth"
          :height="svgHeight"
          :viewBox="`0 0 ${svgWidth} ${svgHeight}`"
          :style="{ minWidth: svgWidth + 'px' }"
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
              stroke="rgba(255,255,255,0.15)"
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
              :fill="posNode.data.id === sectionId ? 'rgba(255,255,255,0.08)' : 'rgba(255,255,255,0.03)'"
              :stroke="posNode.data.status === 'ending' ? 'rgba(156,163,175,0.3)' : 'rgba(255,255,255,0.06)'"
              :stroke-dasharray="posNode.data.status === 'ending' ? '4,3' : 'none'"
              stroke-width="1"
            />

            <!-- Status dot -->
            <circle
              cx="14"
              :cy="NODE_H / 2"
              r="4"
              :fill="statusColorMap[posNode.data.status] || '#9ca3af'"
              :filter="posNode.data.id === sectionId ? 'url(#slp-glow)' : undefined"
            />

            <!-- Date -->
            <text
              x="26"
              y="15"
              fill="rgba(255,255,255,0.3)"
              font-size="9"
              font-family="system-ui, sans-serif"
            >{{ formatDate(posNode.data.period_date) }}</text>

            <!-- Cluster label -->
            <text
              x="26"
              y="30"
              fill="rgba(255,255,255,0.75)"
              font-size="12"
              font-weight="500"
              font-family="system-ui, sans-serif"
            >{{ truncateLabel(posNode.data.cluster_label) }}</text>

            <!-- Meta -->
            <text
              x="26"
              y="46"
              fill="rgba(255,255,255,0.3)"
              font-size="9"
              font-family="system-ui, sans-serif"
            >{{ posNode.data.article_count }} 篇 · {{ posNode.data.thread_count }} 条线索</text>

            <!-- Status badge pill -->
            <rect
              :x="NODE_W - 44"
              y="20"
              width="36"
              height="18"
              rx="4"
              :fill="statusColorMap[posNode.data.status] + '22'"
              :stroke="statusColorMap[posNode.data.status] + '44'"
              stroke-width="0.5"
            />
            <text
              :x="NODE_W - 26"
              y="32"
              :fill="statusColorMap[posNode.data.status]"
              font-size="8"
              font-weight="500"
              font-family="system-ui, sans-serif"
              text-anchor="middle"
            >{{ statusLabel[posNode.data.status] || posNode.data.status }}</text>
          </g>

          <!-- Glow filter definition -->
          <defs>
            <filter id="slp-glow" x="-50%" y="-50%" width="200%" height="200%">
              <feGaussianBlur stdDeviation="3" result="blur" />
              <feMerge>
                <feMergeNode in="blur" />
                <feMergeNode in="SourceGraphic" />
              </feMerge>
            </filter>
          </defs>
        </svg>
      </div>
    </div>
  </Transition>
</template>

<style scoped>
.slp-panel {
  position: fixed;
  top: 0;
  right: 0;
  width: 320px;
  height: 100vh;
  background: #111827;
  border-left: 1px solid rgba(255, 255, 255, 0.08);
  display: flex;
  flex-direction: column;
  z-index: 250;
  box-shadow: -8px 0 32px rgba(0, 0, 0, 0.4);
}

.slp-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.7rem 1rem;
  border-bottom: 1px solid rgba(255, 255, 255, 0.08);
  flex-shrink: 0;
}

.slp-title {
  font-size: 0.85rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.8);
}

.slp-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 24px;
  height: 24px;
  border: none;
  border-radius: 4px;
  background: transparent;
  color: rgba(255, 255, 255, 0.4);
  cursor: pointer;
  font-size: 0.85rem;
  transition: all 0.12s ease;
}

.slp-close:hover {
  background: rgba(255, 255, 255, 0.08);
  color: rgba(255, 255, 255, 0.7);
}

.slp-body {
  flex: 1;
  overflow-y: auto;
  padding: 1rem;
}

.slp-dag-scroll {
  padding: 0;
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
  fill: rgba(255, 255, 255, 0.06);
}

.slp-dag-node-ending {
  opacity: 0.5;
}

/* Empty */
.slp-empty {
  text-align: center;
  padding: 1.5rem 0;
  font-size: 0.75rem;
  color: rgba(255, 255, 255, 0.3);
}

/* Error state */
.slp-error {
  display: flex;
  align-items: center;
  justify-content: center;
  padding: 2rem 0;
  font-size: 0.8rem;
  color: rgba(255, 255, 255, 0.3);
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
  background: rgba(255, 255, 255, 0.08);
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
  background: rgba(255, 255, 255, 0.06);
  animation: slpPulse 1.5s ease-in-out infinite;
}

.slp-skeleton-short { width: 40%; }
.slp-skeleton-long { width: 85%; }

@keyframes slpPulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

/* Slide transition */
.slp-panel-enter-active {
  transition: transform 250ms ease-out, opacity 250ms ease-out;
}

.slp-panel-leave-active {
  transition: transform 200ms ease-in, opacity 200ms ease-in;
}

.slp-panel-enter-from {
  transform: translateX(100%);
  opacity: 0;
}

.slp-panel-leave-to {
  transform: translateX(100%);
  opacity: 0;
}
</style>
