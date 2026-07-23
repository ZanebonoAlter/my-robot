<script setup lang="ts">
import { ref, computed, watch, onMounted, onUnmounted, nextTick } from 'vue'
import { Icon } from '@iconify/vue'
import { useDailyReportsApi, type SectionTimelineNode, type SectionRelation, type DailyReportThread, type BoardTopicListItem } from '~/api/dailyReports'
import { useArticlesApi } from '~/api/articles'
import { fullComponentHighlight } from '~/utils/graphHighlight'
import { filterFocusNodes, isDragMove, buildFocusMeta } from './topicFocus'
import ComposePanel from './ComposePanel.vue'

const props = defineProps<{ boardId: number }>()

const {
  getBoardSectionTimeline,
  getDailyReportDetail,
  updateTopic,
  deleteTopic,
  mergeTopics,
  backfillPersistentTopics,
  listBoardTopics,
} = useDailyReportsApi()
const { getArticle } = useArticlesApi()
const { theme } = useTheme()

const days = ref(14)
const loading = ref(false)
const sections = ref<SectionTimelineNode[]>([])
const relations = ref<SectionRelation[]>([])
const selectedNode = ref<SectionTimelineNode | null>(null)
const hoveredId = ref<number | null>(null)
// 视图模式：timeline=匈牙利相似度 DAG（默认）；lanes=按话题分泳道（identity 驱动）；
// focus=从某条泳道进入单话题专注视图（复用 lanes 数据，仅重投影单话题节点）；
// compose=手动建泳道编排态（预览 + 候选池 + 体检，切片③）。
const viewMode = ref<'timeline' | 'lanes' | 'focus' | 'compose'>('timeline')
// focus 视图锁定的话题 id（由点泳道进入）；lanes/timeline 下保持 null。
const focusedTopicId = ref<number | null>(null)

// --- Popup thread/article state ---
const popupThreads = ref<DailyReportThread[]>([])
const popupThreadsLoading = ref(false)
const expandedThreadId = ref<number | null>(null)
const threadArticles = ref<Map<number, { id: number, title: string }[]>>(new Map())
const threadArticlesLoading = ref(false)

// --- Constants ---
const COL_W = 148
const ROW_H = 60
const PAD = 32
const NODE_R = 7
const LABEL_MAX = 10
const LANE_LABEL_MAX = 8    // 泳道标签截断阈值（避让右侧 hover 操作菜单，防重叠）
// 泳道视图参数
const LANE_LABEL_W = 180 // 左侧话题标签列宽（容结标签 + hover 操作菜单，避免重叠）

// --- Status styling ---
// Colors reference Layer 2 theme tokens (defined in main.css as
// --color-thread-status-*), so they adapt to editorial/dark themes.
const statusColorMap: Record<string, string> = {
  emerging: 'var(--color-thread-status-emerging)',
  continuing: 'var(--color-thread-status-continuing)',
  split: 'var(--color-thread-status-split)',
  merge: 'var(--color-thread-status-merge)',
  ending: 'var(--color-thread-status-ending)',
}

const statusLabels: Record<string, string> = {
  emerging: '新兴',
  continuing: '持续',
  split: '分化',
  merge: '合并',
  ending: '结束',
}

