<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation } from '~/api/dailyReports'
import { useDagLayout } from '~/composables/useDagLayout'

const props = defineProps<{ boardId: number }>()

const { getBoardSectionTimeline } = useDailyReportsApi()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)

// --- Constants ---
const COL_W = 40
const ROW_H = 44
const PAD = 20
const NODE_R = 6

// --- Status styling ---
const statusColorMap: Record<string, string> = {
  emerging: '#34d399',
  continuing: '#60a5fa',
  split: '#fb923c',
  merge: '#c084fc',
  ending: '#9ca3af',
}

const statusLabels: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

function statusFill(status: string): string {
  return statusColorMap[status] || '#9ca3af'
}

// --- Date helpers ---

function formatDateShort(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

// --- DAG layout ---

const dagNodes = computed(() =>
  sections.value.map(s => ({ ...s, id: s.id as number | string })),
)

const dagEdges = computed(() =>
  relations.value.map(r => ({ ...r, from: r.from_id as number | string, to: r.to_id as number | string })),
)

const layout = computed(() =>
  useDagLayout(dagNodes.value, dagEdges.value, {
    direction: 'LR',
    nodeSize: [1, 1],
    gap: [1, 1],
  }),
)

// Scale layout-unit coordinates to pixel positions
function px(layoutVal: number, unit: 'x' | 'y'): number {
  return unit === 'x'
    ? layoutVal * COL_W + PAD + COL_W / 2
    : layoutVal * ROW_H + PAD + ROW_H / 2
}

// Transform an SVG path string from layout units to pixel coordinates
function scalePath(pathStr: string): string {
  return pathStr.replace(
    /(-?[\d.]+)\s*,\s*(-?[\d.]+)/g,
    (_, xs, ys) => `${px(parseFloat(xs), 'x')},${px(parseFloat(ys), 'y')}`,
  )
}

// SVG dimensions
const svgWidth = computed(() => {
  if (!layout.value) return 0
  return layout.value.width * COL_W + PAD * 2
})

const svgHeight = computed(() => {
  if (!layout.value) return 0
  return layout.value.height * ROW_H + PAD * 2
})

// Date column positions derived from layout nodes
interface DateCol {
  date: string
  label: string
  x: number
}

const dateColumns = computed<DateCol[]>(() => {
  if (!layout.value || sections.value.length === 0) return []
  // Collect (date, x) from each node, then deduplicate keeping one per date
  const seen = new Map<string, number>()
  for (const ln of layout.value.nodes) {
    const date = ln.data.period_date.slice(0, 10)
    const nodeX = px(ln.x, 'x')
    if (!seen.has(date) || seen.get(date)! > nodeX) {
      seen.set(date, nodeX)
    }
  }
  const cols: DateCol[] = []
  for (const [date, x] of seen) {
    cols.push({ date, label: formatDateShort(date), x })
  }
  cols.sort((a, b) => a.date.localeCompare(b.date))
  return cols
})

// Positioned node helper
interface PositionedSection {
  data: SectionTimelineNode
  cx: number
  cy: number
}

const positionedNodes = computed<PositionedSection[]>(() => {
  if (!layout.value) return []
  return layout.value.nodes.map(ln => ({
    data: ln.data,
    cx: px(ln.x, 'x'),
    cy: px(ln.y, 'y'),
  }))
})



function selectNode(node: SectionTimelineNode) {
  if (selectedNode.value?.id === node.id) {
    selectedNode.value = null
  } else {
    selectedNode.value = node
  }
}

// --- Data loading ---

async function loadData() {
  loading.value = true
  selectedNode.value = null
  try {
    const res = await getBoardSectionTimeline(props.boardId, days.value)
    if (res.success && res.data) {
      sections.value = res.data.sections || []
      relations.value = res.data.relations || []
    } else {
      sections.value = []
      relations.value = []
    }
  } finally {
    loading.value = false
  }
}

watch(
  () => [props.boardId, days],
  () => { loadData() },
  { immediate: true },
)
</script>

<template>
  <div class="btb-container">
    <!-- Controls -->
    <div class="btb-controls">
      <div class="btb-controls-left">
        <Icon icon="mdi:source-branch" width="15" class="text-white/50" />
        <span class="btb-controls-title">话题总览</span>
      </div>
      <div class="btb-days-toggle">
        <button
          v-for="d in [7, 14, 30, 60]"
          :key="d"
          class="btb-days-btn"
          :class="{ active: days === d }"
          @click="days = d"
        >
          {{ d }}天
        </button>
      </div>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="btb-loading">
      <div v-for="i in 3" :key="i" class="btb-skeleton" />
    </div>

    <!-- Empty -->
    <div v-else-if="sections.length === 0" class="btb-empty">
      <Icon icon="mdi:source-branch" width="28" class="text-white/15" />
      <p>暂无话题数据</p>
    </div>

    <!-- DAG timeline -->
    <div v-else-if="layout" class="btb-chart">
      <!-- Date header -->
      <div class="btb-date-header" :style="{ width: svgWidth + 'px' }">
        <span
          v-for="col in dateColumns"
          :key="col.date"
          class="btb-date-label"
          :style="{ left: col.x + 'px' }"
        >
          {{ col.label }}
        </span>
      </div>

      <!-- SVG canvas -->
      <div class="btb-svg-scroll">
        <svg
          :width="svgWidth"
          :height="svgHeight"
          class="btb-svg"
        >
          <!-- Grid lines -->
          <line
            v-for="col in dateColumns"
            :key="'grid-' + col.date"
            :x1="col.x"
            :y1="0"
            :x2="col.x"
            :y2="svgHeight"
            stroke="rgba(255,255,255,0.04)"
            stroke-width="1"
          />

          <!-- Edges -->
          <path
            v-for="(edge, ei) in layout.edges"
            :key="'edge-' + ei"
            :d="scalePath(edge.path)"
            fill="none"
            stroke="rgba(255,255,255,0.12)"
            stroke-width="1.5"
          />

          <!-- Nodes -->
          <g
            v-for="pn in positionedNodes"
            :key="'node-' + pn.data.id"
            class="btb-dag-node"
            :class="{ 'btb-dag-node--ending': pn.data.status === 'ending', 'btb-dag-node--selected': selectedNode?.id === pn.data.id }"
            @click="selectNode(pn.data)"
          >
            <title>{{ pn.data.cluster_label }} ({{ statusLabels[pn.data.status] || pn.data.status }})</title>
            <circle
              :cx="pn.cx"
              :cy="pn.cy"
              :r="selectedNode?.id === pn.data.id ? NODE_R + 2 : NODE_R"
              :fill="statusFill(pn.data.status)"
              :stroke-dasharray="pn.data.status === 'ending' ? '3 2' : undefined"
              stroke="rgba(255,255,255,0.15)"
              :stroke-width="selectedNode?.id === pn.data.id ? 2 : 1"
            />
          </g>
        </svg>
      </div>
    </div>
  </div>

  <!-- Node detail popup -->
  <Teleport to="body">
    <Transition name="btb-popup">
      <div v-if="selectedNode" class="btb-popup-overlay" @click.self="selectedNode = null">
        <div class="btb-popup">
          <div class="btb-popup-header">
            <span
              class="btb-popup-status"
              :style="{ background: statusFill(selectedNode.status) }"
            >
              {{ statusLabels[selectedNode.status] || selectedNode.status }}
            </span>
            <button class="btb-popup-close" @click="selectedNode = null">
              <Icon icon="mdi:close" width="14" />
            </button>
          </div>
          <div class="btb-popup-title">{{ selectedNode.cluster_label }}</div>
          <div class="btb-popup-meta">
            <span>{{ formatDateShort(selectedNode.period_date) }}</span>
            <span>{{ selectedNode.article_count }} 篇</span>
            <span>{{ selectedNode.thread_count }} 条线索</span>
          </div>
        </div>
      </div>
    </Transition>
  </Teleport>
</template>

<style scoped>
.btb-container {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  margin-top: 1rem;
  padding: 1rem;
  border-radius: 12px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.025);
}

