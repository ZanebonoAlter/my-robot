<script setup lang="ts">
/**
 * 手动建泳道编排态（切片③ tasks 3.2-3.5, 3.8）。
 *
 * 三栏布局（参考 mockups/topic-workbench.html 编排态）：
 *  - 顶：编排态工具条（返回 + 「新建·待保存」徽标 + 泳道名输入 + 保存/取消）
 *  - 中：预览泳道时间轴（实时反映勾选，节点三态，按住拖拽平移）
 *  - 底左：候选 section 池（多选 + 距离标签 + 原属话题 + 离群标黄）
 *  - 底右：体检报告三卡（聚类质量 / 撞车检查 / 未来预期淡显）
 *
 * 纯逻辑见 composeReport.ts（实时算聚合锚点/离群/撞车），API 见
 * persistentTopics.ts。全语义 token，跟随双主题；统一组件库 AppInput/AppButton/AppDialog。
 */
import { ref, computed, watch, onMounted, onUnmounted } from 'vue'
import { Icon } from '@iconify/vue'
import { usePersistentTopicsApi, type ComposeCandidate } from '~/api/persistentTopics'
import { useDailyReportsApi, type DailyReportThread } from '~/api/dailyReports'
import {
  aggregatePreview, cosineDistance, crashReport, distanceTier, filterPoolByRange,
  outlierFlags, rankCandidates, TIER_LABEL, type DistanceTier,
} from './composeReport'
import { isDragMove } from './topicFocus'

/** 引导区展示的候选话题（父组件从 board topics 过滤 status=candidate 传入）。 */
interface CandidateTopicGuide {
  id: number
  label: string
  hitCount: number
  consecutiveHits: number
  canActivate: boolean
  sectionCount: number
}

const props = defineProps<{
  boardId: number | string
  days: number
  candidateTopics?: CandidateTopicGuide[]
}>()
const emit = defineEmits<{
  saved: []
  cancel: []
  'activate-candidate': [topicId: number]
}>()

const { getComposeCandidates, createManualLane, embedQuery } = usePersistentTopicsApi()
const { getDailyReportDetail } = useDailyReportsApi()

// ── state ───────────────────────────────────────────────────────────────────
const loading = ref(false)
const loadError = ref<string | null>(null)
const candidates = ref<ComposeCandidate[]>([])
const matchThreshold = ref(0.3)
const selectedIds = ref<Set<string>>(new Set())
// 已勾选 section 的「查看线索」展开态（单选；null=收起）+ 缓存 + 加载/错误态。
const expandedCand = ref<string | null>(null)
const candThreads = ref<Map<string, DailyReportThread[]>>(new Map())
const candThreadsLoading = ref(false)
const candThreadsError = ref<string | null>(null)
const laneLabel = ref('')
const saving = ref(false)
const saveError = ref<string | null>(null)
// 撞车确认弹窗（spec Scenario「撞车明确提示移出」）：moveOutCount>0 时保存前二次确认。
const crashConfirmOpen = ref(false)

// ── 语义搜索（task 3.13）：文本→embedding 冷启动，勾选后聚合锡点接管排序 ──
const searchText = ref('')
const queryVec = ref<number[] | null>(null)
const searching = ref(false)
const searchError = ref<string | null>(null)
let searchTimer: ReturnType<typeof setTimeout> | null = null

// ── data load ───────────────────────────────────────────────────────────────
async function load() {
  loading.value = true
  loadError.value = null
  const res = await getComposeCandidates(props.boardId, props.days)
  loading.value = false
  if (res.success && res.data) {
    candidates.value = res.data.sections
    matchThreshold.value = res.data.matchThreshold
  } else {
    candidates.value = []
    loadError.value = res.error || '加载候选失败'
  }
}
onMounted(load)
watch(() => [props.boardId, props.days], load)

// ── derived: pool / aggregate / distances ───────────────────────────────────
// 默认序：periodDate 倒序（最新在前），作为无搜索信号时的回退序
// （spec Scenario「清空回退默认排序」）。前端窗口兜底与后端一致（days=0 全部）。
const pool = computed(() => {
  const filtered = filterPoolByRange(candidates.value, props.days)
  return [...filtered].sort((a, b) => Date.parse(b.periodDate) - Date.parse(a.periodDate))
})

const selectedCandidates = computed(() =>
  pool.value.filter(c => selectedIds.value.has(c.id)),
)

// 聚合锚点 = 选中向量 mean pooling（design §4.2）。空选 → mean=null。
const aggregate = computed(() =>
  aggregatePreview(selectedCandidates.value.map(c => c.embedding)),
)

// 每个候选到锚点的距离（未选中也算，用于距离标签）。无锚点 → undefined。
const distanceById = computed(() => {
  const m = aggregate.value.mean
  const map = new Map<string, number>()
  if (!m) return map
  for (const c of pool.value) map.set(c.id, cosineDistance(c.embedding, m))
  return map
})

const tierById = computed(() => {
  const map = new Map<string, DistanceTier>()
  for (const c of pool.value) {
    const d = distanceById.value.get(c.id)
    map.set(c.id, d == null ? 'far' : distanceTier(d, matchThreshold.value))
  }
  return map
})

