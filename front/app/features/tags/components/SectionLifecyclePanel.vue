<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionLifecycleNode, type SectionRelation, type DailyReportThread } from '~/api/dailyReports'
import { useArticlesApi } from '~/api/articles'

const props = defineProps<{
  sectionId: number
}>()

const emit = defineEmits<{
  close: []
  navigate: [node: SectionLifecycleNode]
  openArticle: [articleId: number]
}>()

const { getSectionLifecycle, getDailyReportDetail } = useDailyReportsApi()
const { getArticle } = useArticlesApi()

const sections = ref<SectionLifecycleNode[]>([])
const relations = ref<SectionRelation[]>([])
const loading = ref(false)
const error = ref(false)

// Thread/article state per node
const selectedNodeId = ref<number | null>(null)
const expandedNodeId = ref<number | null>(null)
const nodeThreads = ref<Map<number, DailyReportThread[]>>(new Map())
const nodeThreadsLoading = ref(false)
const expandedThreadId = ref<number | null>(null)
const threadArticles = ref<Map<number, { id: number; title: string }[]>>(new Map())
const threadArticlesLoading = ref(false)

// Layout constants
const COL_W = 260
const ROW_H = 60
const PAD = 16
const NODE_W = 240
const NODE_H = 52

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
  return `${month}/${day} 周${weekDay}`
}

function truncateLabel(label: string, max = 18): string {
  return label.length > max ? label.slice(0, max) + '...' : label
}

// --- Layout: date columns, nodes stacked vertically ---

const sortedDates = computed(() => {
  const dates = new Set(sections.value.map(s => s.period_date.slice(0, 10)))
  return [...dates].sort()
})

const dateIndex = computed(() => {
  const map = new Map<string, number>()
  sortedDates.value.forEach((d, i) => map.set(d, i))
  return map
})

interface PositionedNode {
  data: SectionLifecycleNode
  cx: number
  cy: number
}

