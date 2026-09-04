<script setup lang="ts">
import { computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import type { UpgradeCandidate, UpgradeCluster, UpgradeSuggestion, BoardAffinity, UpgradeSuggestionRow, SemanticBoard } from '~/api/semanticBoards'

const props = defineProps<{
  visible: boolean
  candidates: UpgradeCandidate[]
  clusters: UpgradeCluster[]
  suggestions: UpgradeSuggestion[]
  loading: boolean
  suggesting: boolean
  backfillNotice: boolean
  /** 持久化建议（主数据源，GET /upgrade-suggestions）。 */
  persistedSuggestions: UpgradeSuggestionRow[]
  persistedLoading: boolean
  persistedGenerating: boolean
  /** 全量 active 板块，供 merge 建议选目标版块用。 */
  boards: SemanticBoard[]
}>()

const emit = defineEmits<{
  suggest: [mode: string]
  execute: [suggestion: UpgradeSuggestion, index: number]
  cancel: []
  loadPersisted: [decision: string]
  generate: []
  dismissRow: [id: number]
  confirmRow: [row: UpgradeSuggestionRow]
}>()

const upgradeMode = ref<'discover_new' | 'expand_existing'>('discover_new')
const openMergeIndex = ref<number | null>(null)
// 持久化行：当前展开「合并到...」选目标版块下拉的行 id。
// 所有 merge 行都先选目标（不盲用 LLM/high 给的 top-1），用户在下拉里确认或改选。
const openMergeRowId = ref<number | null>(null)
// 下拉内的搜索关键字（按行隔离：用 openMergeRowId 对应的 row id 作为 key）。
const mergeSearchByRow = ref<Record<number, string>>({})
// 每行勾选的辅助标签 id 集合（row.id → Set<auxId>）。默认全选；emit 时只带勾选子集。
const selectedAuxByRow = ref<Record<number, Set<number>>>({})

// ---- 持久化建议过滤（主数据源） ----
type PersistedFilter = 'all' | 'merge_into_existing' | 'create_new' | 'watch' | 'compose'
const persistedFilter = ref<PersistedFilter>('all')
const filterTabs: { key: PersistedFilter; label: string }[] = [
  { key: 'all', label: '全部' },
  { key: 'merge_into_existing', label: '合并' },
  { key: 'create_new', label: '新建' },
  { key: 'compose', label: '组合' },
  { key: 'watch', label: '观察池' },
]

// all → decision=""（后端默认列表，排除 watch）；其余原样传
function decisionParam(tab: PersistedFilter): string {
  return tab === 'all' ? '' : tab
}

watch(persistedFilter, (tab) => emit('loadPersisted', decisionParam(tab)))
watch(() => props.visible, (v) => {
  if (v) emit('loadPersisted', decisionParam(persistedFilter.value))
})

// ---- evidence 安全读取（§4 未全部落地，缺 key 降级不崩） ----
function laneBriefs(row: UpgradeSuggestionRow): string[] {
  const v = row.evidence?.lane_briefs
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}
function cotagEvents(row: UpgradeSuggestionRow): string[] {
  const v = row.evidence?.cotag_events
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}
interface ShortlistItem {
  board_id?: number
  board_label?: string
  composition_dist?: number
  lane_dist?: number
}
function shortlist(row: UpgradeSuggestionRow): ShortlistItem[] {
  const v = row.evidence?.shortlist
  return Array.isArray(v) ? v.filter((x): x is ShortlistItem => x != null && typeof x === 'object') : []
}
function rowAuxLabels(row: UpgradeSuggestionRow): { id: number; label: string; status?: string }[] {
  return row.auxiliary_labels && row.auxiliary_labels.length > 0
    ? row.auxiliary_labels
    : row.auxiliary_label_ids.map((id) => ({ id, label: `标签 #${id}` }))
}

// ---- 辅助标签勾选（不必全要，emit 时只带勾选子集）----
// 每行维护一个「选中的 id 集合」；默认全选。persistedSuggestions 变化时：
// 已有勾选记录的行沿用原勾选（清理掉已不存在的标签 id）；新行默认全选。
watch(() => props.persistedSuggestions, (rows) => {
  const next: Record<number, Set<number>> = {}
  for (const row of rows) {
    // 默认勾选所有可选辅助标签；失效标签（status≠active，建议生成后被禁用）
    // 默认不勾、置灰展示。用户手动 toggle 的选择在 persistedSuggestions 不变时保留
    // （此 watch 仅在列表重载时触发重置默认值）。
    next[row.id] = new Set(rowAuxLabels(row).filter((al) => !isAuxDisabled(al)).map((al) => al.id))
  }
  selectedAuxByRow.value = next
}, { immediate: true, deep: false })

// 失效标签（status 有值且非 active）：建议生成后可能被禁用，置灰且默认不勾。
function isAuxDisabled(al: { status?: string }): boolean {
  return !!al.status && al.status !== 'active'
}

function isAuxSelected(rowId: number, auxId: number): boolean {
  return selectedAuxByRow.value[rowId]?.has(auxId) ?? true
}
function toggleAux(rowId: number, auxId: number): void {
  const set = selectedAuxByRow.value[rowId] ?? new Set<number>()
  if (set.has(auxId)) set.delete(auxId)
  else set.add(auxId)
  selectedAuxByRow.value[rowId] = set
}
function selectAllAux(row: UpgradeSuggestionRow): void {
  selectedAuxByRow.value[row.id] = new Set(rowAuxLabels(row).map((al) => al.id))
}
function clearAllAux(row: UpgradeSuggestionRow): void {
  selectedAuxByRow.value[row.id] = new Set<number>()
}
function selectedAuxCount(row: UpgradeSuggestionRow): number {
  const all = rowAuxLabels(row)
  return all.filter((al) => isAuxSelected(row.id, al.id)).length
}
// 当前行勾选的 auxiliary_label_ids（供 emit 用）。
function selectedAuxIds(row: UpgradeSuggestionRow): number[] {
  return rowAuxLabels(row).filter((al) => isAuxSelected(row.id, al.id)).map((al) => al.id)
}
function rowTargetLabel(row: UpgradeSuggestionRow): string {
  if (row.target_board_label) return row.target_board_label
  return row.target_board_id ? `板块 #${row.target_board_id}` : ''
}

// ---- merge 行选目标版块（board-upgrade spec 方案 B + 候选优先全量可搜）----
// 所有 merge 行都先展开下拉让用户确认/改选目标，不盲用 LLM/high 给的 top-1。
// 下拉内容：① LLM/算法候选区（evidence.shortlist，高亮置顶）；② 全量 active 板块区
// （可搜索，去重掉已在候选区的）。
function rowOffShortlist(row: UpgradeSuggestionRow): boolean {
  return row.evidence?.target_off_shortlist === true
}

// ---- compose 建议证据（安全读取，缺 key 降级） ----
function composeCooccurrence(row: UpgradeSuggestionRow): number | null {
  const v = row.evidence?.compose_cooccurrence
  return typeof v === 'number' ? v : null
}
function composeWindowDays(row: UpgradeSuggestionRow): number | null {
  const v = row.evidence?.compose_window_days
  return typeof v === 'number' ? v : null
}
function composeRepresentTitles(row: UpgradeSuggestionRow): string[] {
  const v = row.evidence?.compose_representative_titles
  return Array.isArray(v) ? v.filter((x): x is string => typeof x === 'string') : []
}

// 某行下拉的关键字（双向绑定用）。
function mergeSearch(rowId: number): string {
  return mergeSearchByRow.value[rowId] ?? ''
}
function setMergeSearch(rowId: number, val: string): void {
  mergeSearchByRow.value[rowId] = val
}

// 下拉里要展示的全量板块（已排除 shortlist 候选，避免重复；按关键字过滤）。
function extraBoardsForRow(row: UpgradeSuggestionRow): SemanticBoard[] {
  const candidateIds = new Set(
    shortlist(row).map((s) => s.board_id).filter((id): id is number => typeof id === 'number'),
  )
  const kw = (mergeSearchByRow.value[row.id] ?? '').trim().toLowerCase()
  return props.boards.filter((b) => {
    if (candidateIds.has(b.id)) return false
    if (!kw) return true
    return b.label.toLowerCase().includes(kw)
      || (b.aliases ?? []).some((a) => a.toLowerCase().includes(kw))
  })
}

function handlePickRowTarget(row: UpgradeSuggestionRow, boardId: number) {
  openMergeRowId.value = null
  delete mergeSearchByRow.value[row.id]
  // 选「合并到...」目标即按 merge_into_existing 执行：create_new 行也可借此合并到已有板块。
  emit('confirmRow', { ...row, decision: 'merge_into_existing', target_board_id: boardId, auxiliary_label_ids: selectedAuxIds(row) })
}

// create_new 行确认：同样只带勾选的辅助标签子集。
function handleConfirmCreateRow(row: UpgradeSuggestionRow) {
  emit('confirmRow', { ...row, auxiliary_label_ids: selectedAuxIds(row) })
}

// compose 行确认：创建组合标签（带勾选组件子集）。
function handleConfirmComposeRow(row: UpgradeSuggestionRow) {
  emit('confirmRow', { ...row, decision: 'compose', auxiliary_label_ids: selectedAuxIds(row) })
}

// ---- 手动合并（内存建议，保留现状，§6.3 不动） ----
function toggleMerge(index: number) {
  openMergeIndex.value = openMergeIndex.value === index ? null : index
}

function handleMerge(s: UpgradeSuggestion, index: number, boardId: number) {
  emit('execute', {
    ...s,
    decision: 'merge_into_existing' as const,
    target_board_id: boardId,
  }, index)
}

function decisionLabel(d: string): string {
  switch (d) {
    case 'create_new': return '创建新板块'
    case 'merge_into_existing': return '合并到已有板块'
    case 'compose': return '创建组合标签'
    case 'watch': return '观察池'
    case 'skip': return '跳过'
    default: return d
  }
}

function decisionStyle(d: string): { border: string; bg: string; color: string } {
  switch (d) {
    case 'create_new': return { border: 'var(--color-success-border, rgba(61,138,74,0.3))', bg: 'var(--color-success-bg, rgba(61,138,74,0.08))', color: 'var(--color-success)' }
    case 'merge_into_existing': return { border: 'var(--color-link-border)', bg: 'var(--color-link-subtle)', color: 'var(--color-link)' }
    case 'compose': return { border: 'var(--color-accent-border, rgba(120,80,200,0.3))', bg: 'var(--color-accent-subtle)', color: 'var(--color-accent)' }
    case 'watch': return { border: 'var(--color-warning-border, rgba(180,140,40,0.3))', bg: 'var(--color-warning-bg, rgba(180,140,40,0.08))', color: 'var(--color-warning, #b48c28)' }
    case 'skip': return { border: 'var(--color-border-medium)', bg: 'var(--color-bg-sunken)', color: 'var(--color-text-muted)' }
    default: return { border: 'var(--color-border-subtle)', bg: 'var(--color-bg-hover)', color: 'var(--color-text-secondary)' }
  }
}
</script>

<template>
  <Teleport to="body">
    <div v-if="visible" class="usp-overlay" @click.self="emit('cancel'); openMergeIndex = null">
      <div class="usp-card">
        <div class="usp-header">
          <div>
            <h3 class="usp-title">板块升级建议</h3>
            <p class="usp-subtitle">
              候选标签 {{ candidates.length }} 个 · 聚类 {{ clusters.length }} 个
            </p>
          </div>
          <button type="button" class="usp-close" @click="emit('cancel')">
            <Icon icon="mdi:close" width="18" />
          </button>
        </div>

        <!-- 持久化建议（主数据源，§6.1/6.2） -->
        <section class="usp-persisted">
          <div class="usp-persisted-toolbar">
            <div class="usp-filter-tabs">
              <button
                v-for="t in filterTabs"
                :key="t.key"
                type="button"
                class="usp-filter-tab"
                :class="{ 'is-active': persistedFilter === t.key }"
                @click="persistedFilter = t.key"
              >
                {{ t.label }}
              </button>
            </div>
            <button
              type="button"
              class="usp-suggest-btn usp-suggest-btn--small"
              :disabled="persistedGenerating"
              @click="emit('generate')"
            >
              <Icon v-if="persistedGenerating" icon="mdi:loading" width="13" class="animate-spin" />
              <Icon v-else icon="mdi:auto-fix" width="13" />
              {{ persistedGenerating ? '生成中...' : '生成建议' }}
            </button>
          </div>

          <div v-if="persistedLoading" class="usp-loading">
            <Icon icon="mdi:loading" width="20" class="animate-spin text-white/30" />
            <span>加载建议...</span>
          </div>
          <div v-else-if="persistedSuggestions.length === 0" class="usp-persisted-empty">
            <Icon icon="mdi:check-circle-outline" width="16" />
            <span>暂无持久化建议</span>
          </div>
          <div v-else class="usp-persisted-list">
            <div
              v-for="row in persistedSuggestions"
              :key="row.id"
              class="usp-item usp-row"
              :class="{ 'usp-row--high': row.confidence === 'high' }"
              :data-confidence="row.confidence"
              :data-decision="row.decision"
              :style="{ borderColor: decisionStyle(row.decision).border, background: decisionStyle(row.decision).bg }"
            >
              <div class="usp-item-header">
                <span class="usp-item-decision" :style="{ color: decisionStyle(row.decision).color }">
                  {{ decisionLabel(row.decision) }}
                </span>
                <span v-if="row.confidence === 'high'" class="usp-confidence-badge" data-confidence="high">高置信</span>
                <span v-if="rowOffShortlist(row)" class="usp-confidence-badge usp-off-shortlist-badge">目标超出候选范围</span>
                <span v-if="rowTargetLabel(row)" class="usp-item-board">{{ rowTargetLabel(row) }}</span>
                <span v-else-if="row.board_label" class="usp-item-board">{{ row.board_label }}</span>
              </div>
              <p v-if="row.description" class="usp-item-desc">{{ row.description }}</p>
              <div class="usp-aux-toolbar">
                <span class="usp-aux-count">辅助标签 {{ selectedAuxCount(row) }}/{{ rowAuxLabels(row).length }}</span>
                <button type="button" class="usp-aux-toggle" @click="selectAllAux(row)">全选</button>
                <button type="button" class="usp-aux-toggle" @click="clearAllAux(row)">清空</button>
              </div>
              <div class="usp-item-tags">
                <button
                  v-for="al in rowAuxLabels(row)"
                  :key="al.id"
                  type="button"
                  class="usp-item-tag usp-aux-chip"
                  :class="{ 'usp-aux-chip--off': !isAuxSelected(row.id, al.id), 'usp-aux-chip--disabled': isAuxDisabled(al) }"
                  :disabled="isAuxDisabled(al)"
                  :title="isAuxDisabled(al) ? ((al.label || ('标签 #' + al.id)) + '（已失效）') : (al.label || ('标签 #' + al.id))"
                  @click="toggleAux(row.id, al.id)"
                >
                  <Icon
                    :icon="isAuxSelected(row.id, al.id) ? 'mdi:checkbox-marked' : 'mdi:checkbox-blank-outline'"
                    width="12"
                  />
                  {{ al.label || ('标签 #' + al.id) }}
                  <span v-if="isAuxDisabled(al)" class="usp-aux-chip-status">已失效</span>
                </button>
              </div>
              <div v-if="laneBriefs(row).length > 0" class="usp-evidence">
                <span class="usp-evidence-label">泳道：</span>
                <span v-for="(b, bi) in laneBriefs(row)" :key="'lb' + bi" class="usp-evidence-chip">{{ b }}</span>
              </div>
              <div v-if="cotagEvents(row).length > 0" class="usp-evidence">
                <span class="usp-evidence-label">共现：</span>
                <span v-for="(e, ei) in cotagEvents(row)" :key="'ce' + ei" class="usp-evidence-chip">{{ e }}</span>
              </div>
              <div v-if="row.decision === 'compose' && (composeCooccurrence(row) !== null || composeRepresentTitles(row).length > 0)" class="usp-evidence" data-testid="compose-evidence">
                <span class="usp-evidence-label">组合证据：</span>
                <span v-if="composeCooccurrence(row) !== null" class="usp-evidence-chip" data-testid="compose-cooccurrence">
                  共现 {{ composeCooccurrence(row) }} 篇<template v-if="composeWindowDays(row)"> / {{ composeWindowDays(row) }} 天窗口</template>
                </span>
                <span v-for="(title, ti) in composeRepresentTitles(row)" :key="'rt' + ti" class="usp-evidence-chip" :title="title">{{ title }}</span>
              </div>
              <div v-if="shortlist(row).length > 0" class="usp-evidence">
                <span class="usp-evidence-label">候选版块：</span>
                <span v-for="(s, si) in shortlist(row)" :key="'sl' + si" class="usp-evidence-chip">{{ s.board_label || ('板块 #' + s.board_id) }}</span>
              </div>
              <div class="usp-item-actions">
                <template v-if="row.decision === 'merge_into_existing' || row.decision === 'create_new'">
                  <!-- merge 行与 create_new 行都可「合并到...」已有板块：候选优先（shortlist 置顶高亮）+ 全量可搜；create_new 行另保留「创建新版块」按钮 -->
                  <div class="usp-merge-wrapper">
                    <button
                      type="button"
                      class="usp-item-btn usp-item-btn--merge"
                      :disabled="selectedAuxCount(row) === 0"
                      :title="selectedAuxCount(row) === 0 ? '至少勾选一个辅助标签' : undefined"
                      @click="openMergeRowId = openMergeRowId === row.id ? null : row.id"
                    >
                      <Icon icon="mdi:merge" width="12" />
                      合并到...
                    </button>
                    <div v-if="openMergeRowId === row.id" class="usp-merge-dropdown usp-merge-dropdown--search">
                      <input
                        class="usp-merge-search"
                        type="text"
                        placeholder="搜索板块名称或别名…"
                        :value="mergeSearch(row.id)"
                        @input="setMergeSearch(row.id, ($event.target as HTMLInputElement).value)"
                      >
                      <div class="usp-merge-list">
                        <!-- ① LLM/算法候选区（evidence.shortlist），高亮置顶 -->
                        <template v-if="shortlist(row).length > 0">
                          <div class="usp-merge-group-label">候选版块</div>
                          <button
                            v-for="s in shortlist(row)"
                            :key="'sl-' + s.board_id"
                            type="button"
                            class="usp-merge-option usp-merge-option--candidate"
                            @click="s.board_id && handlePickRowTarget(row, s.board_id)"
                          >
                            <span class="usp-merge-option-name">{{ s.board_label || ('板块 #' + s.board_id) }}</span>
                            <span v-if="typeof s.composition_dist === 'number'" class="usp-merge-option-detail">comp {{ s.composition_dist.toFixed(3) }}</span>
                          </button>
                        </template>
                        <!-- ② 全量板块区（去重候选，按搜索词过滤） -->
                        <div class="usp-merge-group-label">{{ shortlist(row).length > 0 ? '其他板块' : '全部板块' }}</div>
                        <button
                          v-for="b in extraBoardsForRow(row)"
                          :key="'ex-' + b.id"
                          type="button"
                          class="usp-merge-option"
                          :class="{ 'usp-merge-option--recommended': row.target_board_id === b.id }"
                          @click="handlePickRowTarget(row, b.id)"
                        >
                          <span class="usp-merge-option-name">{{ b.label }}</span>
                          <span v-if="row.target_board_id === b.id" class="usp-merge-option-tag">推荐</span>
                        </button>
                        <span v-if="extraBoardsForRow(row).length === 0" class="usp-merge-empty">无匹配板块</span>
                      </div>
                    </div>
                  </div>
                </template>
                <button
                  v-if="row.decision === 'create_new'"
                  type="button"
                  class="usp-item-btn usp-item-btn--primary"
                  :disabled="selectedAuxCount(row) === 0"
                  :title="selectedAuxCount(row) === 0 ? '至少勾选一个辅助标签' : undefined"
                  @click="handleConfirmCreateRow(row)"
                >
                  <Icon icon="mdi:check" width="12" />
                  创建新版块
                </button>
                <button
                  v-if="row.decision === 'compose'"
                  type="button"
                  class="usp-item-btn usp-item-btn--primary"
                  :disabled="selectedAuxCount(row) < 2"
                  :title="selectedAuxCount(row) < 2 ? '组合标签至少需要 2 个组件' : undefined"
                  data-testid="compose-confirm"
                  @click="handleConfirmComposeRow(row)"
                >
                  <Icon icon="mdi:check" width="12" />
                  创建组合
                </button>
                <button
                  type="button"
                  class="usp-item-btn usp-item-btn--dismiss"
                  @click="emit('dismissRow', row.id)"
                >
                  <Icon icon="mdi:close" width="12" />
                  忽略
                </button>
              </div>
            </div>
          </div>
        </section>

        <div class="usp-divider"></div>
        <div class="usp-manual-title">手动探索候选</div>

        <div v-if="loading" class="usp-loading">
          <Icon icon="mdi:loading" width="20" class="animate-spin text-white/30" />
          <span>加载候选...</span>
        </div>

        <div v-else-if="suggestions.length === 0" class="usp-empty">
          <p v-if="candidates.length === 0">暂无满足条件的升级候选</p>
          <div v-if="backfillNotice" class="usp-notice">
            <Icon icon="mdi:information-outline" width="14" />
            <span>已执行升级建议。历史标签归属不会自动回填，可手动触发匹配回填让新构成生效。</span>
          </div>
          <div v-if="candidates.length > 0" class="usp-mode-selector">
            <label class="usp-mode-option">
              <input v-model="upgradeMode" type="radio" value="discover_new" />
              <span>发现新版块</span>
            </label>
            <label class="usp-mode-option">
              <input v-model="upgradeMode" type="radio" value="expand_existing" />
              <span>扩充已有版块</span>
            </label>
          </div>
          <button
            v-if="candidates.length > 0"
            type="button"
            class="usp-suggest-btn"
            :disabled="suggesting"
            @click="emit('suggest', upgradeMode)"
          >
            <Icon v-if="suggesting" icon="mdi:loading" width="14" class="animate-spin" />
            <Icon v-else icon="mdi:brain" width="14" />
            {{ suggesting ? 'LLM 分析中...' : '获取 LLM 建议' }}
          </button>
        </div>

        <div v-else class="usp-list">
          <div class="usp-toolbar">
            <span class="usp-toolbar-text">待处理建议 {{ suggestions.length }} 个</span>
            <button
              type="button"
              class="usp-suggest-btn usp-suggest-btn--small"
              :disabled="suggesting"
              @click="emit('suggest', upgradeMode)"
            >
              <Icon v-if="suggesting" icon="mdi:loading" width="13" class="animate-spin" />
              <Icon v-else icon="mdi:refresh" width="13" />
              {{ suggesting ? '重新分析中...' : '重新生成建议' }}
            </button>
          </div>

          <div v-if="backfillNotice" class="usp-notice">
            <Icon icon="mdi:information-outline" width="14" />
            <span>已执行升级建议。历史标签归属不会自动回填，可手动触发匹配回填让新构成生效。</span>
          </div>

          <div
            v-for="(s, i) in suggestions"
            :key="i"
            class="usp-item"
            :style="{ borderColor: decisionStyle(s.decision).border, background: decisionStyle(s.decision).bg }"
          >
            <div class="usp-item-header">
              <span class="usp-item-decision" :style="{ color: decisionStyle(s.decision).color }">
                {{ decisionLabel(s.decision) }}
              </span>
              <span v-if="s.board_label" class="usp-item-board">{{ s.board_label }}</span>
              <span v-else-if="s.target_board_label" class="usp-item-board">{{ s.target_board_label }}</span>
              <span v-else-if="s.target_board_id" class="usp-item-board">板块 #{{ s.target_board_id }}</span>
            </div>
            <p v-if="s.description" class="usp-item-desc">{{ s.description }}</p>
            <p class="usp-item-reason">{{ s.reason }}</p>
            <div class="usp-item-tags">
              <template v-if="s.auxiliary_labels && s.auxiliary_labels.length > 0">
                <span v-for="al in s.auxiliary_labels" :key="al.id" class="usp-item-tag">{{ al.label || ('标签 #' + al.id) }}</span>
              </template>
              <template v-else>
                <span v-for="id in s.auxiliary_label_ids" :key="id" class="usp-item-tag">标签 #{{ id }}</span>
              </template>
            </div>
            <div v-if="s.board_affinities && s.board_affinities.length > 0" class="usp-item-affinities">
              <span class="usp-item-affinities-label">相似板块：</span>
              <span
                v-for="(aff, ai) in s.board_affinities"
                :key="ai"
                class="usp-item-affinity"
              >
                {{ aff.board_label }}
                <span class="usp-item-affinity-detail">
                  ({{ aff.matching_candidates }} candidates, avg distance {{ aff.avg_distance.toFixed(4) }})
                </span>
              </span>
            </div>
            <div class="usp-item-actions">
              <!-- skip 类型的建议也支持手动操作：创建新板块或合并到已有板块 -->
              <template v-if="s.decision === 'skip'">
                <button
                  type="button"
                  class="usp-item-btn usp-item-btn--primary"
                  @click="emit('execute', {
                    ...s,
                    decision: 'create_new' as const,
                    board_label: s.board_label || (s.auxiliary_labels && s.auxiliary_labels.length > 0 ? s.auxiliary_labels[0]!.label : undefined),
                  }, i)"
                >
                  <Icon icon="mdi:plus" width="12" />
                  改为创建
                </button>
                <template v-if="s.board_affinities && s.board_affinities.length > 0">
                  <div class="usp-merge-wrapper">
                    <button
                      type="button"
                      class="usp-item-btn usp-item-btn--merge"
                      @click="toggleMerge(i)"
                    >
                      <Icon icon="mdi:merge" width="12" />
                      合并到...
                    </button>
                    <div v-if="openMergeIndex === i" class="usp-merge-dropdown">
                      <button
                        v-for="aff in s.board_affinities"
                        :key="aff.board_id"
                        type="button"
                        class="usp-merge-option"
                        @click="handleMerge(s, i, aff.board_id)"
                      >
                        {{ aff.board_label }}
                        <span class="usp-merge-option-detail">({{ aff.matching_candidates }} matches)</span>
                      </button>
                    </div>
                  </div>
                </template>
              </template>
              <template v-else>
              <button
                type="button"
                class="usp-item-btn usp-item-btn--primary"
                @click="emit('execute', s, i)"
              >
                <Icon icon="mdi:check" width="12" />
                确认执行
              </button>
              <template v-if="s.board_affinities && s.board_affinities.length > 0">
                <div class="usp-merge-wrapper">
                  <button
                    type="button"
                    class="usp-item-btn usp-item-btn--merge"
                    @click="toggleMerge(i)"
                  >
                    <Icon icon="mdi:merge" width="12" />
                    合并到...
                  </button>
                  <div v-if="openMergeIndex === i" class="usp-merge-dropdown">
                    <button
                      v-for="aff in s.board_affinities"
                      :key="aff.board_id"
                      type="button"
                      class="usp-merge-option"
                      @click="handleMerge(s, i, aff.board_id)"
                    >
                      {{ aff.board_label }}
                      <span class="usp-merge-option-detail">({{ aff.matching_candidates }} matches)</span>
                    </button>
                  </div>
                </div>
              </template>
              </template>
            </div>
          </div>
        </div>
      </div>
    </div>
  </Teleport>