// 离群集合（基于选中项的距离；outlierFlags 只对 selected 有意义，因为锚点由 selected 决定）。
const outlierSelectedIds = computed(() => {
  const sel = selectedCandidates.value
  const dists = sel.map(c => distanceById.value.get(c.id) ?? Number.POSITIVE_INFINITY)
  const flags = outlierFlags(dists, matchThreshold.value)
  if (!flags) return new Set<string>()
  const out = new Set<string>()
  sel.forEach((c, i) => { if (flags[i]) out.add(c.id) })
  return out
})

// 选中项的平均两两距离（聚类质量指标）。
const avgPairwiseDistance = computed(() => {
  const sel = selectedCandidates.value
  if (sel.length < 2) return 0
  let sum = 0, pairs = 0
  for (let i = 0; i < sel.length; i++) {
    for (let j = i + 1; j < sel.length; j++) {
      sum += cosineDistance(sel[i]!.embedding, sel[j]!.embedding)
      pairs++
    }
  }
  return pairs > 0 ? sum / pairs : 0
})

// ── 渐进收敛排序（task 3.11/3.13）：已选聚合锡点优先（用户确证信号），
// 退回文本查询向量（冷启动种子），两者皆空时 rankCandidates 保持 pool 默认序。 ─
const rankedPool = computed(() =>
  rankCandidates(pool.value, aggregate.value.mean, queryVec.value),
)
const hasRankSignal = computed(() => aggregate.value.mean != null || queryVec.value != null)
// 已选置顶分组：已选的保持勾选态方便取消；未选的按命中率排序在其下。
const rankedSelected = computed(() => rankedPool.value.filter(c => selectedIds.value.has(c.id)))
const rankedUnselected = computed(() => rankedPool.value.filter(c => !selectedIds.value.has(c.id)))

type DisplayItem =
  | { kind: 'header', label: string, muted?: boolean }
  | { kind: 'cand', cand: ComposeCandidate }
const displayList = computed<DisplayItem[]>(() => {
  const out: DisplayItem[] = []
  if (rankedSelected.value.length > 0) {
    out.push({ kind: 'header', label: `已选 ${rankedSelected.value.length}` })
    for (const c of rankedSelected.value) out.push({ kind: 'cand', cand: c })
  }
  if (rankedUnselected.value.length > 0) {
    if (hasRankSignal.value) out.push({ kind: 'header', label: '候选 · 按命中率', muted: true })
    for (const c of rankedUnselected.value) out.push({ kind: 'cand', cand: c })
  }
  return out
})

// ── preview timeline: selected grouped by date ─────────────────────────────
interface PreviewCol { date: string, label: string, nodes: ComposeCandidate[] }
const previewCols = computed<PreviewCol[]>(() => {
  const byDate = new Map<string, ComposeCandidate[]>()
  for (const c of selectedCandidates.value) {
    const d = c.periodDate.slice(0, 10)
    if (!byDate.has(d)) byDate.set(d, [])
    byDate.get(d)!.push(c)
  }
  return [...byDate.entries()]
    .sort((a, b) => (a[0] < b[0] ? -1 : 1))
    .map(([date, nodes]) => ({ date, label: formatColDate(date), nodes }))
})

// 预览节点三态：good=实心 / boundary=虚线 / outlier=黄框（far 选中时按 outlier 黄框展示）。
function previewTier(id: string): 'good' | 'boundary' | 'outlier' {
  const t = tierById.value.get(id) ?? 'far'
  return t === 'good' ? 'good' : (t === 'boundary' ? 'boundary' : 'outlier')
}

// ── candidate pool existing topics (for crash report labels) ────────────────
const existingTopics = computed(() => {
  const map = new Map<string, string>()
  for (const c of candidates.value) {
    if (c.persistentTopic) map.set(c.persistentTopic.id, c.persistentTopic.label)
  }
  return [...map.entries()].map(([id, label]) => ({ id, label }))
})

const crash = computed(() => crashReport(selectedCandidates.value, existingTopics.value))

// 最近现有话题距离：锚点到"未选中但有归属"的候选的最小距离（撞车信号）。
const nearestExistingDistance = computed(() => {
  const m = aggregate.value.mean
  if (!m) return null
  let min = Number.POSITIVE_INFINITY
  for (const c of pool.value) {
    if (selectedIds.value.has(c.id)) continue
    if (!c.persistentTopicId) continue
    const d = cosineDistance(c.embedding, m)
    if (d < min) min = d
  }
  return Number.isFinite(min) ? min : null
})

// ── interactions ────────────────────────────────────────────────────────────
// 语义搜索（debounce ~450ms）：文本→embedQuery，失败降级（spec「搜索失败不阻断」）。
// timer ref + onUnmounted 清理，避免组件卸载后回调仍写 state。
watch(searchText, (val) => {
  if (searchTimer) { clearTimeout(searchTimer); searchTimer = null }
  const trimmed = val.trim()
  if (!trimmed) {
    queryVec.value = null
    searching.value = false
    searchError.value = null
    return
  }
  searching.value = true
  searchError.value = null
  const target = trimmed
  searchTimer = setTimeout(async () => {
    const res = await embedQuery(props.boardId, target)
    // 仅在查询仍是当前文本时采纳（防乱序覆盖：a→ab，a 的晚到结果不应覆盖 ab）。
    if (searchText.value.trim() !== target) return
    searching.value = false
    if (res.success && res.data && res.data.embedding.length > 0) {
      queryVec.value = res.data.embedding
      searchError.value = null
    } else {
      queryVec.value = null
      searchError.value = res.error || '搜索失败，已回退默认排序'
    }
  }, 450)
})
onUnmounted(() => {
  if (searchTimer) { clearTimeout(searchTimer); searchTimer = null }
})

