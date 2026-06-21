<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation, type DailyReportThread } from '~/api/dailyReports'
import { useArticlesApi } from '~/api/articles'

const props = defineProps<{ boardId: number }>()

const { getBoardSectionTimeline, getDailyReportDetail, backfillPersistentTopics, updateTopic, mergeTopics } = useDailyReportsApi()
const { getArticle } = useArticlesApi()
const { theme } = useTheme()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)
const hoveredId = ref<number | null>(null)
// 视图模式：timeline=匈牙利相似度 DAG（默认）；lanes=按话题分泳道（identity 驱动）。
const viewMode = ref<'timeline' | 'lanes'>('timeline')

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
const LANE_LABEL_MAX = 12   // 泳道标签截断阈值（比节点标签宽）
// 泳道视图参数
const LANE_LABEL_W = 160 // 左侧话题标签列宽（容结 LANE_LABEL_MAX 个字符，不再截断）

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

// --- Detective wall entry: only on large screens with WebGL support ---
const viewportWidth = ref(typeof window !== 'undefined' ? window.innerWidth : 1280)
const showDetectiveEntry = computed(() => {
  if (viewportWidth.value < 768) return false
  try {
    const c = document.createElement('canvas')
    return !!(c.getContext('webgl2') || c.getContext('webgl'))
  } catch {
    return false
  }
})
// Named handler so it can be removed on unmount (avoid leaked listeners).
function onViewportResize() { viewportWidth.value = window.innerWidth }
onMounted(() => window.addEventListener('resize', onViewportResize))
onUnmounted(() => window.removeEventListener('resize', onViewportResize))

// --- SVG 缩放（滚轮）---
// 用 CSS transform scale 包裹 SVG，不动其坐标，点节点/连线的命中不受影响。
// 按鼠标位置缩放（缩放后鼠标下的点不动），缩放后容器溢出可滚动。
const svgScrollRef = ref<HTMLDivElement | null>(null)
const zoomScale = ref(1)
const MIN_SCALE = 0.4
const MAX_SCALE = 3
const ZOOM_STEP = 0.2
const zoomPercent = computed(() => Math.round(zoomScale.value * 100))

function zoomIn() {
  zoomScale.value = Math.min(MAX_SCALE, +(zoomScale.value + ZOOM_STEP).toFixed(2))
}
function zoomOut() {
  zoomScale.value = Math.max(MIN_SCALE, +(zoomScale.value - ZOOM_STEP).toFixed(2))
}

function resetZoom() {
  zoomScale.value = 1
  const el = svgScrollRef.value
  if (el) { el.scrollLeft = 0; el.scrollTop = 0 }
}

// Theme-aware SVG colors
const svgGridColor = computed(() => theme.value === 'dark' ? 'rgba(255,255,255,0.04)' : 'rgba(26,26,26,0.06)')
const svgEdgeColor = computed(() => theme.value === 'dark' ? 'rgba(255,255,255,' : 'rgba(26,26,26,')
const svgNodeStroke = computed(() => theme.value === 'dark' ? 'rgba(255,255,255,0.15)' : 'rgba(26,26,26,0.15)')
const svgHighlightColor = computed(() => theme.value === 'dark' ? 'rgba(255,255,255,0.65)' : 'rgba(26,26,26,0.65)')
// 泳道相关主题色（本次新增）
const svgLaneLabelColor = computed(() => theme.value === 'dark' ? 'rgba(226,232,240,0.88)' : 'rgba(26,26,26,0.85)')
const svgLaneStripeColor = computed(() => theme.value === 'dark' ? 'rgba(255,255,255,0.025)' : 'rgba(26,26,26,0.03)')

function getEdgeColor(distance: number, highlighted: boolean): string {
  if (highlighted) return svgHighlightColor.value
  return `${svgEdgeColor.value}${edgeOpacity(distance)})`
}

// --- Date helpers ---

function formatDateShort(dateStr: string): string {
  const d = new Date(dateStr)
  return `${d.getMonth() + 1}/${d.getDate()}`
}