</template>

<style scoped>
.usp-overlay {
  position: fixed;
  inset: 0;
  z-index: 100;
  display: flex;
  align-items: center;
  justify-content: center;
  background: var(--color-bg-overlay);
  backdrop-filter: blur(8px);
  padding: 1rem;
}

.usp-card {
  width: min(560px, 95vw);
  max-height: 80vh;
  overflow-y: auto;
  border-radius: 1.25rem;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-elevated);
  padding: 1.5rem;
  box-shadow: 0 20px 60px rgba(0, 0, 0, 0.5);
  display: flex;
  flex-direction: column;
  gap: 1rem;
}

.usp-header {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 1rem;
}

.usp-title {
  font-size: 0.95rem;
  font-weight: 600;
  color: var(--color-text-primary);
}

.usp-subtitle {
  margin-top: 0.25rem;
  font-size: 0.72rem;
  color: var(--color-text-muted);
}

.usp-close {
  display: flex;
  align-items: center;
  justify-content: center;
  width: 28px;
  height: 28px;
  border: none;
  border-radius: 8px;
  background: none;
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.12s ease;
}

.usp-close:hover {
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
}

.usp-loading {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 2rem 0;
  color: var(--color-text-muted);
  font-size: 0.8rem;
}

.usp-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.75rem;
  padding: 2rem 0;
  color: var(--color-text-muted);
  font-size: 0.8rem;
}

