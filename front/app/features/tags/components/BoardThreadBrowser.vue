<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation } from '~/api/dailyReports'

const props = defineProps<{ boardId: number }>()

const { getBoardSectionTimeline } = useDailyReportsApi()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)
const hoveredId = ref<number | null>(null)

// --- Constants ---
const COL_W = 120
const ROW_H = 48
const PAD = 28
const NODE_R = 7
const LABEL_MAX = 8

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

function truncateLabel(label: string): string {
  return label.length > LABEL_MAX ? label.slice(0, LABEL_MAX) + '…' : label
}

// --- Hover highlight graph ---

/** node id → set of directly connected node ids */
const neighborsOf = computed(() => {
  const map = new Map<number, Set<number>>()
  for (const r of relations.value) {
    let s = map.get(r.from_id)
    if (!s) { s = new Set(); map.set(r.from_id, s) }
    s.add(r.to_id)
    s = map.get(r.to_id)
    if (!s) { s = new Set(); map.set(r.to_id, s) }
    s.add(r.from_id)
  }
  return map
})

/** node ids in the full lineage (connected component) of a given node */
function lineageOf(nodeId: number): Set<number> {
  const visited = new Set<number>()
  const stack = [nodeId]
  while (stack.length > 0) {
    const cur = stack.pop()!
    if (visited.has(cur)) continue
    visited.add(cur)
    const nb = neighborsOf.value.get(cur)
    if (nb) for (const n of nb) stack.push(n)
  }
  return visited
}

/** Is this edge related to the hovered node? */
function isEdgeHighlighted(r: SectionRelation): boolean {
  if (hoveredId.value === null) return false
  return r.from_id === hoveredId.value || r.to_id === hoveredId.value
}

/** Is this node in the lineage of the hovered node? */
function isNodeHighlighted(nodeId: number): boolean {
  if (hoveredId.value === null) return false
  return lineageOf(hoveredId.value).has(nodeId)
}

// --- Simple timeline layout: date → column, stack vertically within column ---

/** Sorted unique dates from all sections */
const sortedDates = computed<string[]>(() => {
  const dates = new Set(sections.value.map(s => s.period_date.slice(0, 10)))
  return [...dates].sort()
})

/** date string → column index */
const dateIndex = computed(() => {
  const map = new Map<string, number>()
  sortedDates.value.forEach((d, i) => map.set(d, i))
  return map
})

/** Maximum nodes in any single date column */
const maxRows = computed(() => {
  const counts = new Map<string, number>()
  for (const s of sections.value) {
    const d = s.period_date.slice(0, 10)
    counts.set(d, (counts.get(d) ?? 0) + 1)
  }
  let max = 0
  for (const c of counts.values()) max = Math.max(max, c)
  return max
})

interface PositionedSection {
  data: SectionTimelineNode
  cx: number
  cy: number
}

/** Place each node at (col * COL_W, row * ROW_H) + padding */
const positionedNodes = computed<PositionedSection[]>(() => {
  const rowCounter = new Map<string, number>()
  return sections.value.map(s => {
    const date = s.period_date.slice(0, 10)
    const col = dateIndex.value.get(date) ?? 0
    const row = rowCounter.get(date) ?? 0
    rowCounter.set(date, row + 1)
    return {
      data: s,
      cx: col * COL_W + PAD + COL_W / 2,
      cy: row * ROW_H + PAD + ROW_H / 2,
    }
  })
})

/** Quick lookup: section id → pixel position */
const posById = computed(() => {
  const map = new Map<number, { cx: number; cy: number }>()
  for (const pn of positionedNodes.value) {
    map.set(pn.data.id, { cx: pn.cx, cy: pn.cy })
  }
  return map
})

interface EdgeLine {
  key: string
  d: string
  fromId: number
  toId: number
}

/** Bezier edges between related sections */
const edgePaths = computed<EdgeLine[]>(() => {
  return relations.value.map((r, i) => {
    const from = posById.value.get(r.from_id)
    const to = posById.value.get(r.to_id)
    if (!from || !to) return { key: `edge-${i}`, d: '', fromId: r.from_id, toId: r.to_id }
    const midX = (from.cx + to.cx) / 2
    return {
      key: `edge-${i}`,
      d: `M${from.cx},${from.cy} C${midX},${from.cy} ${midX},${to.cy} ${to.cx},${to.cy}`,
      fromId: r.from_id,
      toId: r.to_id,
    }
  }).filter(e => e.d !== '')
})