function truncateLabel(label: string): string {
  return label.length > LABEL_MAX ? label.slice(0, LABEL_MAX) + '…' : label
}
// 泳道标签用更宽的截断阈值；hover 仍会通过 <title> 显示完整名称。
function truncateLaneLabel(label: string): string {
  return label.length > LANE_LABEL_MAX ? label.slice(0, LANE_LABEL_MAX) + '…' : label
}

// --- Hover highlight graph ---

/** node id → set of directly connected node ids */
const neighborsOf = computed(() => {
  const map = new Map<number, Set<number>>()
  for (const r of relations.value) {
    // 邻居关系跟随当前视图模式：时间线只走匈牙利相似度，泳道只走 identity。
    const isIdentity = r.relation_type === 'identity'
    if (viewMode.value === 'timeline' && isIdentity) continue
    if (viewMode.value === 'lanes' && !isIdentity) continue
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

// --- 泳道视图：按话题归类（topics/unassignedCount 来自下方话题管理块） ---
interface LaneRow { key: string; label: string; color: string; status: string }
const laneRows = computed<LaneRow[]>(() => {
  const rows: LaneRow[] = topics.value.map(t => ({
    key: `topic-${t.id}`, label: t.label, color: t.color, status: t.status,
  }))
  if (unassignedCount.value > 0) {
    rows.push({ key: 'unassigned', label: '未分类', color: '#64748b', status: 'none' })
  }
  return rows
})
const laneIndexByKey = computed(() => {
  const m = new Map<string, number>()
  laneRows.value.forEach((r, i) => m.set(r.key, i))
  return m
})
const sectionById = computed(() => {
  const m = new Map<number, SectionTimelineNode>()
  for (const s of sections.value) m.set(s.id, s)
  return m
})
function sectionLaneKey(s: SectionTimelineNode): string {
  if (s.persistent_topic) return `topic-${s.persistent_topic.id}`
  return 'unassigned'
}

interface PositionedSection {
  data: SectionTimelineNode
  cx: number
  cy: number
}

// 泳道自适应高度：每条泳道高 = 基础行高 + 该泳道内单天最大节点数 × 节点间距。
// 固定高度会导致同天多节点的泳道互相覆盖（节点+标签溢出），动态高度让每条
// 泳道都够容结自己的内容，间隔充裕。
const LANE_NODE_GAP = 16      // 同泳道同天多节点的纵向间距
const LANE_BASE = 40         // 基础行高（单节点时的垂直空间）
const laneLayout = computed(() => {
  const lanes = laneRows.value
  const laneH = new Array(lanes.length).fill(LANE_BASE)
  const subMax = new Map<string, number>()
  for (const s of sections.value) {
    const date = s.period_date.slice(0, 10)
    const li = laneIndexByKey.value.get(sectionLaneKey(s)) ?? 0
    const k = `${li}:${date}`
    const n = (subMax.get(k) ?? 0) + 1
    subMax.set(k, n)
    // 单节点只需 LANE_BASE；每多一个节点 +节点间距（含节点半径与标签余量）。
    const need = LANE_BASE + (n - 1) * LANE_NODE_GAP
    if (need > laneH[li]!) laneH[li] = need
  }
  const laneY: number[] = []
  let acc = 0
  for (const h of laneH) {
    laneY.push(acc)
    acc += h
  }
  return { laneH, laneY, subMax }
})

const positionedNodes = computed<PositionedSection[]>(() => {
  if (viewMode.value === 'lanes') {
    const layout = laneLayout.value
    const seen = new Map<string, number>()
    return sections.value.map(s => {
      const date = s.period_date.slice(0, 10)
      const col = dateIndex.value.get(date) ?? 0
      const li = laneIndexByKey.value.get(sectionLaneKey(s)) ?? 0
      const k = `${li}:${date}`
      const total = layout.subMax.get(k) ?? 1
      const idx = seen.get(k) ?? 0
      seen.set(k, idx + 1)
      const subOffset = (idx - (total - 1) / 2) * 16
      return {
        data: s,
        cx: col * COL_W + PAD + COL_W / 2 + LANE_LABEL_W,
        // 与背景 rect 同基准（rect y = laneY[li] + PAD），节点 cy 也 + PAD。
        cy: PAD + layout.laneY[li]! + layout.laneH[li]! / 2 + subOffset,
      }
    })
  }
  // timeline 模式：同天按出现顺序纵向堆叠。
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
  distance: number
  relationType: string  // 'identity'（同话题）| 'similarity'（匈牙利）
  color?: string        // identity 边的话题色
}

/** Map distance to visual weight: strong (<0.15) → 2, medium (<0.25) → 1.2, weak → 0.6 */
function edgeWeight(dist: number): number {
  if (dist < 0.15) return 2
  if (dist < 0.25) return 1.2
  return 0.6
}

/** Map distance to opacity: strong → 0.35, medium → 0.18, weak → 0.08 */
function edgeOpacity(dist: number): number {
  if (dist < 0.15) return 0.35
  if (dist < 0.25) return 0.18
  return 0.08
}

const edgePaths = computed<EdgeLine[]>(() => {
  return relations.value
    .filter(r => {
      // 时间线模式默认只显匈牙利相似度边（隐藏 identity）。
      if (viewMode.value === 'timeline' && r.relation_type === 'identity') return false
      // 泳道模式只显 identity 边（同话题延续）；跨泳道相似度会切断归类，改看时间线。
      if (viewMode.value === 'lanes' && r.relation_type !== 'identity') return false
      return true
    })
    .map((r, i) => {
      const relType = r.relation_type ?? 'similarity'
      const from = posById.value.get(r.from_id)
      const to = posById.value.get(r.to_id)
      if (!from || !to) return { key: `edge-${i}`, d: '', fromId: r.from_id, toId: r.to_id, distance: r.distance, relationType: relType }
      const midX = (from.cx + to.cx) / 2
      let color: string | undefined
      if (relType === 'identity') {
        const fromNode = sectionById.value.get(r.from_id)
        color = fromNode?.persistent_topic?.color
      }
      return {
        key: `edge-${i}`,
        d: `M${from.cx},${from.cy} C${midX},${from.cy} ${midX},${to.cy} ${to.cx},${to.cy}`,
        fromId: r.from_id,
        toId: r.to_id,
        distance: r.distance,
        relationType: relType,
        color,
      }
    }).filter(e => e.d !== '')
})

const svgWidth = computed(() => {
  const base = sortedDates.value.length * COL_W + PAD * 2
  return viewMode.value === 'lanes' ? base + LANE_LABEL_W : base
})
const svgHeight = computed(() => {
  if (viewMode.value === 'lanes') {
    const total = laneLayout.value.laneH.reduce((a, b) => a + b, 0)
    return total + PAD * 2
  }
  return maxRows.value * ROW_H + PAD * 2
})

interface DateCol {
  date: string
  label: string
  x: number
}

const dateColumns = computed<DateCol[]>(() =>
  sortedDates.value.map((date, i) => ({
    date,
    label: formatDateShort(date),
    x: i * COL_W + PAD + COL_W / 2 + (viewMode.value === 'lanes' ? LANE_LABEL_W : 0),
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
  openDetectiveWall: []
}>()

// --- Persistent topic management (§最小版：话题清单 + 回刷 + 重命名/归档/合并) ---
const showTopicPanel = ref(false)
const backfilling = ref(false)
const managing = ref(false)

interface TopicRow {
  id: number
  label: string
  status: string
  color: string
  sectionCount: number
  firstDate: string
  lastDate: string
}

// 话题清单由 section-timeline 的 persistent_topic 字段聚合，无需额外端点。
const topics = computed<TopicRow[]>(() => {
  const map = new Map<number, TopicRow>()
  for (const s of sections.value) {
    const t = s.persistent_topic
    if (!t) continue
    const row = map.get(t.id) ?? {
      id: t.id, label: t.label, status: t.status, color: t.color,
      sectionCount: 0, firstDate: s.period_date.slice(0, 10), lastDate: s.period_date.slice(0, 10),
    }
    row.sectionCount++
    const d = s.period_date.slice(0, 10)
    if (d < row.firstDate) row.firstDate = d
    if (d > row.lastDate) row.lastDate = d
    // status 以最新一天的为准（timeline 已按时间排序）。
    row.status = t.status
    map.set(t.id, row)
  }
  return [...map.values()].sort((a, b) => {
    // 归档沉底；其余按 section 数降序。
    const aArchived = a.status === 'archived' ? 1 : 0
    const bArchived = b.status === 'archived' ? 1 : 0
    if (aArchived !== bArchived) return aArchived - bArchived
    return b.sectionCount - a.sectionCount
  })
})

const topicStats = computed(() => {
  let active = 0, candidate = 0, archived = 0
  for (const t of topics.value) {
    if (t.status === 'active') active++
    else if (t.status === 'candidate') candidate++
    else if (t.status === 'archived') archived++
  }
  return { active, candidate, archived, total: topics.value.length }
})

const topicStatusLabel: Record<string, string> = { candidate: '候选', active: '活跃', archived: '已归档' }
const unassignedCount = computed(() => sections.value.filter(s => !s.persistent_topic_id).length)

async function runBackfill() {
  if (backfilling.value) return
  // 有未归属 section 时强调语义；无未归属也允许（重新跑全量）。
  const hint = unassignedCount.value > 0
    ? `检测到 ${unassignedCount.value} 条未归属历史动态，将为它们重建话题。继续？`
    : '将为本板块重新构建话题并重排关系。继续？'
  if (!window.confirm(hint)) return
  backfilling.value = true
  try {
    const res = await backfillPersistentTopics(props.boardId)
    if (res.success) {
      // 后台异步执行；给用户提示并几秒后刷新列表。
      window.alert('已提交回刷，后台重建中，几秒后自动刷新。')
      setTimeout(() => { loadData() }, 4000)
    }
  } finally {
    backfilling.value = false
  }
}

async function renameTopic(t: TopicRow) {
  const label = window.prompt('重命名话题', t.label)
  if (!label || label.trim() === '' || label.trim() === t.label) return
  managing.value = true
  try {
    const res = await updateTopic(t.id, { label: label.trim() })
    if (res.success) await loadData()
  } finally { managing.value = false }
}

async function archiveTopic(t: TopicRow) {
  if (!window.confirm(`归档话题「${t.label}」？归档后不再参与新归属。`)) return
  managing.value = true
  try {
    const res = await updateTopic(t.id, { status: 'archived' })
    if (res.success) await loadData()
  } finally { managing.value = false }
}

async function mergeTopic(t: TopicRow) {
  const candidates = topics.value.filter(o => o.id !== t.id && o.status !== 'archived')
  if (candidates.length === 0) {
    window.alert('没有其他可合并的话题')
    return
  }
  // 用原生 prompt 让用户输入目标话题名；最小版不做复杂选择器。
  const lines = candidates.map((c, i) => `${i + 1}. ${c.label}（${c.sectionCount}条）`).join('\n')
  const input = window.prompt(`将「${t.label}」合并进哪个话题？输入序号：\n${lines}`)
  if (!input) return
  const idx = parseInt(input, 10) - 1
  if (Number.isNaN(idx) || idx < 0 || idx >= candidates.length) return
  const target = candidates[idx]!
  if (!window.confirm(`确认将「${t.label}」合并进「${target.label}」？`)) return
  managing.value = true
  try {
    const res = await mergeTopics(target.id, [t.id])
    if (res.success) await loadData()
  } finally { managing.value = false }
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

// 切换视图模式时清空选中/悬停，避免坐标体系变化后的错位高亮。
watch(viewMode, () => {
  selectedNode.value = null
  hoveredId.value = null
  popupThreads.value = []
})
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
      <div class="btb-view-toggle">
        <button
          class="btb-view-btn"
          :class="{ active: viewMode === 'timeline' }"
          title="匈牙利相似度关系（默认）"
          @click="viewMode = 'timeline'"
        >
          <Icon icon="mdi:chart-timeline-variant" width="13" />
          <span>时间线</span>
        </button>
        <button
          class="btb-view-btn"
          :class="{ active: viewMode === 'lanes' }"
          title="按话题归类，同话题连续性实线连接"
          @click="viewMode = 'lanes'"
        >
          <Icon icon="mdi:view-stream-outline" width="13" />
          <span>话题泳道</span>
        </button>
      </div>
      <button
        v-if="showDetectiveEntry"
        class="btb-detective-btn"
        title="进入 3D 侦探墙"
        @click="emit('openDetectiveWall')"
      >
        <Icon icon="mdi:magnify-scan" width="16" />
        <span>侦探墙</span>
      </button>
      <button
        class="btb-detective-btn"
        :class="{ 'btb-detective-btn--active': showTopicPanel }"
        title="话题管理（回刷 / 重命名 / 归档 / 合并）"
        @click="showTopicPanel = !showTopicPanel"
      >
        <Icon icon="mdi:folder-cog-outline" width="16" />
        <span>话题管理</span>
      </button>
    </div>

    <!-- Topic management panel (§最小版) -->
    <div v-if="showTopicPanel" class="btb-topics">
      <div class="btb-topics-head">
        <div class="btb-topics-stats">
          <span><b>{{ topicStats.active }}</b> 活跃</span>
          <span><b>{{ topicStats.candidate }}</b> 候选</span>
          <span><b>{{ topicStats.archived }}</b> 已归档</span>
          <span v-if="unassignedCount > 0" class="btb-topics-warn">{{ unassignedCount }} 条未归属</span>
        </div>
        <button class="btb-topics-backfill" :disabled="backfilling" @click="runBackfill">
          <Icon icon="mdi:database-refresh-outline" width="14" />
          <span>{{ backfilling ? '提交中…' : '回刷历史话题' }}</span>
        </button>
      </div>
      <div v-if="topics.length === 0" class="btb-topics-empty">
        本板块尚无持久话题。点「回刷历史话题」可从历史日报重建。
      </div>
      <div v-else class="btb-topics-list">
        <div
          v-for="t in topics"
          :key="t.id"
          class="btb-topic-row"
          :class="{ 'btb-topic-row--archived': t.status === 'archived' }"
        >
          <span class="btb-topic-color" :style="{ background: t.color }" />
          <div class="btb-topic-main">
            <span class="btb-topic-label">{{ t.label }}</span>
            <span class="btb-topic-meta">
              {{ topicStatusLabel[t.status] || t.status }} · {{ t.sectionCount }} 条 · {{ t.firstDate }}→{{ t.lastDate }}
            </span>
          </div>
          <div class="btb-topic-ops" v-if="t.status !== 'archived'">
            <button class="btb-topic-op" :disabled="managing" @click="renameTopic(t)">重命名</button>
            <button class="btb-topic-op" :disabled="managing" @click="mergeTopic(t)">合并</button>
            <button class="btb-topic-op" :disabled="managing" @click="archiveTopic(t)">归档</button>
          </div>
        </div>
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

      <!-- SVG canvas（+/− 按钮缩放；transform scale 不改变布局尺寸，故 zoom 容器的
           width/height 需按缩放后尺寸算，父滚动容器才能正确撑开出现滚动条） -->
      <div ref="svgScrollRef" class="btb-svg-scroll">
        <div
          class="btb-svg-zoom"
          :style="{
            transform: `scale(${zoomScale})`,
            transformOrigin: '0 0',
            width: Math.round(svgWidth * zoomScale) + 'px',
            height: Math.round(svgHeight * zoomScale) + 'px',
          }"
        >
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
            :stroke="svgGridColor"
            stroke-width="1"
          />

          <!-- Lane backgrounds + labels (lanes mode only) -->
          <template v-if="viewMode === 'lanes'">
            <g v-for="(lane, li) in laneRows" :key="'lane-' + lane.key">
              <rect
                :x="0"
                :y="(laneLayout.laneY[li] ?? 0) + PAD"
                :width="svgWidth"
                :height="laneLayout.laneH[li] ?? LANE_BASE"
                :fill="li % 2 === 0 ? svgLaneStripeColor : 'transparent'"
              />
              <circle
                :cx="14"
                :cy="(laneLayout.laneY[li] ?? 0) + PAD + (laneLayout.laneH[li] ?? LANE_BASE) / 2"
                :r="5"
                :fill="lane.color"
              />
              <text
                :x="24"
                :y="(laneLayout.laneY[li] ?? 0) + PAD + (laneLayout.laneH[li] ?? LANE_BASE) / 2 + 4"
                class="btb-lane-label"
                :fill="svgLaneLabelColor"
              >{{ truncateLaneLabel(lane.label) }}<title>{{ lane.label }}</title></text>
            </g>
          </template>

          <!-- Edges -->
          <path
            v-for="edge in edgePaths"
            :key="edge.key"
            :d="edge.d"
            fill="none"
            :stroke="viewMode === 'lanes' && edge.relationType === 'identity' ? (edge.color || '#60a5fa') : getEdgeColor(edge.distance, isEdgeHighlighted(edge))"
            :stroke-width="viewMode === 'lanes' && edge.relationType === 'identity' ? 2 : (isEdgeHighlighted(edge) ? 2.5 : edgeWeight(edge.distance))"
            :stroke-dasharray="viewMode === 'lanes' && edge.relationType !== 'identity' ? '4 3' : undefined"
            :opacity="viewMode === 'lanes' && edge.relationType === 'identity' ? (isEdgeHighlighted(edge) ? 0.95 : 0.55) : (viewMode === 'lanes' ? 0.5 : 1)"
          >
            <title>{{ edge.relationType === 'identity' ? '同话题延续' : '相似度' }} · 距离: {{ edge.distance.toFixed(3) }}</title>
          </path>

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
            <title>{{ pn.data.cluster_label }}（{{ pn.data.period_date.slice(0, 10) }}）{{ pn.data.persistent_topic ? '\n话题：' + pn.data.persistent_topic.label : '' }}{{ pn.data.article_count ? '\n' + pn.data.article_count + ' 篇 · ' + pn.data.thread_count + ' 线索' : '' }}</title>
            <circle
              :cx="pn.cx"
              :cy="pn.cy"
              :r="selectedNode?.id === pn.data.id ? NODE_R + 2 : NODE_R"
              :fill="statusFill(pn.data.status)"
              :stroke-dasharray="pn.data.status === 'ending' ? '3 2' : undefined"
              :stroke="svgNodeStroke"
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
      <!-- 缩放控制条 -->
      <div class="btb-zoom-bar">
        <button class="btb-zoom-btn" :disabled="zoomScale <= MIN_SCALE" title="缩小" @click="zoomOut">
          <Icon icon="mdi:minus" width="14" />
        </button>
        <button class="btb-zoom-label" :disabled="zoomScale === 1" title="重置为 100%" @click="resetZoom">{{ zoomPercent }}%</button>
        <button class="btb-zoom-btn" :disabled="zoomScale >= MAX_SCALE" title="放大" @click="zoomIn">
          <Icon icon="mdi:plus" width="14" />
        </button>
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
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
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
  color: var(--color-text-secondary);
}

.btb-days-toggle {
  display: flex;
  gap: 0.25rem;
}

.btb-days-btn {
  padding: 0.2rem 0.55rem;
  font-size: 0.65rem;
  border: 1px solid var(--color-border-medium);
  border-radius: 4px;
  background: transparent;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.12s ease;
}

.btb-days-btn:hover {
  color: var(--color-text-secondary);
  border-color: var(--color-border-strong);
}

.btb-days-btn.active {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
  border-color: var(--color-border-strong);
}

/* Detective wall entry button */
.btb-detective-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.25rem 0.6rem;
  font-size: 0.65rem;
  border: 1px solid #DC2626;
  border-radius: 4px;
  background: rgba(220, 38, 38, 0.1);
  color: #DC2626;
  cursor: pointer;
  transition: all 0.12s ease;
}

.btb-detective-btn:hover {
  background: #DC2626;
  color: #fff;
}

/* §话题管理最小版 */
.btb-detective-btn--active {
  background: rgba(96, 165, 250, 0.18);
  border-color: #60a5fa;
  color: #93c5fd;
}
.btb-topics {
  margin: 0 0 0.75rem;
  padding: 0.75rem;
  border: 1px solid rgba(96, 165, 250, 0.22);
  border-radius: 8px;
  background: rgba(15, 23, 42, 0.45);
}
.btb-topics-head {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
  margin-bottom: 0.6rem;
  flex-wrap: wrap;
}
.btb-topics-stats {
  display: flex;
  gap: 0.85rem;
  font-size: 0.72rem;
  color: rgba(226, 232, 240, 0.7);
}
.btb-topics-stats b {
  color: #e2e8f0;
  font-weight: 600;
}
.btb-topics-warn {
  color: #fbbf24;
}
.btb-topics-backfill {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.3rem 0.7rem;
  font-size: 0.7rem;
  border: 1px solid #60a5fa;
  border-radius: 4px;
  background: rgba(96, 165, 250, 0.12);
  color: #93c5fd;
  cursor: pointer;
  transition: all 0.12s ease;
}
.btb-topics-backfill:hover:not(:disabled) {
  background: rgba(96, 165, 250, 0.28);
}
.btb-topics-backfill:disabled {
  opacity: 0.55;
  cursor: wait;
}
.btb-topics-empty {
  padding: 1rem 0.5rem;
  font-size: 0.72rem;
  text-align: center;
  color: rgba(226, 232, 240, 0.45);
}
.btb-topics-list {
  display: flex;
  flex-direction: column;
  gap: 0.35rem;
  max-height: 320px;
  overflow-y: auto;
}
.btb-topic-row {
  display: flex;
  align-items: center;
  gap: 0.55rem;
  padding: 0.45rem 0.55rem;
  border-radius: 6px;
  background: rgba(30, 41, 59, 0.5);
}
.btb-topic-row--archived {
  opacity: 0.5;
}
.btb-topic-color {
  width: 10px;
  height: 10px;
  border-radius: 50%;
  flex-shrink: 0;
}
.btb-topic-main {
  flex: 1;
  min-width: 0;
  display: flex;
  flex-direction: column;
  gap: 1px;
}
.btb-topic-label {
  font-size: 0.8rem;
  font-weight: 600;
  color: #e2e8f0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.btb-topic-meta {
  font-size: 0.65rem;
  color: rgba(148, 163, 184, 0.8);
}
.btb-topic-ops {
  display: flex;
  gap: 0.25rem;
  flex-shrink: 0;
}
.btb-topic-op {
  padding: 0.2rem 0.45rem;
  font-size: 0.65rem;
  border: 1px solid rgba(148, 163, 184, 0.3);
  border-radius: 4px;
  background: transparent;
  color: rgba(203, 213, 225, 0.85);
  cursor: pointer;
  transition: all 0.12s ease;
}
.btb-topic-op:hover:not(:disabled) {
  border-color: #60a5fa;
  color: #93c5fd;
}
.btb-topic-op:disabled {
  opacity: 0.5;
  cursor: wait;
}

/* §视图切换 + 泳道 */
.btb-view-toggle {
  display: inline-flex;
  gap: 2px;
  padding: 2px;
  border: 1px solid rgba(148, 163, 184, 0.25);
  border-radius: 5px;
  background: rgba(15, 23, 42, 0.4);
}
.btb-view-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.25rem;
  padding: 0.2rem 0.5rem;
  font-size: 0.65rem;
  color: rgba(203, 213, 225, 0.65);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  transition: all 0.12s ease;
}
.btb-view-btn:hover { color: #e2e8f0; }
.btb-view-btn.active {
  background: rgba(96, 165, 250, 0.18);
  color: #93c5fd;
}
.btb-lane-label {
  font-size: 10px;
  font-weight: 600;
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
  background: var(--color-bg-hover);
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
  color: var(--color-text-muted);
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
  color: var(--color-text-muted);
  white-space: nowrap;
}

/* SVG scroll container（缩放后两向可滚动） */
.btb-svg-scroll {
  overflow: auto;
  position: relative;
}

.btb-svg-zoom {
  /* transform 由内联 style 设置；这里只保证布局基准为左上角 */
}

.btb-svg {
  display: block;
}

/* 缩放控制条（右下角浮动，主题适配） */
.btb-zoom-bar {
  position: absolute;
  bottom: 0.4rem;
  right: 0.5rem;
  display: inline-flex;
  align-items: center;
  gap: 2px;
  padding: 2px;
  background: var(--color-bg-overlay);
  border: 1px solid var(--color-border-medium);
  border-radius: 5px;
  z-index: 2;
}
.btb-zoom-btn {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  padding: 0;
  color: var(--color-text-secondary);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease;
}
.btb-zoom-btn:hover:not(:disabled) {
  background: var(--color-bg-hover);
  color: var(--color-text-primary);
}
.btb-zoom-btn:disabled {
  opacity: 0.35;
  cursor: not-allowed;
}
.btb-zoom-label {
  min-width: 42px;
  padding: 0 0.3rem;
  font-size: 0.65rem;
  font-variant-numeric: tabular-nums;
  color: var(--color-text-secondary);
  background: transparent;
  border: none;
  border-radius: 3px;
  cursor: pointer;
}
.btb-zoom-label:hover:not(:disabled) {
  color: var(--color-text-primary);
}
.btb-zoom-label:disabled {
  cursor: default;
  color: var(--color-text-muted);
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
  background: var(--color-bg-overlay);
}

.btb-popup {
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-medium);
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
  color: var(--color-text-muted);
  cursor: pointer;
  transition: color 0.12s ease;
}

.btb-popup-close:hover {
  color: var(--color-text-secondary);
}

.btb-popup-title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.4;
  margin-bottom: 0.3rem;
}

.btb-popup-meta {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  font-size: 0.65rem;
  color: var(--color-text-muted);
}

/* Threads section */
.btb-threads {
  margin-top: 0.7rem;
  border-top: 1px solid var(--color-border-subtle);
  padding-top: 0.6rem;
}

.btb-threads-loading,
.btb-threads-empty {
  font-size: 0.7rem;
  color: var(--color-text-muted);
  text-align: center;
  padding: 0.5rem 0;
}

.btb-thread-skeleton {
  height: 28px;
  border-radius: 4px;
  background: var(--color-bg-hover);
  margin-bottom: 0.3rem;
  animation: btbPulse 1.5s ease-in-out infinite;
}

.btb-thread {
  margin-bottom: 0.35rem;
  border-radius: 6px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
  overflow: hidden;
}

.btb-thread--expanded {
  border-color: var(--color-border-medium);
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
  background: var(--color-bg-hover);
}

.btb-thread-arrow {
  color: var(--color-text-muted);
  transition: transform 0.15s ease;
  flex-shrink: 0;
}

.btb-thread--expanded .btb-thread-arrow {
  transform: rotate(90deg);
}

.btb-thread-title {
  flex: 1;
  font-size: 0.72rem;
  color: var(--color-text-secondary);
  line-height: 1.3;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btb-thread-count {
  font-size: 0.6rem;
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.btb-thread-summary {
  font-size: 0.65rem;
  color: var(--color-text-muted);
  line-height: 1.4;
  padding: 0 0.5rem 0.3rem 1.4rem;
}

/* Articles */
.btb-articles {
  border-top: 1px solid var(--color-border-subtle);
  padding: 0.3rem 0.5rem 0.4rem;
}

.btb-articles-loading {
  font-size: 0.65rem;
  color: var(--color-text-muted);
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
  background: var(--color-bg-hover);
}

.btb-article:hover .btb-article-title {
  color: var(--color-text-primary);
}

.btb-article-icon {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.btb-article-title {
  flex: 1;
  font-size: 0.65rem;
  color: var(--color-text-muted);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.btb-article:hover .btb-article-title {
  color: var(--color-text-primary);
}

.btb-article-external {
  color: var(--color-text-muted);
  flex-shrink: 0;
}

.btb-articles-more {
  font-size: 0.6rem;
  color: var(--color-text-muted);
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