function toggle(id: string) {
  const next = new Set(selectedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  selectedIds.value = next
}

// 已勾选 section 就地展开看线索（spec: 编排态已勾选 section 查看线索）。
// 复用 getDailyReportDetail(report_id) → report.sections.find(id).threads。
async function toggleCandThreads(cand: ComposeCandidate) {
  if (expandedCand.value === cand.id) {
    expandedCand.value = null
    return
  }
  expandedCand.value = cand.id
  candThreadsError.value = null
  if (candThreads.value.has(cand.id)) return // 已缓存，直接显示
  if (!cand.reportId) return
  candThreadsLoading.value = true
  try {
    const res = await getDailyReportDetail(Number(cand.reportId))
    if (res.success && res.data) {
      const section = res.data.report.sections?.find(s => s.id === Number(cand.id))
      candThreads.value = new Map(candThreads.value).set(cand.id, section?.threads ?? [])
    } else {
      candThreadsError.value = res.error || '加载线索失败'
    }
  } finally {
    candThreadsLoading.value = false
  }
}

// 取消勾选时收起其线索区（避免展开态悬空）。
watch(selectedIds, (ids) => {
  if (expandedCand.value != null && !ids.has(expandedCand.value)) {
    expandedCand.value = null
  }
})

// ── 候选话题引导区（迁移自 TopicManageDialog）：候选的一键激活/并入 ──
// 分两组：可激活（累计 hit_count≥阈值，can_activate）+ 观察中（未达标）。
// 门禁口径为累计命中次数（hit_count），非连续天数——话题有间隔也能累计达标。
const activatableCandidates = computed(() => {
  const list = (props.candidateTopics ?? []).filter(t => t.canActivate)
  return [...list].sort((a, b) => b.hitCount - a.hitCount)
})
const observingCandidates = computed(() => {
  const list = (props.candidateTopics ?? []).filter(t => !t.canActivate)
  return [...list].sort((a, b) => b.hitCount - a.hitCount)
})
const hasAnyCandidate = computed(() => activatableCandidates.value.length > 0 || observingCandidates.value.length > 0)
const activatingId = ref<number | null>(null)
// 激活成功后父组件刷新，该候选变 active 从列表消失 → 清除 loading 态。
watch([activatableCandidates, observingCandidates], () => {
  const all = [...activatableCandidates.value, ...observingCandidates.value]
  if (activatingId.value !== null && !all.some(t => t.id === activatingId.value)) {
    activatingId.value = null
  }
})

// 「并入新泳道」：把候选池中归属该候选的 section 全部选中（纯前端，零 API）。
function mergeCandidateIntoLane(topicId: number) {
  const tid = String(topicId)
  const next = new Set(selectedIds.value)
  for (const c of pool.value) {
    if (c.persistentTopicId === tid) next.add(c.id)
  }
  selectedIds.value = next
}

// 「确认启用」：交父组件调 updateTopic(status→active) + 刷新；编排态不退场。
function activateCandidate(topicId: number) {
  if (activatingId.value !== null) return
  activatingId.value = topicId
  emit('activate-candidate', topicId)
}

// 一键剔除离群（design §4.4：用户主动点，不自动删）。
function kickOutliers() {
  const next = new Set(selectedIds.value)
  for (const id of outlierSelectedIds.value) next.delete(id)
  selectedIds.value = next
}

// 撞车时保存前二次确认；无撞车直接保存。
function requestSave() {
  saveError.value = null
  if (selectedCandidates.value.length === 0) {
    saveError.value = '请至少勾选一条 section'
    return
  }
  if (!laneLabel.value.trim()) {
    saveError.value = '请输入泳道名称'
    return
  }
  if (crash.value.moveOutCount > 0) {
    crashConfirmOpen.value = true
    return
  }
  void doSave()
}

async function doSave() {
  crashConfirmOpen.value = false
  saving.value = true
  saveError.value = null
  const res = await createManualLane(props.boardId, laneLabel.value.trim(), [...selectedIds.value])
  saving.value = false
  if (res.success) {
    emit('saved')
  } else {
    saveError.value = res.error || '保存失败'
  }
}

// ── preview timeline drag-pan（区分 click/drag 阈值 3px，参照 focus 视图）───
const scrollRef = ref<HTMLDivElement | null>(null)
const drag = ref({ down: false, startX: 0, startScroll: 0, moved: false })
const DRAG_THRESHOLD = 3
function onPointerDown(e: PointerEvent) {
  const el = scrollRef.value
  if (!el) return
  if ((e.target as Element)?.closest?.('.cp-node')) return
  drag.value = { down: true, startX: e.clientX, startScroll: el.scrollLeft, moved: false }
  el.setPointerCapture?.(e.pointerId)
}
function onPointerMove(e: PointerEvent) {
  if (!drag.value.down) return
  const dx = e.clientX - drag.value.startX
  if (isDragMove(dx, DRAG_THRESHOLD)) drag.value.moved = true
  const el = scrollRef.value
  if (el) el.scrollLeft = drag.value.startScroll - dx
}
function endDrag() { drag.value.down = false; drag.value.moved = false }

// ── format helpers ──────────────────────────────────────────────────────────
function formatColDate(date: string): string {
  const d = new Date(`${date}T00:00:00Z`)
  const mm = String(d.getUTCMonth() + 1).padStart(2, '0')
  const dd = String(d.getUTCDate()).padStart(2, '0')
  return `${mm}-${dd}`
}
function fmtDist(d: number | undefined | null): string {
  if (d == null || !Number.isFinite(d)) return '—'
  return d.toFixed(2)
}
</script>

<template>
  <div class="cp-root">
    <!-- 编排态工具条（task 3.2）-->
    <div class="cp-bar">
      <button class="cp-back" title="返回总览" @click="emit('cancel')">
        <Icon icon="mdi:arrow-left" width="13" />
        返回总览
      </button>
      <span class="cp-badge"><span class="cp-badge__dot" />新建 · 待保存</span>
      <AppInput
        v-model="laneLabel"
        type="text"
        placeholder="输入新泳道名称（如：美伊博弈）"
        class="cp-name"
      />
      <span class="cp-src">将保存为 Active · 来源：手动串联</span>
      <span class="cp-bar-sep" />
      <AppButton variant="secondary" @click="emit('cancel')">取消</AppButton>
      <AppButton variant="primary" :loading="saving" @click="requestSave">保存为新泳道</AppButton>
    </div>

    <!-- 候选话题引导区（迁移自 TopicManageDialog）：候选的一键激活/并入，独立于候选池窗口 -->
    <div v-if="hasAnyCandidate" class="cp-guide">
      <div class="cp-guide__head">
        <Icon icon="mdi:ribbon-star-outline" width="14" />
        <b>候选话题</b>
        <span class="cp-hint">按累计命中，达标可启用或并入新泳道</span>
      </div>
      <!-- 可激活（累计命中达门禁） -->
      <div v-if="activatableCandidates.length" class="cp-guide__list">
        <div class="cp-guide__grp">可激活</div>
        <div
          v-for="ct in activatableCandidates"
          :key="ct.id"
          class="cp-guide__item cp-guide__item--ready"
        >
          <div class="cp-guide__main">
            <span class="cp-guide__label" :title="ct.label">{{ ct.label }}</span>
            <span class="cp-guide__meta">累计命中 {{ ct.hitCount }} 次 · 含 {{ ct.sectionCount }} 条</span>
          </div>
          <div class="cp-guide__ops">
            <AppButton
              variant="primary"
              size="sm"
              :loading="activatingId === ct.id"
              title="人工确认后转 Active 进入泳道"
              @click="activateCandidate(ct.id)"
            >确认启用</AppButton>
            <AppButton variant="ghost" size="sm" @click="mergeCandidateIntoLane(ct.id)">并入新泳道</AppButton>
          </div>
        </div>
      </div>
      <!-- 观察中（累计未达门禁）：仅可并入 -->
      <div v-if="observingCandidates.length" class="cp-guide__list">
        <div class="cp-guide__grp cp-guide__grp--muted">观察中 · 累计未达标</div>
        <div
          v-for="ct in observingCandidates"
          :key="ct.id"
          class="cp-guide__item cp-guide__item--muted"
        >
          <div class="cp-guide__main">
            <span class="cp-guide__label" :title="ct.label">{{ ct.label }}</span>
            <span class="cp-guide__meta">累计命中 {{ ct.hitCount }} 次 · 含 {{ ct.sectionCount }} 条</span>
          </div>
          <div class="cp-guide__ops">
            <AppButton variant="ghost" size="sm" @click="mergeCandidateIntoLane(ct.id)">并入新泳道</AppButton>
          </div>
        </div>
      </div>
    </div>

    <!-- 加载/错误 -->
    <div v-if="loading" class="cp-loading">加载候选中…</div>
    <div v-else-if="loadError" class="cp-error" role="alert">{{ loadError }}</div>
    <div v-else-if="pool.length === 0" class="cp-empty">
      当前时间窗口内暂无可串联的 section（需有 embedding）。
    </div>

    <template v-else>
      <!-- 预览泳道时间轴（task 3.3）-->
      <div
        ref="scrollRef"
        class="cp-tl-wrap"
        :class="{ 'cp-tl-wrap--dragging': drag.down && drag.moved }"
        @pointerdown="onPointerDown"
        @pointermove="onPointerMove"
        @pointerup="endDrag"
        @pointercancel="endDrag"
      >
        <div class="cp-tl">
          <div v-if="previewCols.length === 0" class="cp-tl-placeholder">
            勾选左侧 section → 这里实时预览新泳道
          </div>
          <div v-for="col in previewCols" :key="col.date" class="cp-col">
            <div class="cp-col__date"><b>{{ col.label }}</b></div>
            <div v-for="n in col.nodes" :key="n.id" class="cp-node-grp">
              <div class="cp-node" :class="'cp-node--' + previewTier(n.id)" :data-tier="previewTier(n.id)">
                <i />
              </div>
              <div class="cp-nlabel">{{ n.clusterLabel }}</div>
            </div>
          </div>
        </div>
      </div>
      <div class="cp-tl-hint">
        ← 拖拽查看更多日期 · 节点三态（实心=贴合 · 虚线=边界 · 黄框=离群建议剔除）→
      </div>

      <!-- 双栏：候选池 + 体检 -->
      <div class="cp-cols">
        <!-- 候选 section 池（task 3.4）-->
        <div class="cp-col-l">
          <div class="cp-sub">
            <Icon icon="mdi:format-list-checkbox" width="14" />
            <b>候选 section</b>
            <span class="cp-hint">{{ selectedCandidates.length }}/{{ pool.length }} 已选</span>
          </div>
          <!-- 语义搜索（task 3.13）：冷启动种子，勾选后聚合锡点接管排序 -->
          <div class="cp-search">
            <Icon icon="mdi:magnify" width="14" class="cp-search__icon" />
            <AppInput
              v-model="searchText"
              type="text"
              placeholder="输入关键词找相关的（如：半导体、美伊博弈）"
              class="cp-search__input"
            />
            <Icon v-if="searching" icon="mdi:loading" width="14" class="cp-search__spin" />
          </div>
          <div v-if="searchError" class="cp-search-err" role="alert">{{ searchError }}</div>
          <div class="cp-cand-list">
            <template v-for="(item, i) in displayList" :key="item.kind === 'header' ? `h${i}` : item.cand.id">
              <div
                v-if="item.kind === 'header'"
                class="cp-cand-grp__t"
                :class="{ 'cp-cand-grp__t--muted': item.muted }"
              >{{ item.label }}</div>
              <div v-else class="cp-cand-cell">
                <button
                  type="button"
                  class="cp-cand"
                  :class="{
                    'cp-cand--checked': selectedIds.has(item.cand.id),
                    'cp-cand--outlier': selectedIds.has(item.cand.id) && outlierSelectedIds.has(item.cand.id),
                  }"
                  :data-cand-id="item.cand.id"
                  @click="toggle(item.cand.id)"
                >
                  <span class="cp-cand__box">
                    <Icon v-if="selectedIds.has(item.cand.id)" icon="mdi:check" width="11" />
                  </span>
                  <span class="cp-cand__body">
                    <span class="cp-cand__title">{{ item.cand.clusterLabel }}</span>
                    <span class="cp-cand__sub">
                      <span
                        class="cp-cand__dist"
                        :class="'cp-cand__dist--' + (tierById.get(item.cand.id) ?? 'far')"
                      >↳ {{ fmtDist(distanceById.get(item.cand.id)) }} · {{ TIER_LABEL[tierById.get(item.cand.id) ?? 'far'] }}</span>
                      <span v-if="item.cand.persistentTopic" class="cp-cand__tag">原属：{{ item.cand.persistentTopic.label }}</span>
                      <span v-if="selectedIds.has(item.cand.id) && outlierSelectedIds.has(item.cand.id)" class="cp-cand__warn">⚠ 建议剔除</span>
                    </span>
                  </span>
                </button>
                <!-- 已勾选 section 才显示「查看线索」入口（spec: 编排态已勾选 section 查看线索）-->
                <div v-if="selectedIds.has(item.cand.id)" class="cp-cand-threads">
                  <button
                    type="button"
                    class="cp-cand-threads__toggle"
                    :data-cand-threads="item.cand.id"
                    @click="toggleCandThreads(item.cand)"
                  >
                    <Icon icon="mdi:format-list-bulleted" width="12" />
                    {{ expandedCand === item.cand.id ? '收起线索' : '查看线索' }}
                  </button>
                  <div v-if="expandedCand === item.cand.id" class="cp-cand-threads__body">
                    <div v-if="candThreadsLoading && !candThreads.has(item.cand.id)" class="cp-cand-threads__hint">加载中…</div>
                    <div v-else-if="candThreadsError" class="cp-cand-threads__hint cp-cand-threads__hint--err">{{ candThreadsError }}</div>
                    <template v-else>
                      <div v-if="(candThreads.get(item.cand.id) ?? []).length === 0" class="cp-cand-threads__hint">无线索</div>
                      <div
                        v-else
                        v-for="th in (candThreads.get(item.cand.id) ?? [])"
                        :key="th.id"
                        class="cp-thread-row"
                      >
                        <Icon icon="mdi:chevron-right" width="11" class="cp-thread-row__ico" />
                        <span class="cp-thread-row__title">{{ th.title }}</span>
                        <span class="cp-thread-row__count">{{ th.related_article_ids?.length ?? 0 }}篇</span>
                      </div>
                    </template>
                  </div>
                </div>
              </div>
            </template>
          </div>
        </div>

        <!-- 体检报告三卡（task 3.5）-->
        <div class="cp-col-r">
          <div class="cp-sub">
            <Icon icon="mdi:clipboard-pulse-outline" width="14" />
            <b>这条泳道的体检报告</b>
          </div>

          <!-- ① 聚类质量 -->
          <div class="cp-hcard cp-hcard--ok">
            <div class="cp-hcard__t">
              <Icon icon="mdi:checkbox-multiple-blank-circle-outline" width="15" />① 聚类质量
            </div>
            <div class="cp-hcard__row">
              选中 <b>{{ selectedCandidates.length }}</b> 条，平均两两距离 <b>{{ fmtDist(avgPairwiseDistance) }}</b>。
            </div>
            <div v-if="outlierSelectedIds.size > 0" class="cp-hcard__row cp-hcard__row--warn">
              有 <b>{{ outlierSelectedIds.size }}</b> 条离群，剔除后质量更优。
            </div>
            <div class="cp-hcard__act">
              <button
                class="cp-hbtn"
                :disabled="outlierSelectedIds.size === 0"
                @click="kickOutliers"
              >一键剔除 {{ outlierSelectedIds.size }} 个离群</button>
            </div>
          </div>

          <!-- ② 撞车检查 -->
          <div class="cp-hcard" :class="{ 'cp-hcard--warn': crash.moveOutCount > 0 }">
            <div class="cp-hcard__t">
              <Icon icon="mdi:alert-outline" width="15" />② 撞车检查
            </div>
            <template v-if="crash.moveOutCount === 0">
              <div class="cp-hcard__row">选中 section 暂无现有话题归属，保存不产生移出。</div>
            </template>
            <template v-else>
              <div v-for="m in crash.moveOutByTopic" :key="m.topicId" class="cp-hcard__row">
                <b>{{ m.count }}</b> 条将从「<b>{{ m.label }}</b>」移出、归入新泳道（单值覆盖）。
              </div>
            </template>
            <div v-if="nearestExistingDistance != null" class="cp-hcard__row cp-hcard__row--muted">
              最近现有话题距离 <b>{{ fmtDist(nearestExistingDistance) }}</b>。
            </div>
          </div>

          <!-- ③ 未来预期（v1 淡显"规划中"，不实现）-->
          <div class="cp-hcard cp-hcard--dim">
            <div class="cp-hcard__t">
              <Icon icon="mdi:clock-outline" width="15" />③ 未来预期
            </div>
            <div class="cp-hcard__row">历史 section 潜在命中预览。</div>
          </div>
        </div>
      </div>
    </template>

    <!-- 保存错误条 -->
    <div v-if="saveError" class="cp-save-error" role="alert">
      <Icon icon="mdi:alert-circle-outline" width="14" />
      <span>{{ saveError }}</span>
      <button class="cp-save-error-close" @click="saveError = null">✕</button>
    </div>

    <!-- 撞车确认弹窗（spec Scenario「撞车明确提示移出」）-->
    <AppDialog
      :model-value="crashConfirmOpen"
      title="保存将移出部分 section"
      width="460px"
      @update:model-value="(v) => { if (!v) crashConfirmOpen = false }"
    >
      <p class="cp-confirm-body">
        保存后，共 <b>{{ crash.moveOutCount }}</b> 条 section 将从原话题移出、归入新泳道「<b>{{ laneLabel || '未命名' }}</b>」（单值覆盖，原话题内容保留）。
      </p>
      <ul class="cp-confirm-list">
        <li v-for="m in crash.moveOutByTopic" :key="m.topicId">
          {{ m.count }} 条 ← {{ m.label }}
        </li>
      </ul>
      <template #footer>
        <AppButton variant="secondary" @click="crashConfirmOpen = false">取消</AppButton>
        <AppButton variant="primary" :loading="saving" @click="doSave">坚持新建</AppButton>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
