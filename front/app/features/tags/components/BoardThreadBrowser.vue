<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation, type DailyReportThread } from '~/api/dailyReports'
import { useArticlesApi } from '~/api/articles'

const props = defineProps<{ boardId: number }>()

const { getBoardSectionTimeline, getDailyReportDetail } = useDailyReportsApi()
const { getArticle } = useArticlesApi()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)
const hoveredId = ref<number | null>(null)

// --- Popup thread/article state ---
const popupThreads = ref<DailyReportThread[]>([])
const popupThreadsLoading = ref(false)
const expandedThreadId = ref<number | null>(null)
const threadArticles = ref<Map<number, { id: number, title: string }[]>>(new Map())
const threadArticlesLoading = ref(false)

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

function isEdgeHighlighted(r: { fromId: number; toId: number }): boolean {
  if (hoveredId.value === null) return false
  return r.fromId === hoveredId.value || r.toId === hoveredId.value
}

function isNodeHighlighted(nodeId: number): boolean {
  if (hoveredId.value === null) return false
  if (nodeId === hoveredId.value) return true
  const nb = neighborsOf.value.get(hoveredId.value)
  return nb ? nb.has(nodeId) : false
}

// --- Simple timeline layout ---

const sortedDates = computed<string[]>(() => {
  const dates = new Set(sections.value.map(s => s.period_date.slice(0, 10)))
  return [...dates].sort()
})

const dateIndex = computed(() => {
  const map = new Map<string, number>()
  sortedDates.value.forEach((d, i) => map.set(d, i))
  return map
})

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

const svgWidth = computed(() => sortedDates.value.length * COL_W + PAD * 2)
const svgHeight = computed(() => maxRows.value * ROW_H + PAD * 2)

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

// --- Node select → load threads ---

async function selectNode(node: SectionTimelineNode) {
  if (selectedNode.value?.id === node.id) {
    selectedNode.value = null
    popupThreads.value = []
    expandedThreadId.value = null
    return
  }
  selectedNode.value = node
  expandedThreadId.value = null
  popupThreads.value = []
  threadArticles.value = new Map()

  popupThreadsLoading.value = true
  try {
    const res = await getDailyReportDetail(node.report_id)
    if (res.success && res.data) {
      const section = res.data.report.sections?.find(s => s.id === node.id)
      popupThreads.value = section?.threads || []
    }
  } finally {
    popupThreadsLoading.value = false
  }
}

async function toggleThreadArticles(thread: DailyReportThread) {
  if (expandedThreadId.value === thread.id) {
    expandedThreadId.value = null
    return
  }
  expandedThreadId.value = thread.id

  if (threadArticles.value.has(thread.id)) return
  if (!thread.related_article_ids?.length) return

  threadArticlesLoading.value = true
  const ids = thread.related_article_ids.slice(0, 10)
  const results = await Promise.allSettled(ids.map(id => getArticle(id)))
  const articles = results.map((r, i) => {
    const aid = ids[i]!
    if (r.status === 'fulfilled' && r.value.success && r.value.data) {
      return { id: aid, title: r.value.data.title || '(无标题)' }
    }
    return { id: aid, title: `文章 #${aid}` }
  })
  threadArticles.value = new Map(threadArticles.value).set(thread.id, articles)
  threadArticlesLoading.value = false
}

const emit = defineEmits<{
  openArticle: [articleId: number]
}>()

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
            :stroke="isEdgeHighlighted(edge) ? 'rgba(255,255,255,0.65)' : 'rgba(255,255,255,0.08)'"
            :stroke-width="isEdgeHighlighted(edge) ? 2.5 : 1"
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

          <!-- Threads -->
          <div class="btb-threads">
            <div v-if="popupThreadsLoading" class="btb-threads-loading">
              <div v-for="i in 2" :key="i" class="btb-thread-skeleton" />
            </div>
            <div v-else-if="popupThreads.length === 0" class="btb-threads-empty">
              无关联线索
            </div>
            <div
              v-else
              v-for="thread in popupThreads"
              :key="thread.id"
              class="btb-thread"
              :class="{ 'btb-thread--expanded': expandedThreadId === thread.id }"
            >
              <div class="btb-thread-header" @click="toggleThreadArticles(thread)">
                <Icon icon="mdi:chevron-right" width="14" class="btb-thread-arrow" />
                <span class="btb-thread-title">{{ thread.title }}</span>
                <span class="btb-thread-count">{{ thread.related_article_ids?.length || 0 }}篇</span>
              </div>
              <div v-if="thread.summary" class="btb-thread-summary">{{ thread.summary }}</div>

              <!-- Articles -->
              <Transition name="btb-slide">
                <div v-if="expandedThreadId === thread.id" class="btb-articles">
                  <div v-if="threadArticlesLoading" class="btb-articles-loading">加载中…</div>
                  <template v-else>
                    <div
                      v-for="art in (threadArticles.get(thread.id) || [])"
                      :key="art.id"
                      class="btb-article"
                      @click="emit('openArticle', art.id)"
                    >
                      <Icon icon="mdi:file-document-outline" width="12" class="btb-article-icon" />
                      <span class="btb-article-title">{{ art.title }}</span>
                      <Icon icon="mdi:eye-outline" width="10" class="btb-article-external" />
                    </div>
                    <div
                      v-if="thread.related_article_ids?.length > 10"
                      class="btb-articles-more"
                    >
                      还有 {{ thread.related_article_ids.length - 10 }} 篇…
                    </div>
                  </template>
                </div>
              </Transition>
            </div>
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