.usp-mode-selector {
  display: flex;
  gap: 1rem;
  justify-content: center;
}

.usp-mode-option {
  display: flex;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.75rem;
  color: var(--color-text-secondary);
  cursor: pointer;
}

.usp-mode-option input[type="radio"] {
  accent-color: var(--color-accent);
  width: 14px;
  height: 14px;
}

.usp-suggest-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.5rem 1rem;
  border-radius: 10px;
  border: 1px solid var(--color-accent);
  background: var(--color-accent-subtle);
  color: var(--color-accent-hover);
  font-size: 0.8rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.usp-suggest-btn:hover:not(:disabled) {
  background: var(--color-accent-subtle);
}

.usp-suggest-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

.usp-suggest-btn--small {
  padding: 0.35rem 0.65rem;
  font-size: 0.72rem;
}

.usp-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.usp-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.75rem;
}

.usp-toolbar-text {
  font-size: 0.72rem;
  color: var(--color-text-muted);
}

.usp-notice {
  display: flex;
  align-items: flex-start;
  gap: 0.45rem;
  padding: 0.65rem 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--color-info-bg, rgba(61,122,138,0.25));
  background: var(--color-info-bg, rgba(61,122,138,0.08));
  color: var(--color-info);
  font-size: 0.72rem;
  line-height: 1.5;
}