/* Controls */
.btb-controls {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
}

.btb-controls-left {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.btb-controls-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.7);
}

.btb-days-toggle {
  display: flex;
  gap: 0.25rem;
}

.btb-days-btn {
  padding: 0.2rem 0.55rem;
  font-size: 0.65rem;
  border: 1px solid rgba(255, 255, 255, 0.08);
  border-radius: 4px;
  background: transparent;
  color: rgba(255, 255, 255, 0.35);
  cursor: pointer;
  transition: all 0.12s ease;
}

.btb-days-btn:hover {
  color: rgba(255, 255, 255, 0.6);
  border-color: rgba(255, 255, 255, 0.15);
}

.btb-days-btn.active {
  background: rgba(255, 255, 255, 0.1);
  color: rgba(255, 255, 255, 0.85);
  border-color: rgba(255, 255, 255, 0.2);
}

/* Loading */
.btb-loading {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}

.btb-skeleton {
  height: 36px;
  border-radius: 6px;
  background: rgba(255, 255, 255, 0.03);
  animation: btbPulse 1.5s ease-in-out infinite;
}

@keyframes btbPulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

/* Empty */
.btb-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  padding: 2.5rem 0;
  color: rgba(255, 255, 255, 0.3);
  font-size: 0.8rem;
}