.btb-dag-node--lineage circle {
  filter: drop-shadow(0 0 8px rgba(255, 255, 255, 0.45));
  stroke: rgba(255, 255, 255, 0.5) !important;
  stroke-width: 2 !important;
  r: 9;
}

.btb-dag-node--lineage .btb-node-label {
  opacity: 1;
}

.btb-dag-node--dimmed {
  opacity: 0.15;
  filter: saturate(0.3) brightness(0.6);
}

.btb-dag-node--dimmed .btb-node-label {
  opacity: 0.3;
}

.btb-dag-node--ending {
  opacity: 0.45;
}

.btb-dag-node--ending circle {
  stroke: #9ca3af;
}

.btb-dag-node--ending.btb-dag-node--lineage {
  opacity: 0.8;
}

.btb-dag-node--selected circle {
  filter: drop-shadow(0 0 6px rgba(255, 255, 255, 0.35));
  stroke: rgba(255, 255, 255, 0.5);
}

/* Popup overlay */
.btb-popup-overlay {
  position: fixed;
  inset: 0;
  z-index: 200;
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
  max-width: 420px;
  width: 90vw;
  max-height: 80vh;
  overflow-y: auto;
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

/* Threads section */
.btb-threads {
  margin-top: 0.7rem;
  border-top: 1px solid rgba(255, 255, 255, 0.06);
  padding-top: 0.6rem;
}

.btb-threads-loading,
.btb-threads-empty {
  font-size: 0.7rem;
  color: rgba(255, 255, 255, 0.25);
  text-align: center;
  padding: 0.5rem 0;
}

.btb-thread-skeleton {
  height: 28px;
  border-radius: 4px;
  background: rgba(255, 255, 255, 0.03);
  margin-bottom: 0.3rem;
  animation: btbPulse 1.5s ease-in-out infinite;
}

.btb-thread {
  margin-bottom: 0.35rem;
  border-radius: 6px;
  border: 1px solid rgba(255, 255, 255, 0.06);
  background: rgba(255, 255, 255, 0.02);
  overflow: hidden;
}

.btb-thread--expanded {
  border-color: rgba(255, 255, 255, 0.1);
}

.btb-thread-header {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.4rem 0.5rem;
  cursor: pointer;
  transition: background 0.1s ease;
}

.btb-thread-header:hover {
  background: rgba(255, 255, 255, 0.04);
}

.btb-thread-arrow {
  color: rgba(255, 255, 255, 0.25);
  transition: transform 0.15s ease;
  flex-shrink: 0;
}

.btb-thread--expanded .btb-thread-arrow {
  transform: rotate(90deg);
}

.btb-thread-title {
  flex: 1;
  font-size: 0.72rem;
  color: rgba(255, 255, 255, 0.7);
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btb-thread-count {
  font-size: 0.6rem;
  color: rgba(255, 255, 255, 0.2);
  flex-shrink: 0;
}

.btb-thread-summary {
  font-size: 0.65rem;
  color: rgba(255, 255, 255, 0.35);
  line-height: 1.4;
  padding: 0 0.5rem 0.3rem 1.4rem;
}

/* Articles */
.btb-articles {
  border-top: 1px solid rgba(255, 255, 255, 0.04);
  padding: 0.3rem 0.5rem 0.4rem;
}

.btb-articles-loading {
  font-size: 0.65rem;
  color: rgba(255, 255, 255, 0.25);
  padding: 0.2rem 0;
}

.btb-article {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.25rem 0.3rem;
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.1s ease;
}

.btb-article:hover {
  background: rgba(255, 255, 255, 0.05);
}

.btb-article:hover .btb-article-title {
  color: rgba(255, 255, 255, 0.9);
}

.btb-article-icon {
  color: rgba(255, 255, 255, 0.2);
  flex-shrink: 0;
}

.btb-article-title {
  flex: 1;
  font-size: 0.65rem;
  color: rgba(255, 255, 255, 0.5);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btb-article:hover .btb-article-title {
  color: rgba(255, 255, 255, 0.9);
}

.btb-article-external {
  color: rgba(255, 255, 255, 0.15);
  flex-shrink: 0;
}

.btb-articles-more {
  font-size: 0.6rem;
  color: rgba(255, 255, 255, 0.2);
  padding: 0.15rem 0.3rem;
}

/* Slide animation for articles */
.btb-slide-enter-active {
  transition: max-height 150ms ease-out, opacity 150ms ease-out;
}
.btb-slide-leave-active {
  transition: max-height 100ms ease-in, opacity 100ms ease-in;
}
.btb-slide-enter-from,
.btb-slide-leave-to {
  max-height: 0;
  opacity: 0;
  overflow: hidden;
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