.cp-root {
  display: flex;
  flex-direction: column;
  flex: 1;
  min-height: 0;
  overflow: hidden;
}

/* 编排态工具条 */
.cp-bar {
  flex: none;
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem 1rem;
  background: var(--color-bg-elevated);
  border-bottom: 1px solid var(--color-border-medium);
  flex-wrap: wrap;
}
.cp-back {
  appearance: none;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-base);
  color: var(--color-text-secondary);
  padding: 0.3rem 0.6rem;
  border-radius: 6px;
  font-size: 0.74rem;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  font-family: inherit;
}
.cp-back:hover { border-color: var(--color-accent); color: var(--color-accent); }
.cp-badge {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.64rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  color: var(--color-accent);
  padding: 0.18rem 0.5rem;
  border: 1px solid var(--color-accent);
  border-radius: 10px;
  background: var(--color-accent-subtle);
}
.cp-badge__dot { width: 6px; height: 6px; border-radius: 50%; background: var(--color-accent); }
.cp-name { flex: 1; min-width: 200px; }
.cp-src {
  font-size: 0.66rem;
  color: var(--color-text-muted);
  font-family: ui-monospace, monospace;
}
.cp-bar-sep { flex: 1; }

/* loading / error / empty */
.cp-loading, .cp-empty {
  padding: 2rem 1rem;
  text-align: center;
  color: var(--color-text-muted);
  font-size: 0.8rem;
}
.cp-error, .cp-save-error {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.7rem;
  font-size: 0.72rem;
  color: var(--color-error);
  background: var(--color-bg-active);
  border: 1px solid var(--color-error);
  border-radius: 6px;
  margin: 0.6rem 1rem 0;
}
.cp-save-error-close {
  margin-left: auto;
  border: 0;
  background: transparent;
  color: inherit;
  cursor: pointer;
  font-size: 0.8rem;
}