.usp-item {
  padding: 0.85rem;
  border-radius: 12px;
  border: 1px solid;
  display: flex;
  flex-direction: column;
  gap: 0.4rem;
}

.usp-item-header {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.usp-item-decision {
  font-size: 0.72rem;
  font-weight: 600;
  padding: 0.15rem 0.4rem;
  border-radius: 6px;
  background: var(--color-input-bg);
}

.usp-item-board {
  font-size: 0.8rem;
  font-weight: 500;
  color: var(--color-text-primary);
}

.usp-item-desc {
  font-size: 0.75rem;
  color: var(--color-text-muted);
  line-height: 1.5;
}

.usp-item-reason {
  font-size: 0.72rem;
  color: var(--color-text-muted);
  line-height: 1.5;
}

.usp-item-tags {
  display: flex;
  flex-wrap: wrap;
  gap: 0.3rem;
}

.usp-item-tag {
  font-size: 0.65rem;
  color: var(--color-text-muted);
  padding: 0.1rem 0.35rem;
  border-radius: 6px;
  background: var(--color-bg-hover);
}

/* 辅助标签勾选 chip */
.usp-aux-toolbar {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  margin-bottom: 0.15rem;
}

.usp-aux-count {
  font-size: 0.64rem;
  color: var(--color-text-muted);
}

.usp-aux-toggle {
  border: none;
  background: none;
  color: var(--color-text-muted);
  font-size: 0.64rem;
  cursor: pointer;
  padding: 0.05rem 0.2rem;
  border-radius: 4px;
  transition: all 0.1s ease;
}

.usp-aux-toggle:hover {
  color: var(--color-text-secondary);
  background: var(--color-bg-hover);
}

.usp-aux-chip {
  display: inline-flex;
  align-items: center;
  gap: 0.2rem;
  border: 1px solid transparent;
  cursor: pointer;
  transition: all 0.1s ease;
  user-select: none;
}

.usp-aux-chip:hover {
  background: var(--color-bg-sunken);
}

/* 选中态：用 link 色高亮 */
.usp-aux-chip:not(.usp-aux-chip--off) {
  color: var(--color-link);
  background: var(--color-link-subtle);
}

/* 取消态：灰化 + 删除线 */
.usp-aux-chip--off {
  color: var(--color-text-muted);
  background: transparent;
  border-color: var(--color-border-subtle);
  text-decoration: line-through;
  opacity: 0.65;
}

/* 失效标签（建议生成后被禁用）：在 --off 基础上锁死不可交互 */
.usp-aux-chip--disabled {
  cursor: not-allowed;
  opacity: 0.5;
}

.usp-aux-chip--disabled:hover {
  background: transparent;
}

.usp-aux-chip-status {
  margin-left: 0.2rem;
  padding: 0.02rem 0.2rem;
  border-radius: 3px;
  background: var(--color-warning-bg, rgba(180, 140, 40, 0.18));
  color: var(--color-warning, #b48c28);
  font-size: 0.58rem;
  font-weight: 600;
}

.usp-item-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 0.25rem;
}

.usp-item-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.7rem;
  border-radius: 8px;
  border: 1px solid var(--color-border-medium);
  background: none;
  color: var(--color-text-muted);
  font-size: 0.72rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.usp-item-btn--primary {
  border-color: var(--color-success-border, rgba(61,138,74,0.3));
  background: var(--color-success-bg, rgba(61,138,74,0.1));
  color: var(--color-success);
}

.usp-item-btn--primary:hover {
  background: var(--color-success-bg, rgba(61,138,74,0.18));
}

.usp-item-affinities {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.35rem;
  font-size: 0.68rem;
  color: var(--color-text-muted);
}

.usp-item-affinities-label {
  color: var(--color-text-muted);
}

.usp-item-affinity {
  padding: 0.1rem 0.3rem;
  border-radius: 4px;
  background: var(--color-link-subtle);
  color: var(--color-link);
}

.usp-item-affinity-detail {
  color: var(--color-text-muted);
}

.usp-merge-wrapper {
  position: relative;
}

.usp-item-btn--merge {
  border-color: var(--color-link-border);
  background: var(--color-link-subtle);
  color: var(--color-link);
}

.usp-item-btn--merge:hover {
  background: var(--color-link-border);
}

.usp-merge-dropdown {
  position: absolute;
  right: 0;
  bottom: 100%;
  margin-bottom: 4px;
  min-width: 200px;
  border-radius: 8px;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-elevated);
  box-shadow: 0 8px 24px rgba(0, 0, 0, 0.4);
  z-index: 10;
  overflow: hidden;
}