const positionedNodes = computed<PositionedNode[]>(() => {
  const rowCounter = new Map<string, number>()
  return sections.value.map(s => {
    const date = s.period_date.slice(0, 10)
    const col = dateIndex.value.get(date) ?? 0
    const row = rowCounter.get(date) ?? 0
    rowCounter.set(date, row + 1)
    return {
      data: s,
      cx: col * COL_W + PAD + NODE_W / 2,
      cy: row * ROW_H + PAD + NODE_H / 2,
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

const svgWidth = computed(() => {
  const cols = sortedDates.value.length
  return Math.max(400, cols * COL_W + PAD * 2)
})
const svgHeight = computed(() => {
  const maxRows = Math.max(1, ...(() => {
    const counts = new Map<string, number>()
    for (const s of sections.value) {
      const d = s.period_date.slice(0, 10)
      counts.set(d, (counts.get(d) ?? 0) + 1)
    }
    return [...counts.values()]
  })())
  return maxRows * ROW_H + PAD * 2
})

const currentSection = computed(() =>
  sections.value.find(s => s.id === props.sectionId),
)

// --- Hover highlight ---
const hoveredId = ref<number | null>(null)

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

function isEdgeHighlighted(r: { from_id: number; to_id: number }): boolean {
  if (hoveredId.value === null) return false
  return r.from_id === hoveredId.value || r.to_id === hoveredId.value
}

function isNodeHighlighted(nodeId: number): boolean {
  if (hoveredId.value === null) return false
  if (nodeId === hoveredId.value) return true
  const nb = neighborsOf.value.get(hoveredId.value)
  return nb ? nb.has(nodeId) : false
}

// --- Thread/article loading ---

async function toggleNodeThreads(node: SectionLifecycleNode) {
  if (nodeThreads.value.has(node.id)) return

  nodeThreadsLoading.value = true
  try {
    const res = await getDailyReportDetail(node.report_id)
    if (res.success && res.data) {
      const section = res.data.report.sections?.find(s => s.id === node.id)
      nodeThreads.value = new Map(nodeThreads.value).set(node.id, section?.threads || [])
    }
  } finally {
    nodeThreadsLoading.value = false
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

// --- Data loading ---

let fetchId = 0

async function fetchLifecycle() {
  loading.value = true
  error.value = false
  selectedNodeId.value = null
  expandedThreadId.value = null
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
    if (currentFetch === fetchId) loading.value = false
  }
}

watch(() => props.sectionId, () => { fetchLifecycle() }, { immediate: true })

function handleNodeClick(node: SectionLifecycleNode) {
  if (selectedNodeId.value === node.id) {
    selectedNodeId.value = null
    expandedNodeId.value = null
    expandedThreadId.value = null
    return
  }
  selectedNodeId.value = node.id
  toggleNodeThreads(node)
}

function navigateToNode(node: SectionLifecycleNode) {
  emit('navigate', node)
}
</script>

<template>
  <div class="slp-inline">
    <!-- Header -->
    <div class="slp-header">
      <button type="button" class="slp-back" @click="$emit('close')">
        <Icon icon="mdi:arrow-left" width="16" />
        <span>返回日报</span>
      </button>
      <span v-if="currentSection" class="slp-header-title">{{ currentSection.cluster_label }}</span>
      <span class="slp-header-label">话题生命周期</span>
    </div>

    <!-- Loading -->
    <div v-if="loading" class="slp-body">
      <div v-for="i in 2" :key="i" class="slp-skeleton-node">
        <div class="slp-skeleton-dot" />
        <div class="slp-skeleton-lines">
          <div class="slp-skeleton-line slp-skeleton-short" />
          <div class="slp-skeleton-line slp-skeleton-long" />
        </div>
      </div>
    </div>

    <div v-else-if="error" class="slp-body">
      <div class="slp-error">加载失败</div>
    </div>

    <div v-else-if="sections.length === 0" class="slp-body">
      <div class="slp-empty">独立话题，无演化记录</div>
    </div>

    <!-- Timeline DAG -->
    <div v-else class="slp-body">
      <!-- SVG timeline -->
      <div class="slp-svg-scroll">
        <svg :width="svgWidth" :height="svgHeight" class="slp-svg">
          <!-- Date grid lines -->
          <line
            v-for="(date, ci) in sortedDates"
            :key="'grid-' + date"
            :x1="ci * COL_W + PAD + NODE_W / 2"
            :y1="0"
            :x2="ci * COL_W + PAD + NODE_W / 2"
            :y2="svgHeight"
            stroke="rgba(80,60,30,0.06)"
          />

          <!-- Edges -->
          <path
            v-for="edge in edgePaths"
            :key="edge.key"
            :d="edge.d"
            fill="none"
            :stroke="isEdgeHighlighted(edge) ? 'rgba(80,60,30,0.5)' : 'rgba(80,60,30,0.12)'"
            :stroke-width="isEdgeHighlighted(edge) ? 2.5 : 1.2"
          />

          <!-- Nodes -->
          <g
            v-for="pn in positionedNodes"
            :key="'n-' + pn.data.id"
            class="slp-node"
            :class="{
              'slp-node--current': pn.data.id === sectionId,
              'slp-node--selected': selectedNodeId === pn.data.id,
              'slp-node--ending': pn.data.status === 'ending',
              'slp-node--lineage': isNodeHighlighted(pn.data.id),
              'slp-node--dimmed': hoveredId !== null && !isNodeHighlighted(pn.data.id),
            }"
            :transform="`translate(${pn.cx - NODE_W / 2}, ${pn.cy - NODE_H / 2})`"
            @click="handleNodeClick(pn.data)"
            @mouseenter="hoveredId = pn.data.id"
            @mouseleave="hoveredId = null"
          >
            <rect
              :width="NODE_W"
              :height="NODE_H"
              rx="6"
              :fill="pn.data.id === sectionId ? 'rgba(80,60,20,0.08)' : 'rgba(255,255,255,0.55)'"
              :stroke="pn.data.status === 'ending' ? 'rgba(120,120,120,0.25)' : 'rgba(80,60,30,0.15)'"
              :stroke-dasharray="pn.data.status === 'ending' ? '4,3' : 'none'"
              stroke-width="1"
            />
            <!-- Navigate button -->
            <g class="slp-nav-btn" @click.stop="navigateToNode(pn.data)">
              <rect :x="NODE_W - 18" y="3" width="14" height="14" rx="3" fill="rgba(80,60,30,0.06)" />
              <path d="M${NODE_W - 14},7 L${NODE_W - 8},10 L${NODE_W - 14},13" stroke="rgba(80,60,30,0.3)" stroke-width="1" fill="none" />
            </g>
            <!-- Status dot -->
            <circle cx="12" :cy="NODE_H / 2" r="4" :fill="statusColorMap[pn.data.status] || '#9ca3af'" />
            <!-- Label -->
            <text x="22" y="18" fill="rgba(40,30,10,0.85)" font-size="11" font-weight="500" font-family="system-ui,sans-serif">
              {{ truncateLabel(pn.data.cluster_label) }}
            </text>
            <!-- Meta -->
            <text x="22" y="34" fill="rgba(80,60,30,0.4)" font-size="9" font-family="system-ui,sans-serif">
              {{ formatDate(pn.data.period_date) }} · {{ pn.data.article_count }}篇 · {{ pn.data.thread_count }}条线索
            </text>
            <!-- Status badge -->
            <rect :x="NODE_W - 42" y="17" width="34" height="16" rx="3"
              :fill="statusColorMap[pn.data.status] + '15'"
              :stroke="statusColorMap[pn.data.status] + '35'"
              stroke-width="0.5"
            />
            <text :x="NODE_W - 25" y="28" :fill="statusColorMap[pn.data.status]"
              font-size="8" font-weight="500" text-anchor="middle" font-family="system-ui,sans-serif">
              {{ statusLabel[pn.data.status] || pn.data.status }}
            </text>
          </g>
        </svg>
      </div>

      <!-- Node detail panel: threads & articles -->
      <div class="slp-detail">
        <template v-for="pn in positionedNodes" :key="'detail-' + pn.data.id">
          <div v-if="selectedNodeId === pn.data.id" class="slp-detail-node">
            <div class="slp-detail-title">{{ pn.data.cluster_label }}</div>

            <div v-if="nodeThreadsLoading" class="slp-detail-loading">加载线索中…</div>
            <div v-else-if="(nodeThreads.get(pn.data.id) || []).length === 0" class="slp-detail-empty">无关联线索</div>
            <template v-else>
              <div
                v-for="thread in (nodeThreads.get(pn.data.id) || [])"
                :key="thread.id"
                class="slp-thread"
                :class="{ 'slp-thread--expanded': expandedThreadId === thread.id }"
              >
                <div class="slp-thread-header" @click="toggleThreadArticles(thread)">
                  <Icon icon="mdi:chevron-right" width="12" class="slp-thread-arrow" />
                  <span class="slp-thread-title">{{ thread.title }}</span>
                  <span class="slp-thread-count">{{ thread.related_article_ids?.length || 0 }}篇</span>
                </div>
                <div v-if="thread.summary" class="slp-thread-summary">{{ thread.summary }}</div>

                <Transition name="slp-slide">
                  <div v-if="expandedThreadId === thread.id" class="slp-articles">
                    <div v-if="threadArticlesLoading" class="slp-articles-loading">加载中…</div>
                    <template v-else>
                      <div
                        v-for="art in (threadArticles.get(thread.id) || [])"
                        :key="art.id"
                        class="slp-article"
                        @click="emit('openArticle', art.id)"
                      >
                        <Icon icon="mdi:file-document-outline" width="11" class="slp-article-icon" />
                        <span class="slp-article-title">{{ art.title }}</span>
                        <Icon icon="mdi:eye-outline" width="10" class="slp-article-eye" />
                      </div>
                      <div v-if="(thread.related_article_ids?.length || 0) > 10" class="slp-articles-more">
                        还有 {{ thread.related_article_ids.length - 10 }} 篇…
                      </div>
                    </template>
                  </div>
                </Transition>
              </div>
            </template>
          </div>
        </template>
      </div>
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
  padding-bottom: 0.7rem;
  border-bottom: 1px solid rgba(80, 60, 30, 0.1);
  margin-bottom: 0.7rem;
  flex-shrink: 0;
}

.slp-back {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.25rem 0.55rem;
  border: 1px solid rgba(80, 60, 30, 0.12);
  border-radius: 5px;
  background: rgba(255, 255, 255, 0.4);
  color: rgba(60, 40, 10, 0.65);
  font-size: 0.72rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.slp-back:hover {
  background: rgba(255, 255, 255, 0.6);
  color: rgba(40, 20, 0, 0.85);
}

.slp-header-title {
  font-size: 0.82rem;
  font-weight: 600;
  color: rgba(40, 30, 10, 0.85);
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slp-header-label {
  font-size: 0.68rem;
  color: rgba(80, 60,  30, 0.35);
  flex-shrink: 0;
}

/* Body */
.slp-body {
  flex: 1;
  min-height: 0;
  overflow-y: auto;
}

.slp-svg-scroll {
  overflow-x: auto;
  margin-bottom: 0.6rem;
}

.slp-svg {
  display: block;
}

.slp-node {
  cursor: pointer;
}

.slp-node:hover rect {
  fill: rgba(255, 255, 255, 0.75);
}

.slp-node--current rect {
  stroke-width: 2;
  stroke: rgba(60, 40, 10, 0.3);
}

.slp-node--selected rect {
  stroke-width: 1.5;
  stroke: rgba(60, 40, 10, 0.25);
  fill: rgba(255, 255, 255, 0.7) !important;
}

.slp-nav-btn {
  cursor: pointer;
  opacity: 0;
  transition: opacity 0.12s;
}

.slp-node:hover .slp-nav-btn {
  opacity: 1;
}

.slp-nav-btn:hover rect {
  fill: rgba(80, 60, 30, 0.12) !important;
}

.slp-node--ending {
  opacity: 0.5;
}

.slp-node--lineage rect {
  stroke: rgba(60, 40, 10, 0.35) !important;
  stroke-width: 1.5;
  fill: rgba(255, 255, 255, 0.75) !important;
}

.slp-node--dimmed {
  opacity: 0.2;
  filter: saturate(0.3) brightness(0.7);
}

/* Detail panel */
.slp-detail {
  flex-shrink: 0;
}

.slp-detail-node {
  border-top: 1px solid rgba(80, 60, 30, 0.08);
  padding-top: 0.5rem;
}

.slp-detail-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: rgba(40, 30, 10, 0.8);
  margin-bottom: 0.4rem;
}

.slp-detail-loading,
.slp-detail-empty {
  font-size: 0.7rem;
  color: rgba(80, 60, 30, 0.3);
  padding: 0.3rem 0;
}

/* Threads */
.slp-thread {
  margin-bottom: 0.25rem;
  border-radius: 4px;
  border: 1px solid rgba(80, 60, 30, 0.06);
  background: rgba(255, 255, 255, 0.35);
  overflow: hidden;
}

.slp-thread--expanded {
  border-color: rgba(80, 60, 30, 0.12);
}

.slp-thread-header {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.3rem 0.4rem;
  cursor: pointer;
  transition: background 0.1s;
}

.slp-thread-header:hover {
  background: rgba(255, 255, 255, 0.3);
}

.slp-thread-arrow {
  color: rgba(80, 60, 30, 0.25);
  transition: transform 0.15s;
  flex-shrink: 0;
}

.slp-thread--expanded .slp-thread-arrow {
  transform: rotate(90deg);
}

.slp-thread-title {
  flex: 1;
  font-size: 0.68rem;
  color: rgba(40, 30, 10, 0.75);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slp-thread-count {
  font-size: 0.58rem;
  color: rgba(80, 60, 30, 0.3);
  flex-shrink: 0;
}

.slp-thread-summary {
  font-size: 0.62rem;
  color: rgba(80, 60, 30, 0.4);
  line-height: 1.4;
  padding: 0 0.4rem 0.25rem 1.2rem;
}

/* Articles */
.slp-articles {
  border-top: 1px solid rgba(80, 60, 30, 0.05);
  padding: 0.2rem 0.35rem 0.3rem;
}

.slp-articles-loading {
  font-size: 0.62rem;
  color: rgba(80, 60, 30, 0.25);
  padding: 0.15rem 0;
}

.slp-article {
  display: flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.25rem;
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.1s;
}

.slp-article:hover {
  background: rgba(255, 255, 255, 0.4);
}

.slp-article:hover .slp-article-title {
  color: rgba(30, 15, 0, 0.9);
}

.slp-article-icon {
  color: rgba(80, 60, 30, 0.2);
  flex-shrink: 0;
}

.slp-article-title {
  flex: 1;
  font-size: 0.62rem;
  color: rgba(40, 30, 10, 0.55);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.slp-article-eye {
  color: rgba(80, 60, 30, 0.15);
  flex-shrink: 0;
}

.slp-articles-more {
  font-size: 0.58rem;
  color: rgba(80, 60, 30, 0.2);
  padding: 0.1rem 0.25rem;
}

/* Slide animation */
.slp-slide-enter-active {
  transition: max-height 150ms ease-out, opacity 150ms ease-out;
}
.slp-slide-leave-active {
  transition: max-height 100ms ease-in, opacity 100ms ease-in;
}
.slp-slide-enter-from,
.slp-slide-leave-to {
  max-height: 0;
  opacity: 0;
  overflow: hidden;
}

/* Skeleton */
.slp-skeleton-node {
  display: flex;
  gap: 0.6rem;
  padding: 0.5rem 0;
}

.slp-skeleton-dot {
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: rgba(80, 60, 30, 0.08);
  margin-top: 4px;
}

.slp-skeleton-lines {
  display: flex;
  flex-direction: column;
  gap: 0.3rem;
  flex: 1;
}

.slp-skeleton-line {
  height: 8px;
  border-radius: 3px;
  background: rgba(80, 60, 30, 0.05);
  animation: slpPulse 1.5s ease-in-out infinite;
}

.slp-skeleton-short { width: 40%; }
.slp-skeleton-long { width: 80%; }

@keyframes slpPulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

.slp-error {
  text-align: center;
  padding: 2rem 0;
  font-size: 0.78rem;
  color: rgba(80, 60, 30, 0.35);
}

.slp-empty {
  text-align: center;
  padding: 2rem 0;
  font-size: 0.78rem;
  color: rgba(80, 60, 30, 0.3);
}
</style>