/* 预览时间轴 */
.cp-tl-wrap {
  flex: none;
  position: relative;
  overflow-x: auto;
  overflow-y: hidden;
  cursor: grab;
  user-select: none;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-bg-hover);
}
.cp-tl-wrap--dragging { cursor: grabbing; }
.cp-tl {
  display: flex;
  align-items: flex-start;
  padding: 0.85rem 1rem 1.1rem;
  min-width: 100%;
}
.cp-tl-placeholder {
  color: var(--color-text-muted);
  font-size: 0.8rem;
  font-style: italic;
  padding: 1rem 0;
}
.cp-col {
  flex: 0 0 120px;
  width: 120px;
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.4rem;
  position: relative;
}
.cp-col__date {
  font-size: 0.72rem;
  color: var(--color-text-muted);
  padding-bottom: 0.5rem;
  border-bottom: 1px solid var(--color-border-subtle);
  margin-bottom: 0.7rem;
}
.cp-col__date b { display: block; color: var(--color-text-secondary); font-size: 0.9rem; }
.cp-node-grp { display: flex; flex-direction: column; align-items: center; gap: 0.25rem; }
.cp-node {
  width: 34px;
  height: 34px;
  border-radius: 50%;
  background: var(--color-bg-base);
  border: 3px solid var(--color-accent);
  display: flex;
  align-items: center;
  justify-content: center;
  transition: transform 0.12s;
}
.cp-node i {
  width: 13px;
  height: 13px;
  border-radius: 50%;
  background: var(--color-accent);
  opacity: 0.85;
}
.cp-node--good { border-style: solid; }
.cp-node--boundary { border-style: dashed; }
.cp-node--boundary i { opacity: 0.4; }
.cp-node--outlier { border-color: var(--color-warning); border-style: dashed; }
.cp-node--outlier i { background: var(--color-warning); opacity: 0.5; }
.cp-nlabel {
  font-size: 0.66rem;
  line-height: 1.3;
  color: var(--color-text-secondary);
  max-width: 112px;
  text-align: center;
}
.cp-tl-hint {
  flex: none;
  text-align: center;
  padding: 0.38rem;
  color: var(--color-text-muted);
  font-size: 0.66rem;
  font-style: italic;
  background: var(--color-bg-elevated);
  border-bottom: 1px solid var(--color-border-subtle);
}