/* Chart */
.btb-chart {
  display: flex;
  flex-direction: column;
}

/* Date header */
.btb-date-header {
  position: relative;
  height: 1.4rem;
  margin-bottom: 0.15rem;
  flex-shrink: 0;
}

.btb-date-label {
  position: absolute;
  transform: translateX(-50%);
  font-size: 0.55rem;
  color: rgba(255, 255, 255, 0.25);
  white-space: nowrap;
}

/* SVG scroll container */
.btb-svg-scroll {
  overflow-x: auto;
  overflow-y: hidden;
}

.btb-svg {
  display: block;
}

/* DAG nodes */
.btb-dag-node {
  cursor: pointer;
  opacity: 1;
  transition: opacity 0.12s ease;
}

.btb-dag-node circle {
  transition: r 0.1s ease, filter 0.1s ease, stroke-width 0.1s ease;
}

.btb-dag-node:hover circle {
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.25));
  r: 8;
}

.btb-dag-node--ending {
  opacity: 0.45;
}

.btb-dag-node--ending circle {
  stroke: #9ca3af;
}

.btb-dag-node--selected circle {
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.35));
  stroke: rgba(255, 255, 255, 0.5);
}

/* Popup overlay */
.btb-popup-overlay {
  position: fixed;
  inset: 0;
  z-index: 300;
  display: flex;
  align-items: center;
  justify-content: center;
  background: rgba(0, 0, 0, 0.4);
}

.btb-popup {
  background: #1e1e2e;
  border: 1px solid rgba(255, 255, 255, 0.1);
  border-radius: 10px;
  padding: 1rem 1.2rem;
  max-width: 400px;
  width: 90vw;
  box-shadow: 0 8px 32px rgba(0, 0, 0, 0.5);
}

.btb-popup-header {
  display: flex;
  align-items: center;
  justify-content: space-between;
  margin-bottom: 0.5rem;
}

.btb-popup-status {
  display: inline-block;
  font-size: 0.6rem;
  font-weight: 500;
  padding: 0.12rem 0.4rem;
  border-radius: 3px;
  color: white;
}

.btb-popup-close {
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
  transition: color 0.12s ease;
}

.btb-popup-close:hover {
  color: rgba(255, 255, 255, 0.8);
}

.btb-popup-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: rgba(255, 255, 255, 0.85);
  line-height: 1.4;
  margin-bottom: 0.3rem;
}

.btb-popup-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.65rem;
  color: rgba(255, 255, 255, 0.3);
}

/* Popup animation */
.btb-popup-enter-active {
  transition: opacity 150ms ease-out;
}
.btb-popup-enter-active .btb-popup {
  transition: opacity 200ms ease-out, transform 200ms ease-out;
}
.btb-popup-leave-active {
  transition: opacity 120ms ease-in;
}
.btb-popup-leave-active .btb-popup {
  transition: opacity 120ms ease-in, transform 120ms ease-in;
}
.btb-popup-enter-from {
  opacity: 0;
}
.btb-popup-enter-from .btb-popup {
  opacity: 0;
  transform: scale(0.95) translateY(4px);
}
.btb-popup-leave-to {
  opacity: 0;
}
.btb-popup-leave-to .btb-popup {
  opacity: 0;
  transform: scale(0.95) translateY(4px);
}
</style>