// SVG dimensions
const svgWidth = computed(() => sortedDates.value.length * COL_W + PAD * 2)
const svgHeight = computed(() => maxRows.value * ROW_H + PAD * 2)

// Date column headers
interface DateCol {
  date: string
  label: string
  x: number
}

const dateColumns = computed<DateCol[]>(() =>
  sortedDates.value.map((date, i) => ({
    date,
    label: formatDateShort(date),
    x: i * COL_W + PAD + COL_W / 2,
  })),
)

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

    <!-- Timeline -->
    <div v-else class="btb-chart">
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
            v-for="edge in edgePaths"
            :key="edge.key"
            :d="edge.d"
            fill="none"
            :stroke="isEdgeHighlighted({ from_id: edge.fromId, to_id: edge.toId, distance: 0 }) ? 'rgba(255,255,255,0.55)' : 'rgba(255,255,255,0.1)'"
            :stroke-width="isEdgeHighlighted({ from_id: edge.fromId, to_id: edge.toId, distance: 0 }) ? 2 : 1.5"
          />

          <!-- Nodes -->
          <g
            v-for="pn in positionedNodes"
            :key="'node-' + pn.data.id"
            class="btb-dag-node"
            :class="{
              'btb-dag-node--ending': pn.data.status === 'ending',
              'btb-dag-node--selected': selectedNode?.id === pn.data.id,
              'btb-dag-node--lineage': isNodeHighlighted(pn.data.id),
              'btb-dag-node--dimmed': hoveredId !== null && !isNodeHighlighted(pn.data.id),
            }"
            @click="selectNode(pn.data)"
            @mouseenter="hoveredId = pn.data.id"
            @mouseleave="hoveredId = null"
          >
            <circle
              :cx="pn.cx"
              :cy="pn.cy"
              :r="selectedNode?.id === pn.data.id ? NODE_R + 2 : NODE_R"
              :fill="statusFill(pn.data.status)"
              :stroke-dasharray="pn.data.status === 'ending' ? '3 2' : undefined"
              stroke="rgba(255,255,255,0.15)"
              :stroke-width="selectedNode?.id === pn.data.id ? 2 : 1"
            />
            <text
              :x="pn.cx"
              :y="pn.cy + NODE_R + 12"
              text-anchor="middle"
              class="btb-node-label"
              :fill="statusFill(pn.data.status)"
            >
              {{ truncateLabel(pn.data.cluster_label) }}
            </text>
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
  font-size: 0.6rem;
  color: rgba(255, 255, 255, 0.3);
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

/* Node label */
.btb-node-label {
  font-size: 9px;
  pointer-events: none;
  opacity: 0.7;
  transition: opacity 0.12s ease;
}

/* DAG nodes */
.btb-dag-node {
  cursor: pointer;
  transition: opacity 0.15s ease;
}

.btb-dag-node circle {
  transition: r 0.1s ease, filter 0.1s ease, stroke-width 0.1s ease;
}

.btb-dag-node:hover circle {
  filter: drop-shadow(0 0 5px rgba(255, 255, 255, 0.3));
  r: 9;
}

.btb-dag-node:hover .btb-node-label {
  opacity: 1;
}

/* Lineage highlight: hovered node's entire connected component */
.btb-dag-node--lineage circle {
  filter: drop-shadow(0 0 4px rgba(255, 255, 255, 0.2));
}

.btb-dag-node--lineage .btb-node-label {
  opacity: 1;
}

/* Dimmed: not in the hovered lineage */
.btb-dag-node--dimmed {
  opacity: 0.2;
}

/* Ending nodes */
.btb-dag-node--ending {
  opacity: 0.45;
}

.btb-dag-node--ending circle {
  stroke: #9ca3af;
}

.btb-dag-node--ending.btb-dag-node--lineage {
  opacity: 0.8;
}

/* Selected */
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