/* 双栏 */
.cp-cols {
  flex: 1;
  display: flex;
  gap: 1px;
  background: var(--color-border-subtle);
  min-height: 0;
  overflow: hidden;
}
.cp-col-l, .cp-col-r {
  background: var(--color-bg-base);
  overflow: auto;
  display: flex;
  flex-direction: column;
}
.cp-col-l { flex: 1.4; }
.cp-col-r { flex: 1; }
.cp-sub {
  flex: none;
  position: sticky;
  top: 0;
  z-index: 2;
  background: var(--color-bg-elevated);
  padding: 0.55rem 0.9rem;
  border-bottom: 1px solid var(--color-border-subtle);
  font-size: 0.74rem;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  gap: 0.4rem;
}
.cp-sub b { color: var(--color-text-primary); }
.cp-hint { color: var(--color-text-muted); font-size: 0.7rem; }

/* 候选话题引导区（连续命中候选的一键激活/并入） */
.cp-guide {
  flex: none;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
  background: var(--color-bg-elevated);
  border-bottom: 1px solid var(--color-border-subtle);
  padding: 0.5rem 0.9rem 0.6rem;
}
.cp-guide__head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  font-size: 0.74rem;
  color: var(--color-text-secondary);
  margin-bottom: 0.4rem;
}
.cp-guide__head b { color: var(--color-text-primary); }
.cp-guide__list { display: flex; flex-direction: column; gap: 0.3rem; }
.cp-guide__item {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  padding: 0.35rem 0.45rem;
  border-radius: 4px;
  background: var(--color-bg-base);
  border: 1px solid var(--color-border-subtle);
}
/* can_activate（已达阈值）高亮：accent 描边 + 左色条 */
.cp-guide__item--ready {
  border-color: var(--color-accent);
  box-shadow: inset 3px 0 0 var(--color-accent);
}
/* 分组标题 */
.cp-guide__grp {
  font-size: 0.68rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}