.usp-merge-option {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: none;
  background: none;
  color: var(--color-text-secondary);
  font-size: 0.72rem;
  cursor: pointer;
  transition: background 0.1s ease;
}

.usp-merge-option:hover {
  background: var(--color-bg-hover);
}

.usp-merge-option-detail {
  color: var(--color-text-muted);
  font-size: 0.65rem;
}

/* 带 search 的下拉：加宽 + 内容区滚动 */
.usp-merge-dropdown--search {
  min-width: 260px;
  max-height: 320px;
  display: flex;
  flex-direction: column;
}

.usp-merge-search {
  width: 100%;
  padding: 0.45rem 0.65rem;
  border: none;
  border-bottom: 1px solid var(--color-border-subtle);
  background: var(--color-input-bg);
  color: var(--color-text-primary);
  font-size: 0.74rem;
  outline: none;
}

.usp-merge-search::placeholder {
  color: var(--color-text-muted);
}

.usp-merge-list {
  overflow-y: auto;
  max-height: 260px;
}

.usp-merge-group-label {
  padding: 0.3rem 0.65rem 0.15rem;
  font-size: 0.62rem;
  font-weight: 600;
  color: var(--color-text-muted);
  letter-spacing: 0.02em;
  background: var(--color-bg-sunken);
}