function statusFill(status: string): string {
  return statusColorMap[status] || 'var(--color-thread-status-ending)'
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

async function setZoom(nextScale: number) {
  const el = svgScrollRef.value
  const oldScale = zoomScale.value
  const centerX = el ? (el.scrollLeft + el.clientWidth / 2) / oldScale : 0
  const centerY = el ? (el.scrollTop + el.clientHeight / 2) / oldScale : 0
  zoomScale.value = nextScale
  await nextTick()
  if (el) {
    el.scrollLeft = Math.max(0, centerX * nextScale - el.clientWidth / 2)
    el.scrollTop = Math.max(0, centerY * nextScale - el.clientHeight / 2)
  }
}

function zoomIn() {
  void setZoom(Math.min(MAX_SCALE, +(zoomScale.value + ZOOM_STEP).toFixed(2)))
}
function zoomOut() {
  void setZoom(Math.max(MIN_SCALE, +(zoomScale.value - ZOOM_STEP).toFixed(2)))
}

function resetZoom() {
  void setZoom(1)
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

const visibleRelations = computed(() => relations.value.filter((r) => {
  const isIdentity = r.relation_type === 'identity'
  return viewMode.value === 'lanes' ? isIdentity : !isIdentity
}))

const highlightedNodeIds = computed(() => {
  if (hoveredId.value === null) return new Set<number>()
  return fullComponentHighlight(
    hoveredId.value,
    visibleRelations.value.map(r => ({ source: r.from_id, target: r.to_id })),
  )
})

function isEdgeHighlighted(r: { fromId: number; toId: number }): boolean {
  if (hoveredId.value === null) return false
  return highlightedNodeIds.value.has(r.fromId) && highlightedNodeIds.value.has(r.toId)
}

function isNodeHighlighted(nodeId: number): boolean {
  return hoveredId.value !== null && highlightedNodeIds.value.has(nodeId)
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
interface LaneRow { key: string; label: string; color: string; status: string; topicId: number | null }
const laneRows = computed<LaneRow[]>(() => {
  const rows: LaneRow[] = topics.value.filter(t => t.status === 'active').map(t => ({
    key: `topic-${t.id}`, label: t.label, color: t.color, status: t.status, topicId: t.id,
  }))
  if (nonActiveSectionCount.value > 0) {
    rows.push({ key: 'unassigned', label: '待确认 / 未分类', color: '#64748b', status: 'none', topicId: null })
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
  if (s.persistent_topic?.status === 'active') return `topic-${s.persistent_topic.id}`
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
const LANE_NODE_GAP = 24      // 同泳道同天多节点的纵向间距
const LANE_BASE = 52         // 基础行高（单节点时的垂直空间）
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
      const subOffset = (idx - (total - 1) / 2) * LANE_NODE_GAP
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
  return visibleRelations.value
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

// --- 编排态（切片③）：手动建泳道 ---
// 进入 compose 时记住原视图，保存/取消回到原 viewMode（spec Scenario「保存后返回总览」）。
const prevViewMode = ref<'timeline' | 'lanes'>('lanes')
function enterCompose() {
  prevViewMode.value = viewMode.value === 'timeline' ? 'timeline' : 'lanes'
  viewMode.value = 'compose'
}
function exitCompose() {
  viewMode.value = prevViewMode.value
}
async function onComposeSaved() {
  exitCompose()
  await loadData()
}

// 编排态「确认启用」候选话题：candidate→active（后端校验累计 hit_count≥阈值），刷新总览。
async function onActivateCandidate(topicId: number) {
  const res = await updateTopic(topicId, { status: 'active' })
  if (res.success) {
    await loadData()
  } else {
    opsError.value = res.error || '启用候选失败'
  }
}

// --- Persistent topic management (list drives the lanes view) ---

interface TopicRow {
  id: number
  label: string
  status: string
  color: string
  sectionCount: number
  firstDate: string
  lastDate: string
  hitCount: number
  consecutiveHits: number
  canActivate: boolean
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
      hitCount: t.hit_count ?? 0, consecutiveHits: t.consecutive_hits, canActivate: t.can_activate,
    }
    row.sectionCount++
    const d = s.period_date.slice(0, 10)
    if (d < row.firstDate) row.firstDate = d
    if (d > row.lastDate) row.lastDate = d
    // status 以最新一天的为准（timeline 已按时间排序）。
    row.status = t.status
    row.consecutiveHits = t.consecutive_hits
    row.hitCount = t.hit_count ?? 0
    row.canActivate = t.can_activate
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
const nonActiveSectionCount = computed(() => sections.value.filter(s => s.persistent_topic?.status !== 'active').length)

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
  () => [props.boardId, days.value],
  () => { loadData() },
  { immediate: true },
)

// --- Focus view (slice C′)：重投影单话题节点到专注布局（复用 lanes 数据）---
const focusScrollRef = ref<HTMLDivElement | null>(null)
// focus 列宽（节点间距），独立于 lanes 的 COL_W，给单话题更多呼吸空间。
const FOCUS_COL_W = 96
// 拖拽 vs 点击阈值：超过该位移视为拖拽，吞掉后续 click（避免误触展开）。
const FOCUS_DRAG_THRESHOLD = 3

const focusNodes = computed<SectionTimelineNode[]>(() =>
  focusedTopicId.value == null ? [] : filterFocusNodes(sections.value, focusedTopicId.value),
)
const focusMeta = computed(() => buildFocusMeta(focusNodes.value))
const focusTimelineWidth = computed(() => focusNodes.value.length * FOCUS_COL_W + 32)
const focusTopicLabel = computed(() => {
  const id = focusedTopicId.value
  if (id == null) return ''
  return topics.value.find(t => t.id === id)?.label ?? `话题 #${id}`
})
const focusStatusLabel = computed(() => {
  const id = focusedTopicId.value
  if (id == null) return ''
  const t = topics.value.find(x => x.id === id)
  if (!t) return ''
  return t.status === 'active' ? 'Active · 持续演进' : (topicStatusLabel[t.status] ?? t.status)
})

function enterFocus(topicId: number) {
  focusedTopicId.value = topicId
  selectedNode.value = null
  popupThreads.value = []
  expandedThreadId.value = null
  viewMode.value = 'focus'
}
function exitFocus() {
  viewMode.value = 'lanes'
}
// 只有具名话题泳道（active topic）可进入专注视图；unassigned 不进。
function onLaneClick(lane: LaneRow) {
  if (lane.topicId == null) return
  enterFocus(lane.topicId)
}

// 拖拽平移：pointer 事件区分 click vs drag（超阈值吞掉后续 click）。
const dragState = ref({ down: false, startX: 0, startScroll: 0, moved: false })
let suppressNextFocusClick = false
function onPointerDown(e: PointerEvent) {
  const el = focusScrollRef.value
  if (!el) return
  // 点在节点上时不启动拖拽，让节点 click 优先。
  if ((e.target as Element)?.closest?.('.btb-focus-node')) return
  dragState.value = { down: true, startX: e.clientX, startScroll: el.scrollLeft, moved: false }
  el.setPointerCapture?.(e.pointerId)
}
function onPointerMove(e: PointerEvent) {
  if (!dragState.value.down) return
  const dx = e.clientX - dragState.value.startX
  if (isDragMove(dx, FOCUS_DRAG_THRESHOLD)) dragState.value.moved = true
  const el = focusScrollRef.value
  if (el) el.scrollLeft = dragState.value.startScroll - dx
}
function endDrag() {
  if (dragState.value.down && dragState.value.moved) suppressNextFocusClick = true
  dragState.value.down = false
  dragState.value.moved = false
}
function onPointerUp() { endDrag() }
function onPointerCancel() { endDrag() }
async function onFocusNodeClick(node: SectionTimelineNode) {
  // 拖拽刚结束 → 吞掉这次 click，避免误触就地展开。
  if (suppressNextFocusClick) { suppressNextFocusClick = false; return }
  await selectNode(node)
}

// --- 工具条话题操作（回刷 / 合并）+ 泳道 hover 操作（重命名 / 归档·恢复 / 删除）---
// 原 TopicManageDialog 能力迁入工作台；二次确认统一走 AppDialog（禁 window.*）。
const opsError = ref<string | null>(null)
const busyTopicId = ref<number | null>(null)
const backfilling = ref(false)

// 泳道 hover 操作菜单的纵向锚点（泳道竖向中心）。
function laneOpsY(li: number): number {
  return (laneLayout.value.laneY[li] ?? 0) + PAD + (laneLayout.value.laneH[li] ?? LANE_BASE) / 2
}

// 重命名
const renameTarget = ref<TopicRow | null>(null)
const renameLabel = ref('')
function startRename(lane: LaneRow) {
  const row = topics.value.find(t => t.id === lane.topicId) ?? null
  renameTarget.value = row
  renameLabel.value = lane.label
}
async function confirmRename() {
  const t = renameTarget.value
  if (!t) return
  const label = renameLabel.value.trim()
  if (!label || label === t.label) { renameTarget.value = null; return }
  busyTopicId.value = t.id
  const res = await updateTopic(t.id, { label })
  busyTopicId.value = null
  if (res.success) {
    renameTarget.value = null
    await loadData()
  } else {
    opsError.value = res.error || '重命名失败'
  }
}

// 归档 / 恢复（active→archived；archived→active）
const statusTarget = ref<TopicRow | null>(null)
const statusNext = ref<'archived' | 'active'>('archived')
function startStatusToggle(lane: LaneRow) {
  const row = topics.value.find(t => t.id === lane.topicId) ?? null
  if (!row) return
  statusTarget.value = row
  statusNext.value = row.status === 'archived' ? 'active' : 'archived'
}
async function confirmStatusToggle() {
  const t = statusTarget.value
  if (!t) return
  busyTopicId.value = t.id
  const res = await updateTopic(t.id, { status: statusNext.value })
  busyTopicId.value = null
  if (res.success) {
    statusTarget.value = null
    await loadData()
  } else {
    opsError.value = res.error || (statusNext.value === 'archived' ? '归档失败' : '恢复失败')
  }
}

// 删除（输入名称二次确认，硬删除不可恢复）
const deleteTarget = ref<TopicRow | null>(null)
const deleteConfirmText = ref('')
const deleteCanConfirm = computed(() => {
  const t = deleteTarget.value
  return !!t && deleteConfirmText.value.trim() === t.label.trim()
})
function startDelete(lane: LaneRow) {
  const row = topics.value.find(t => t.id === lane.topicId) ?? null
  if (!row) return
  deleteTarget.value = row
  deleteConfirmText.value = ''
}
async function confirmDelete() {
  const t = deleteTarget.value
  if (!t || !deleteCanConfirm.value) return
  busyTopicId.value = t.id
  const res = await deleteTopic(t.id)
  busyTopicId.value = null
  if (res.success) {
    deleteTarget.value = null
    deleteConfirmText.value = ''
    await loadData()
  } else {
    opsError.value = res.error || '删除失败'
  }
}

// 合并预览：拉取全量话题（含已归档），选源 + 目标，调 merge API。
const mergeOpen = ref(false)
const mergeAllTopics = ref<BoardTopicListItem[]>([])
const mergeLoading = ref(false)
const mergeSourceId = ref<number | null>(null)
const mergeTargetId = ref<number | null>(null)
const mergeCandidates = computed(() =>
  mergeAllTopics.value.filter(t => t.id !== mergeSourceId.value && t.status !== 'archived'),
)
async function startMerge() {
  mergeOpen.value = true
  mergeSourceId.value = null
  mergeTargetId.value = null
  mergeLoading.value = true
  const res = await listBoardTopics(props.boardId)
  mergeLoading.value = false
  if (res.success && res.data) {
    mergeAllTopics.value = res.data.topics ?? []
  } else {
    mergeAllTopics.value = []
    opsError.value = res.error || '加载话题失败'
  }
}
async function confirmMerge() {
  const sId = mergeSourceId.value
  const tId = mergeTargetId.value
  if (sId == null || tId == null) return
  busyTopicId.value = sId
  const res = await mergeTopics(tId, [sId])
  busyTopicId.value = null
  if (res.success) {
    mergeOpen.value = false
    mergeSourceId.value = null
    mergeTargetId.value = null
    await loadData()
  } else {
    opsError.value = res.error || '合并失败'
  }
}

// 回刷历史话题归属（原弹窗能力，迁入工具条）
async function runBackfill() {
  if (backfilling.value) return
  backfilling.value = true
  opsError.value = null
  const res = await backfillPersistentTopics(props.boardId)
  backfilling.value = false
  if (res.success) {
    // 后端异步重建，稍后刷新。
    setTimeout(() => { loadData() }, 4000)
  } else {
    opsError.value = res.error || '回刷失败'
  }
}

// 切换视图模式时清空选中/悬停，避免坐标体系变化后的错位高亮。
watch(viewMode, () => {
  selectedNode.value = null
  hoveredId.value = null
  popupThreads.value = []
  if (viewMode.value !== 'focus') focusedTopicId.value = null
})
</script>

<template>
  <div class="btb-container">
    <!-- Controls -->
    <div v-if="viewMode !== 'compose'" class="btb-controls">
      <div class="btb-controls-left">
        <Icon icon="mdi:source-branch" width="15" class="text-white/50" />
        <span class="btb-controls-title">话题总览</span>
      </div>
      <div class="btb-controls-actions">
        <div class="btb-days-toggle">
        <button
          v-for="opt in [{ v: 7, label: '7天' }, { v: 14, label: '14天' }, { v: 30, label: '30天' }, { v: 0, label: '全部' }]"
          :key="opt.v"
          class="btb-days-btn"
          :class="{ active: days === opt.v }"
          :title="opt.v === 0 ? '全部历史' : `最近 ${opt.v} 天`"
          @click="days = opt.v"
        >
          {{ opt.label }}
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
          class="btb-tool-btn"
          title="回刷历史话题归属（原话题管理能力）"
          :disabled="backfilling"
          @click="runBackfill"
        >
          <Icon icon="mdi:database-refresh-outline" width="14" />
          <span>{{ backfilling ? '回刷中…' : '回刷归属' }}</span>
        </button>
        <button
          class="btb-tool-btn"
          title="合并预览（选源 + 目标话题）"
          @click="startMerge"
        >
          <Icon icon="mdi:merge" width="14" />
          <span>合并预览</span>
        </button>
        <button
          class="btb-tool-btn btb-tool-btn--primary"
          title="新建泳道（进入编排态）"
          @click="enterCompose"
        >
          <Icon icon="mdi:plus" width="14" />
          <span>新建泳道</span>
        </button>
      </div>
    </div>

    <!-- Ops error banner -->
    <div v-if="opsError" class="btb-ops-error" role="alert">
      <Icon icon="mdi:alert-circle-outline" width="14" />
      <span>{{ opsError }}</span>
      <button class="btb-ops-error-close" @click="opsError = null">✕</button>
    </div>

    <!-- 编排态：手动建泳道（预览 + 候选池 + 体检），自带工具条；总览工具条隐藏 -->
    <ComposePanel
      v-if="viewMode === 'compose'"
      :board-id="boardId"
      :days="days"
      :candidate-topics="topics.filter(t => t.status === 'candidate')"
      @saved="onComposeSaved"
      @cancel="exitCompose"
      @activate-candidate="onActivateCandidate"
    />

    <!-- Loading -->
    <div v-else-if="loading" class="btb-loading">
      <div v-for="i in 3" :key="i" class="btb-skeleton" />
    </div>

    <!-- Empty -->
    <div v-else-if="sections.length === 0" class="btb-empty">
      <Icon icon="mdi:source-branch" width="28" class="text-white/15" />
      <p>暂无话题数据</p>
    </div>

    <!-- Focus view (single topic deep read) -->
    <div v-else-if="viewMode === 'focus'" class="btb-focus">
      <!-- 空话题降级：该话题在当前时间窗口内无节点，不报错 -->
      <div v-if="focusMeta.empty" class="btb-focus-empty">
        <Icon icon="mdi:bookmark-off-outline" width="22" />
        <p>该话题在当前时间窗口内暂无新动态</p>
        <button class="btb-focus-back" @click="exitFocus">
          <Icon icon="mdi:arrow-left" width="13" />
          返回总览
        </button>
      </div>
      <template v-else>
        <!-- sticky 标题：话题名 + 状态徽章 + 元信息（动态数/跨度/最近日期）-->
        <div class="btb-focus-head">
          <div class="btb-focus-head__top">
            <button class="btb-focus-back" @click="exitFocus">
              <Icon icon="mdi:arrow-left" width="13" />
              返回总览
            </button>
            <span v-if="focusStatusLabel" class="btb-focus-status">
              <span class="btb-focus-status__dot" />
              {{ focusStatusLabel }}
            </span>
          </div>
          <h2 class="btb-focus-title">{{ focusTopicLabel }}</h2>
          <div class="btb-focus-meta">
            <span><b>{{ focusMeta.count }}</b> 个动态</span>
            <span>跨度 <b>{{ focusMeta.spanDays }} 天</b></span>
            <span>最近 <b>{{ focusMeta.lastDate ? formatDateShort(focusMeta.lastDate) : '' }}</b></span>
          </div>
        </div>
        <!-- 横向时间轴（按住拖拽平移，区分 click/drag）-->
        <div
          ref="focusScrollRef"
          class="btb-focus-timeline-wrap"
          :class="{ 'btb-focus-timeline-wrap--dragging': dragState.down && dragState.moved }"
          @pointerdown="onPointerDown"
          @pointermove="onPointerMove"
          @pointerup="onPointerUp"
          @pointercancel="onPointerCancel"
        >
          <div class="btb-focus-timeline" :style="{ width: focusTimelineWidth + 'px' }">
            <div v-for="n in focusNodes" :key="'fn-' + n.id" class="btb-focus-col">
              <div class="btb-focus-col__date"><b>{{ formatDateShort(n.period_date) }}</b></div>
              <div
                class="btb-focus-node"
                :data-section="n.id"
                :class="{ 'btb-focus-node--selected': selectedNode?.id === n.id }"
                @click="onFocusNodeClick(n)"
              >
                <div class="btb-focus-node__inner" />
              </div>
              <div class="btb-focus-node-label">{{ truncateLabel(n.cluster_label) }}</div>
            </div>
          </div>
        </div>
        <!-- 就地展开 thread（accordion，focus 模式 SHALL NOT 弹 popup overlay）-->
        <div v-if="selectedNode" class="btb-focus-inline">
          <div class="btb-focus-inline__head">
            <span class="btb-focus-inline__date">{{ formatDateShort(selectedNode.period_date) }}</span>
            <span class="btb-focus-inline__title">{{ selectedNode.cluster_label }}</span>
            <span class="btb-focus-inline__meta">{{ selectedNode.article_count }} 篇 · {{ selectedNode.thread_count }} 线索</span>
          </div>
          <div class="btb-threads">
            <div v-if="popupThreadsLoading" class="btb-threads-loading">
              <div v-for="i in 2" :key="i" class="btb-thread-skeleton" />
            </div>
            <div v-else-if="popupThreads.length === 0" class="btb-threads-empty">无关联线索</div>
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
                    <div v-if="(thread.related_article_ids?.length ?? 0) > 10" class="btb-articles-more">
                      还有 {{ (thread.related_article_ids?.length ?? 0) - 10 }} 篇…
                    </div>
                  </template>
                </div>
              </Transition>
            </div>
          </div>
        </div>
      </template>
    </div>

    <!-- Timeline -->
    <div v-else class="btb-chart">
      <!-- SVG canvas: outer spacer owns scaled dimensions; SVG alone is transformed. -->
      <div ref="svgScrollRef" class="btb-svg-scroll">
        <div
          class="btb-svg-zoom"
          :style="{
            width: Math.round(svgWidth * zoomScale) + 'px',
            height: Math.round(svgHeight * zoomScale) + 'px',
          }"
        >
        <svg
          :width="svgWidth"
          :height="svgHeight"
          class="btb-svg"
          :style="{ transform: `scale(${zoomScale})` }"
        >
          <!-- Date labels live inside the same pan/zoom coordinate system. -->
          <text
            v-for="col in dateColumns"
            :key="'date-' + col.date"
            :x="col.x"
            :y="12"
            text-anchor="middle"
            class="btb-date-label"
          >{{ col.label }}</text>

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
            <g
              v-for="(lane, li) in laneRows"
              :key="'lane-' + lane.key"
              class="btb-lane-row"
              :class="{ 'btb-lane-row--enterable': lane.topicId != null }"
              :data-topic="lane.topicId ?? ''"
              @click="onLaneClick(lane)"
            >
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
              <!-- 泳道 hover 操作菜单（重命名 / 归档·恢复 / 删除）；source=manual 与 auto 一视同仁 -->
              <g
                v-if="lane.topicId != null"
                class="btb-lane-ops"
                :data-topic="lane.topicId"
                :transform="`translate(${LANE_LABEL_W - 68}, ${laneOpsY(li) - 9})`"
                @click.stop
              >
                <g class="btb-lane-op" role="button" aria-label="重命名" @click.stop="startRename(lane)">
                  <title>重命名</title>
                  <rect class="btb-lane-op-hit" width="20" height="18" rx="3" />
                  <path class="btb-lane-op-ico" d="M3 14l1.2-3.2 6.3-6.3 2 2-6.3 6.3L3 14z M10.5 6.5l2 2" />
                </g>
                <g class="btb-lane-op" transform="translate(23 0)" role="button"
                   :aria-label="lane.status === 'archived' ? '恢复' : '归档'" @click.stop="startStatusToggle(lane)">
                  <title>{{ lane.status === 'archived' ? '恢复' : '归档' }}</title>
                  <rect class="btb-lane-op-hit" width="20" height="18" rx="3" />
                  <path class="btb-lane-op-ico" d="M3 5h14v3H3z M5.5 8.5v6.5h9V8.5 M9 8.5v6.5" />
                </g>
                <g class="btb-lane-op btb-lane-op--danger" transform="translate(46 0)" role="button"
                   aria-label="删除" @click.stop="startDelete(lane)">
                  <title>删除</title>
                  <rect class="btb-lane-op-hit" width="20" height="18" rx="3" />
                  <path class="btb-lane-op-ico" d="M5 6.5h10l-.8 8H5.8L5 6.5z M8 4.5h4 M4 6.5h12" />
                </g>
              </g>
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
              'btb-dag-node--manual': pn.data.topic_match_confidence === 'manual',
              'btb-dag-node--selected': selectedNode?.id === pn.data.id,
              'btb-dag-node--lineage': isNodeHighlighted(pn.data.id),
              'btb-dag-node--dimmed': hoveredId !== null && !isNodeHighlighted(pn.data.id),
            }"
            @click="selectNode(pn.data)"
            @mouseenter="hoveredId = pn.data.id"
            @mouseleave="hoveredId = null"
          >
            <title>{{ pn.data.cluster_label }}（{{ pn.data.period_date.slice(0, 10) }}）{{ pn.data.persistent_topic ? '\n话题：' + pn.data.persistent_topic.label : '' }}{{ pn.data.topic_match_confidence === 'manual' ? '\n人工归属' : '' }}{{ pn.data.article_count ? '\n' + pn.data.article_count + ' 篇 · ' + pn.data.thread_count + ' 线索' : '' }}</title>
            <template v-if="pn.data.topic_match_confidence === 'manual'">
              <!-- manual confidence：双环描边，独立样式，不套算法三态（task 3.9）-->
              <circle
                :cx="pn.cx"
                :cy="pn.cy"
                :r="selectedNode?.id === pn.data.id ? NODE_R + 2 : NODE_R"
                class="btb-manual-ring"
              />
              <circle
                :cx="pn.cx"
                :cy="pn.cy"
                :r="(selectedNode?.id === pn.data.id ? NODE_R + 2 : NODE_R) - 4"
                class="btb-manual-core"
              />
            </template>
            <circle
              v-else
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
      <div v-if="selectedNode && viewMode !== 'focus'" class="btb-popup-overlay" @click.self="selectedNode = null">
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

  <!-- 泳道操作确认弹窗（迁自 TopicManageDialog；二次确认统一走 AppDialog，禁 window.*） -->
  <!-- 重命名 -->
  <AppDialog
    :model-value="renameTarget !== null"
    title="重命名话题"
    width="440px"
    @update:model-value="(v) => { if (!v) renameTarget = null }"
  >
    <div class="btb-field">
      <label class="btb-field-label">话题名称</label>
      <AppInput v-model="renameLabel" type="text" placeholder="输入新名称" />
    </div>
    <template #footer>
      <AppButton variant="secondary" @click="renameTarget = null">取消</AppButton>
      <AppButton variant="primary" :loading="busyTopicId === renameTarget?.id" @click="confirmRename">保存</AppButton>
    </template>
  </AppDialog>

  <!-- 归档 / 恢复 -->
  <AppDialog
    :model-value="statusTarget !== null"
    :title="statusNext === 'archived' ? '归档话题' : '恢复话题'"
    width="420px"
    @update:model-value="(v) => { if (!v) statusTarget = null }"
  >
    <p class="btb-confirm-body">
      {{ statusNext === 'archived'
        ? `确定归档「${statusTarget?.label}」？归档后不再参与新归属，可在此恢复。`
        : `确定恢复「${statusTarget?.label}」为活跃话题？恢复后将重新参与话题泳道与新归属。` }}
    </p>
    <template #footer>
      <AppButton variant="secondary" @click="statusTarget = null">取消</AppButton>
      <AppButton variant="primary" :loading="busyTopicId === statusTarget?.id" @click="confirmStatusToggle">
        {{ statusNext === 'archived' ? '归档' : '恢复' }}
      </AppButton>
    </template>
  </AppDialog>

  <!-- 删除（输入名称二次确认） -->
  <AppDialog
    :model-value="deleteTarget !== null"
    title="删除话题"
    width="440px"
    @update:model-value="(v) => { if (!v) deleteTarget = null }"
  >
    <div class="btb-field">
      <div class="btb-danger-banner">
        <Icon icon="mdi:alert" width="18" />
        <span>硬删除不可恢复。话题「{{ deleteTarget?.label }}」下的 {{ deleteTarget?.sectionCount }} 条 section 将解除归属但保留内容。</span>
      </div>
      <label class="btb-field-label">输入话题名称「{{ deleteTarget?.label }}」以确认</label>
      <AppInput v-model="deleteConfirmText" type="text" placeholder="话题名称" />
    </div>
    <template #footer>
      <AppButton variant="secondary" @click="deleteTarget = null">取消</AppButton>
      <AppButton variant="danger" :disabled="!deleteCanConfirm" :loading="busyTopicId === deleteTarget?.id" @click="confirmDelete">删除</AppButton>
    </template>
  </AppDialog>

  <!-- 合并预览 -->
  <AppDialog
    :model-value="mergeOpen"
    title="合并话题"
    width="480px"
    @update:model-value="(v) => { mergeOpen = v; if (!v) { mergeSourceId = null; mergeTargetId = null } }"
  >
    <p class="btb-confirm-body">选择源话题（将被归档、其 section 改指目标）与目标话题（保留）。</p>
    <div class="btb-merge-cols">
      <div class="btb-merge-col">
        <div class="btb-merge-col__t">源话题</div>
        <div v-if="mergeLoading" class="btb-merge-empty">加载中…</div>
        <button
          v-for="t in mergeAllTopics"
          v-else
          :key="'ms-' + t.id"
          type="button"
          class="btb-merge-item"
          :class="{ 'btb-merge-item--sel': mergeSourceId === t.id }"
          @click="mergeSourceId = t.id"
        >
          <span class="btb-merge-dot" :style="{ background: t.color }" />
          <span class="btb-merge-label">{{ t.label }}</span>
          <span class="btb-merge-count">{{ t.section_count }} 条</span>
        </button>
      </div>
      <div class="btb-merge-col">
        <div class="btb-merge-col__t">目标话题</div>
        <div v-if="mergeLoading" class="btb-merge-empty">加载中…</div>
        <button
          v-for="t in mergeCandidates"
          v-else
          :key="'mt-' + t.id"
          type="button"
          class="btb-merge-item"
          :class="{ 'btb-merge-item--sel': mergeTargetId === t.id }"
          @click="mergeTargetId = t.id"
        >
          <span class="btb-merge-dot" :style="{ background: t.color }" />
          <span class="btb-merge-label">{{ t.label }}</span>
          <span class="btb-merge-count">{{ t.section_count }} 条</span>
        </button>
        <div v-if="!mergeLoading && mergeCandidates.length === 0" class="btb-merge-empty">无其他可合并话题</div>
      </div>
    </div>
    <template #footer>
      <AppButton variant="secondary" @click="mergeOpen = false">取消</AppButton>
      <AppButton
        variant="primary"
        :disabled="mergeSourceId === null || mergeTargetId === null"
        :loading="busyTopicId === mergeSourceId"
        @click="confirmMerge"
      >确认合并</AppButton>
    </template>
  </AppDialog>
</template>

<style scoped>
.btb-container {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  /* 工作台化：去除悬浮卡片 chrome（修悬浮留白），擑满父容器宽高 */
  margin-top: 0;
  padding: 0.6rem 0.75rem;
  border-radius: 0;
  border: 0;
  background: transparent;
}

/* Controls */
.btb-controls {
  display: flex;
  align-items: center;
  justify-content: flex-start;
  gap: 0.5rem;
}

.btb-controls-actions {
  display: flex;
  align-items: center;
  justify-content: flex-end;
  flex-wrap: wrap;
  gap: 0.5rem;
  margin-left: auto;
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
  cursor: not-allowed;
}
.btb-topic-op--confirm:not(:disabled) {
  border-color: rgba(52, 211, 153, 0.6);
  color: #6ee7b7;
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
  position: relative;
}

.btb-date-label {
  font-size: 0.6rem;
  fill: var(--color-text-muted);
  pointer-events: none;
}

/* SVG scroll container（缩放后两向可滚动） */
.btb-svg-scroll {
  overflow: auto;
  position: relative;
  /* 占满工作台主体高度：接近 content 高度；泳道多时纵向滚动 */
  height: calc(100vh - 200px);
  min-height: 320px;
  max-height: none;
}

.btb-svg-zoom {
  position: relative;
}

.btb-svg {
  display: block;
  transform-origin: 0 0;
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
  font-size: 10px;
  font-weight: 500;
  pointer-events: none;
  opacity: 0.82;
  paint-order: stroke;
  stroke: var(--color-bg-hover);
  stroke-width: 3px;
  stroke-linejoin: round;
  transition: opacity 0.12s ease;
}

@media (max-width: 900px) {
  .btb-controls {
    align-items: flex-start;
  }
  .btb-controls-actions {
    gap: 0.35rem;
  }
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

/* manual confidence 节点：双环描边（task 3.9），独立于算法三态（实心/虚线/空心）。
   hover 显示"人工归属"由 <title> 提供；跟随双主题用语义 token。 */
.btb-dag-node--manual .btb-manual-ring {
  fill: var(--color-bg-base);
  stroke: var(--color-accent);
  stroke-width: 2.5;
}
.btb-dag-node--manual .btb-manual-core {
  fill: var(--color-accent);
  stroke: none;
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

/* ===== Focus view (slice C′) — single-topic deep read — all semantic tokens ===== */
.btb-focus {
  display: flex;
  flex-direction: column;
  gap: 0.8rem;
  padding: 0.5rem 0 1rem;
}
.btb-focus-head {
  position: sticky;
  top: 0;
  z-index: 5;
  background: var(--color-bg-base);
  border-bottom: 1px solid var(--color-border-medium);
  padding: 0.8rem 1rem;
}
.btb-focus-head__top {
  display: flex;
  align-items: center;
  gap: 0.6rem;
}
.btb-focus-back {
  appearance: none;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-elevated);
  color: var(--color-text-secondary);
  padding: 0.32rem 0.7rem;
  border-radius: 6px;
  font-size: 0.74rem;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-family: inherit;
}
.btb-focus-back:hover {
  border-color: var(--color-accent);
  color: var(--color-accent);
}
.btb-focus-status {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.66rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--color-accent);
  padding: 0.18rem 0.5rem;
  border: 1px solid var(--color-accent);
  border-radius: 10px;
  background: var(--color-accent-subtle);
}
.btb-focus-status__dot {
  width: 6px;
  height: 6px;
  border-radius: 50%;
  background: var(--color-accent);
}
.btb-focus-title {
  margin: 0.7rem 0 0;
  font-size: 1.6rem;
  font-weight: 800;
  line-height: 1.2;
  color: var(--color-text-primary);
}
.btb-focus-meta {
  margin-top: 0.4rem;
  display: flex;
  flex-wrap: wrap;
  gap: 1.1rem;
  color: var(--color-text-muted);
  font-size: 0.72rem;
}
.btb-focus-meta b {
  color: var(--color-text-secondary);
  font-size: 0.82rem;
}
.btb-focus-timeline-wrap {
  position: relative;
  overflow-x: auto;
  overflow-y: hidden;
  cursor: grab;
  user-select: none;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
}
.btb-focus-timeline-wrap--dragging {
  cursor: grabbing;
}
.btb-focus-timeline {
  display: flex;
  align-items: flex-start;
  padding: 0.8rem 1rem;
}
.btb-focus-col {
  display: flex;
  flex-direction: column;
  align-items: center;
  width: 96px;
  flex: 0 0 96px;
  position: relative;
}
.btb-focus-col__date {
  font-size: 0.72rem;
  color: var(--color-text-muted);
  padding-bottom: 0.7rem;
}
.btb-focus-col__date b {
  display: block;
  color: var(--color-text-secondary);
  font-size: 0.95rem;
}
/* 话题延续主线（竖向，穿过节点）*/
.btb-focus-col::after {
  content: '';
  position: absolute;
  left: 50%;
  top: 3.4rem;
  bottom: 0.4rem;
  width: 2px;
  transform: translateX(-50%);
  background: linear-gradient(180deg, var(--color-accent), transparent);
  opacity: 0.45;
}
.btb-focus-node {
  position: relative;
  z-index: 1;
  width: 40px;
  height: 40px;
  border-radius: 50%;
  background: var(--color-bg-base);
  border: 3px solid var(--color-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  cursor: pointer;
  transition: transform 0.15s, background 0.15s;
}
.btb-focus-node:hover {
  transform: scale(1.08);
}
.btb-focus-node__inner {
  width: 16px;
  height: 16px;
  border-radius: 50%;
  background: var(--color-accent);
  opacity: 0.85;
}
.btb-focus-node--selected {
  background: var(--color-accent);
  box-shadow: 0 0 0 4px var(--color-accent-subtle);
}
.btb-focus-node--selected .btb-focus-node__inner {
  background: var(--color-bg-base);
  opacity: 1;
}
.btb-focus-node-label {
  margin-top: 0.5rem;
  font-size: 0.72rem;
  line-height: 1.35;
  color: var(--color-text-secondary);
  max-width: 92px;
  text-align: center;
}
.btb-focus-node--selected ~ .btb-focus-node-label {
  color: var(--color-accent);
  font-weight: 700;
}
.btb-focus-inline {
  padding: 0.8rem 1rem;
  background: var(--color-bg-elevated);
  border: 1px solid var(--color-border-subtle);
  border-radius: 10px;
}
.btb-focus-inline__head {
  display: flex;
  align-items: baseline;
  gap: 0.8rem;
  padding-bottom: 0.6rem;
  border-bottom: 1px solid var(--color-border-subtle);
  margin-bottom: 0.6rem;
}
.btb-focus-inline__date {
  color: var(--color-text-muted);
  font-size: 0.74rem;
}
.btb-focus-inline__title {
  color: var(--color-text-primary);
  font-weight: 700;
}
.btb-focus-inline__meta {
  margin-left: auto;
  color: var(--color-text-muted);
  font-size: 0.72rem;
}
.btb-focus-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.6rem;
  padding: 2rem 1rem;
  color: var(--color-text-muted);
  text-align: center;
}
.btb-focus-empty p {
  margin: 0;
}
.btb-lane-row--enterable {
  cursor: pointer;
}

/* ===== 工作台工具条按钮（回刷 / 合并 / 新建，全语义 token）===== */
.btb-tool-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.28rem 0.6rem;
  font-size: 0.7rem;
  border: 1px solid var(--color-border-medium);
  border-radius: 6px;
  background: var(--color-bg-elevated);
  color: var(--color-text-secondary);
  cursor: pointer;
  transition: background 0.12s ease, color 0.12s ease, border-color 0.12s ease;
}
.btb-tool-btn:hover:not(:disabled) {
  border-color: var(--color-border-strong);
  color: var(--color-text-primary);
  background: var(--color-bg-hover);
}
.btb-tool-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.btb-tool-btn--primary {
  background: var(--color-accent);
  border-color: var(--color-accent);
  color: var(--color-text-inverted);
  font-weight: 600;
}
.btb-tool-btn--primary:hover:not(:disabled) {
  background: var(--color-accent-hover);
  border-color: var(--color-accent-hover);
  color: var(--color-text-inverted);
}

/* 操作错误条 */
.btb-ops-error {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.7rem;
  font-size: 0.72rem;
  color: var(--color-error);
  background: var(--color-bg-active);
  border: 1px solid var(--color-error);
  border-radius: 6px;
}
.btb-ops-error-close {
  margin-left: auto;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 0.8rem;
}

/* 泳道 hover 操作菜单（SVG 内，hover 显现；默认隐藏不占交互） */
.btb-lane-ops {
  opacity: 0;
  transition: opacity 0.12s ease;
}
.btb-lane-row:hover .btb-lane-ops {
  opacity: 1;
}
.btb-lane-op {
  cursor: pointer;
}
.btb-lane-op-hit {
  fill: transparent;
}
.btb-lane-op:hover .btb-lane-op-hit {
  fill: var(--color-bg-hover);
}
.btb-lane-op-ico {
  fill: none;
  stroke: var(--color-text-muted);
  stroke-width: 1.4;
  stroke-linecap: round;
  stroke-linejoin: round;
  pointer-events: none;
}
.btb-lane-op:hover .btb-lane-op-ico {
  stroke: var(--color-text-primary);
}
.btb-lane-op--danger:hover .btb-lane-op-ico {
  stroke: var(--color-error);
}

/* 弹窗内表单/确认样式（复用 TopicManageDialog 视觉约定，全语义 token） */
.btb-field {
  display: flex;
  flex-direction: column;
  gap: 0.5rem;
}
.btb-field-label {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}
.btb-confirm-body {
  font-size: 0.84rem;
  color: var(--color-text-primary);
  line-height: 1.5;
  margin: 0;
}
.btb-danger-banner {
  display: flex;
  gap: 0.5rem;
  padding: 0.55rem 0.7rem;
  background: var(--color-bg-active);
  border: 1px solid var(--color-error);
  border-radius: 8px;
  color: var(--color-error);
  font-size: 0.76rem;
  line-height: 1.5;
}

/* 合并预览双栏 */
.btb-merge-cols {
  display: grid;
  grid-template-columns: 1fr 1fr;
  gap: 0.75rem;
  margin-top: 0.5rem;
}
.btb-merge-col {
  display: flex;
  flex-direction: column;
  gap: 0.25rem;
  max-height: 40vh;
  overflow-y: auto;
}
.btb-merge-col__t {
  font-size: 0.72rem;
  font-weight: 600;
  color: var(--color-text-secondary);
  padding-bottom: 0.25rem;
}
.btb-merge-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.4rem 0.55rem;
  border: 1px solid var(--color-border-subtle);
  border-radius: 6px;
  background: transparent;
  cursor: pointer;
  text-align: left;
}
.btb-merge-item:hover {
  background: var(--color-bg-hover);
}
.btb-merge-item--sel {
  border-color: var(--color-accent);
  background: var(--color-accent-subtle);
}
.btb-merge-dot {
  width: 9px;
  height: 9px;
  border-radius: 50%;
  flex-shrink: 0;
}
.btb-merge-label {
  flex: 1;
  font-size: 0.78rem;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.btb-merge-count {
  font-size: 0.66rem;
  color: var(--color-text-muted);
}
.btb-merge-empty {
  padding: 0.75rem;
  text-align: center;
  font-size: 0.74rem;
  color: var(--color-text-muted);
}
</style>