.cp-guide__grp--muted { color: var(--color-text-muted); font-weight: 400; }
/* 已中断项弱化 */
.cp-guide__item--muted { opacity: 0.65; }
.cp-guide__main { display: flex; flex-direction: column; gap: 0.1rem; min-width: 0; flex: 1; }
.cp-guide__label {
  font-size: 0.74rem;
  font-weight: 600;
  color: var(--color-text-primary);
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
}
.cp-guide__meta { font-size: 0.66rem; color: var(--color-text-muted); }
.cp-guide__ops { display: flex; gap: 0.3rem; flex: none; }

/* 语义搜索框 */
.cp-search {
  flex: none;
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.45rem 0.9rem;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
}
.cp-search__icon { color: var(--color-text-muted); flex: none; }
.cp-search__input { flex: 1; }
.cp-search__spin { color: var(--color-accent); flex: none; animation: cp-spin 0.9s linear infinite; }
@keyframes cp-spin { to { transform: rotate(360deg); } }
.cp-search-err {
  flex: none;
  padding: 0.35rem 0.9rem;
  font-size: 0.66rem;
  color: var(--color-warning);
  background: var(--color-warning-subtle);
  border-bottom: 1px solid var(--color-border-subtle);
}
/* 候选分组标题 */
.cp-cand-grp__t {
  flex: none;
  padding: 0.4rem 0.9rem 0.25rem;
  font-size: 0.64rem;
  letter-spacing: 0.06em;
  text-transform: uppercase;
  color: var(--color-accent);
  background: var(--color-bg-sunken);
}
.cp-cand-grp__t--muted { color: var(--color-text-muted); }

/* 候选池条目 */
.cp-cand-list { padding: 0; }
.cp-cand {
  width: 100%;
  display: flex;
  align-items: flex-start;
  gap: 0.55rem;
  padding: 0.6rem 0.9rem;
  border: 0;
  border-bottom: 1px solid var(--color-border-subtle);
  background: transparent;
  cursor: pointer;
  text-align: left;
  transition: background 0.1s;
}
.cp-cand:hover { background: var(--color-bg-hover); }
.cp-cand--outlier { background: var(--color-warning-subtle); }