/* LLM/算法候选区高亮 */
.usp-merge-option--candidate {
  background: var(--color-link-subtle);
}

.usp-merge-option--candidate:hover {
  background: var(--color-link-border);
}

.usp-merge-option-name {
  flex: 1;
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

/* LLM/high 已推荐的目标，在全量列表里标记 */
.usp-merge-option--recommended {
  background: var(--color-success-bg, rgba(61, 138, 74, 0.08));
}

.usp-merge-option--recommended:hover {
  background: var(--color-success-bg, rgba(61, 138, 74, 0.18));
}

.usp-merge-option-tag {
  margin-left: 0.4rem;
  padding: 0.05rem 0.3rem;
  border-radius: 4px;
  background: var(--color-success-bg, rgba(61, 138, 74, 0.2));
  color: var(--color-success);
  font-size: 0.6rem;
  font-weight: 600;
}

.usp-persisted {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.usp-persisted-toolbar {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 0.5rem;
  flex-wrap: wrap;
}

.usp-filter-tabs {
  display: flex;
  gap: 0.2rem;
  border-radius: 8px;
  background: var(--color-bg-sunken);
  padding: 0.15rem;
}

.usp-filter-tab {
  padding: 0.25rem 0.55rem;
  border: none;
  border-radius: 6px;
  background: none;
  color: var(--color-text-muted);
  font-size: 0.7rem;
  cursor: pointer;
  transition: all 0.12s ease;
}

.usp-filter-tab:hover {
  color: var(--color-text-secondary);
}

.usp-filter-tab.is-active {
  background: var(--color-bg-elevated);
  color: var(--color-text-primary);
  box-shadow: 0 1px 3px rgba(0, 0, 0, 0.2);
}

.usp-persisted-empty {
  display: flex;
  align-items: center;
  justify-content: center;
  gap: 0.4rem;
  padding: 1.2rem 0;
  color: var(--color-text-muted);
  font-size: 0.78rem;
}

.usp-persisted-list {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
}

.usp-row--high {
  box-shadow: inset 3px 0 0 var(--color-success, #3d8a4a);
}

.usp-confidence-badge {
  font-size: 0.62rem;
  font-weight: 600;
  padding: 0.1rem 0.35rem;
  border-radius: 4px;
  background: var(--color-success-bg, rgba(61, 138, 74, 0.15));
  color: var(--color-success);
}

/* target_off_shortlist 警示徽标（board-upgrade spec 方案 B） */
.usp-off-shortlist-badge {
  background: var(--color-warning-bg, rgba(180, 140, 40, 0.15));
  color: var(--color-warning, #b48c28);
}

.usp-merge-empty {
  display: block;
  padding: 0.45rem 0.65rem;
  color: var(--color-text-muted);
  font-size: 0.7rem;
}

.usp-evidence {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 0.25rem;
  font-size: 0.66rem;
  color: var(--color-text-muted);
}

.usp-evidence-label {
  color: var(--color-text-muted);
}

.usp-evidence-chip {
  padding: 0.08rem 0.3rem;
  border-radius: 4px;
  background: var(--color-bg-hover);
  color: var(--color-text-secondary);
}

.usp-item-btn--dismiss {
  border-color: var(--color-border-medium);
  color: var(--color-text-muted);
}

.usp-item-btn--dismiss:hover {
  background: var(--color-bg-hover);
  color: var(--color-danger, #c05050);
}

.usp-divider {
  height: 1px;
  background: var(--color-border-subtle);
  margin: 0.15rem 0;
}

.usp-manual-title {
  font-size: 0.7rem;
  font-weight: 600;
  color: var(--color-text-secondary);
  letter-spacing: 0.02em;
}
</style>