/* 已勾选 section 的线索展开区（cell 包裹勾选 button + 线索列表）*/
.cp-cand-cell { display: flex; flex-direction: column; border-bottom: 1px solid var(--color-border-subtle); }
.cp-cand-cell .cp-cand { border-bottom: 0; }
.cp-cand-threads { display: flex; flex-direction: column; padding: 0 0.6rem 0.5rem 2rem; }
.cp-cand-threads__toggle {
  align-self: flex-start;
  border: 0;
  background: transparent;
  cursor: pointer;
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  font-size: 0.68rem;
  color: var(--color-text-muted);
  padding: 1px 4px;
  border-radius: 4px;
}
.cp-cand-threads__toggle:hover { background: var(--color-bg-hover); color: var(--color-text-secondary); }
.cp-cand-threads__body { display: flex; flex-direction: column; margin-top: 0.15rem; }
.cp-cand-threads__hint { font-size: 0.66rem; color: var(--color-text-muted); padding: 0.1rem 0.4rem; }
.cp-cand-threads__hint--err { color: var(--color-warning, var(--color-text-secondary)); }
.cp-thread-row {
  display: flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.12rem 0.4rem;
  font-size: 0.7rem;
  color: var(--color-text-secondary);
}
.cp-thread-row__ico { color: var(--color-text-muted); flex: none; }
.cp-thread-row__title {
  flex: 1;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.cp-thread-row__count {
  flex: none;
  font-size: 0.6rem;
  color: var(--color-text-muted);
  font-family: ui-monospace, monospace;
}
.cp-cand--outlier:hover { background: var(--color-bg-hover); }
.cp-cand__box {
  flex: none;
  width: 16px;
  height: 16px;
  margin-top: 0.15rem;
  border: 1.5px solid var(--color-border-medium);
  border-radius: 4px;
  display: inline-flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-base);
  color: var(--color-text-inverted);
}
.cp-cand--checked .cp-cand__box { background: var(--color-accent); border-color: var(--color-accent); }
.cp-cand__body { flex: 1; min-width: 0; }
.cp-cand__title {
  font-size: 0.82rem;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.45;
}
.cp-cand__sub { margin-top: 0.2rem; display: flex; align-items: center; gap: 0.4rem; flex-wrap: wrap; }
.cp-cand__dist { font-family: ui-monospace, monospace; font-size: 0.64rem; color: var(--color-text-muted); }
.cp-cand__dist--good { color: var(--color-success); }
.cp-cand__dist--outlier, .cp-cand__dist--far { color: var(--color-warning); }
.cp-cand__tag {
  font-size: 0.6rem;
  font-family: ui-monospace, monospace;
  color: var(--color-text-muted);
  background: var(--color-bg-sunken);
  padding: 0.06rem 0.32rem;
  border-radius: 8px;
}
.cp-cand__warn {
  font-size: 0.62rem;
  font-style: italic;
  color: var(--color-warning);
}

/* 体检卡 */
.cp-hcard {
  margin: 0.55rem 0.75rem;
  padding: 0.65rem 0.8rem;
  border: 1px solid var(--color-border-medium);
  border-radius: 6px;
  background: var(--color-bg-elevated);
}
.cp-hcard__t {
  font-size: 0.72rem;
  font-weight: 700;
  color: var(--color-text-secondary);
  display: flex;
  align-items: center;
  gap: 0.35rem;
  margin-bottom: 0.35rem;
}
.cp-hcard--ok { border-color: var(--color-success); }
.cp-hcard--ok .cp-hcard__t { color: var(--color-success); }
.cp-hcard--warn { border-color: var(--color-warning); background: var(--color-warning-subtle); }
.cp-hcard--warn .cp-hcard__t { color: var(--color-warning); }
.cp-hcard__row { font-size: 0.72rem; color: var(--color-text-secondary); line-height: 1.55; }
.cp-hcard__row b { color: var(--color-text-primary); font-family: ui-monospace, monospace; }
.cp-hcard__row--warn { color: var(--color-warning); margin-top: 0.2rem; }
.cp-hcard__row--warn b { color: var(--color-warning); }
.cp-hcard__row--muted { color: var(--color-text-muted); margin-top: 0.3rem; }
.cp-hcard__act { margin-top: 0.5rem; }
.cp-hbtn {
  appearance: none;
  border: 1px solid var(--color-border-medium);
  background: transparent;
  color: var(--color-text-secondary);
  padding: 0.28rem 0.55rem;
  border-radius: 5px;
  font-size: 0.66rem;
  cursor: pointer;
  font-family: inherit;
}
.cp-hbtn:hover:not(:disabled) { border-color: var(--color-accent); color: var(--color-accent); }
.cp-hbtn:disabled { opacity: 0.4; cursor: not-allowed; }
.cp-hcard--dim { opacity: 0.5; }
.cp-hcard--dim .cp-hcard__t::after {
  content: '规划中';
  font-size: 0.56rem;
  font-weight: 400;
  color: var(--color-text-muted);
  margin-left: auto;
  font-style: italic;
}

/* 撞车确认弹窗 */
.cp-confirm-body { font-size: 0.84rem; color: var(--color-text-primary); line-height: 1.5; margin: 0; }
.cp-confirm-body b { color: var(--color-accent); }
.cp-confirm-list {
  margin: 0.6rem 0 0;
  padding-left: 1.2rem;
  font-size: 0.76rem;
  color: var(--color-text-secondary);
  line-height: 1.7;
}
</style>
