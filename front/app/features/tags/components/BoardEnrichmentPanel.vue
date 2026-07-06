<script setup lang="ts">
import { ref, watch, computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useBoardEnrichment } from '~/features/tags/composables/useBoardEnrichment'
import { useBoardEnrichmentApi, type ContextGranularity, type ContextRow, type ResultDetailRow, type ResultSector, type ReviewRow, type DataSourceRow } from '~/api/boardEnrichment'
import { useNotify } from '~/composables/useNotify'

const props = defineProps<{
  boardId: number
}>()

const {
  // topic selector
  topics, topicsLoading, selectedTopicId, loadTopics, selectTopic,
  // table 1
  contexts, contextsLoading, regenerating, saveContext, regenerateContext,
  // table 2
  results, resultsLoading, triggering, loadResults, triggerEnrichment,
  // table 3
  reviews, reviewsLoading, loadReviews, saveReviewDeviation, applyReview, createReview,
  // data sources
  dataSources, dataSourcesLoading, loadDataSources, saveDataSource, removeDataSource,
  // misc
  loadAllTopicTables,
} = useBoardEnrichment()

const api = useBoardEnrichmentApi()
const { success: notifySuccess, error: notifyError } = useNotify()

const GRANS: ContextGranularity[] = ['week', 'month', 'year', 'all']

// ── lifecycle: board switch ──────────────────────────────────────────────
async function bootstrap(boardId: number) {
  await Promise.all([loadTopics(boardId), loadDataSources(boardId)])
  if (selectedTopicId.value !== null) {
    await loadAllTopicTables(selectedTopicId.value)
  }
}

void bootstrap(props.boardId)
watch(() => props.boardId, (id) => { void bootstrap(id) })
watch(selectedTopicId, (id) => {
  if (id !== null) void loadAllTopicTables(id)
})

// ── helpers ──────────────────────────────────────────────────────────────
function granularityLabel(g: ContextGranularity): string {
  return { week: '本周', month: '本月', year: '本年', all: '全部' }[g]
}
function sourceLabel(s: string | undefined): string {
  if (!s) return ''
  return ({ manual: '手动', llm_assisted: 'LLM', auto: '自动' } as Record<string, string>)[s] ?? s
}
function sourceIcon(s: string | undefined): string {
  if (s === 'manual') return 'mdi:hand-editing-outline'
  if (s === 'llm_assisted') return 'mdi:robot-outline'
  return 'mdi:flash-outline'
}
function formatDate(s: string | undefined): string {
  if (!s) return '—'
  const d = new Date(s)
  if (Number.isNaN(d.getTime())) return s
  return `${d.getFullYear()}-${String(d.getMonth() + 1).padStart(2, '0')}-${String(d.getDate()).padStart(2, '0')}`
}
function preview(text: string | undefined | null, n = 140): string {
  if (!text) return ''
  return text.length > n ? text.slice(0, n) + '…' : text
}
async function copyText(text: string) {
  try {
    await navigator.clipboard.writeText(text)
    notifySuccess('已复制')
  } catch {
    notifyError('复制失败')
  }
}
function contextOf(g: ContextGranularity): ContextRow | undefined {
  return contexts.value.find(c => c.granularity === g)
}

// ── Table 1: context edit dialog ─────────────────────────────────────────
const editingContext = ref<{ granularity: ContextGranularity; content: string } | null>(null)
function openEditContext(row: ContextRow) {
  editingContext.value = { granularity: row.granularity, content: row.content }
}
async function saveEditingContext() {
  if (!editingContext.value || selectedTopicId.value === null) return
  const { granularity, content } = editingContext.value
  const ok = await saveContext(selectedTopicId.value, granularity, content)
  if (ok) editingContext.value = null
}

// ── Table 2: result detail dialog ────────────────────────────────────────
const viewingResult = ref<{ id: number; loading: boolean; detail: ResultDetailRow | null } | null>(null)
async function openResultDetail(id: number) {
  if (selectedTopicId.value === null) return
  viewingResult.value = { id, loading: true, detail: null }
  const res = await api.getResult(selectedTopicId.value, id)
  if (viewingResult.value) {
    viewingResult.value.loading = false
    if (res.success && res.data) viewingResult.value.detail = res.data
    else notifyError(res.error || '加载详情失败')
  }
}
function sectorCount(sectors: ResultSector[] | null | undefined): number {
  return Array.isArray(sectors) ? sectors.length : 0
}
function jsonString(v: unknown): string {
  if (v == null) return '—'
  try { return JSON.stringify(v, null, 2) } catch { return String(v) }
}

// ── Table 3: review edit / create dialogs ────────────────────────────────
const editingReview = ref<{ id: number; deviation: string } | null>(null)
function openEditReview(row: ReviewRow) {
  editingReview.value = { id: row.id, deviation: row.deviation_summary }
}
async function saveEditingReview() {
  if (!editingReview.value || selectedTopicId.value === null) return
  const { id, deviation } = editingReview.value
  const ok = await saveReviewDeviation(selectedTopicId.value, id, deviation)
  if (ok) editingReview.value = null
}

const creatingReview = ref<{ curr_result_id: number | null; deviation: string; prev_result_id: number | null } | null>(null)
function openCreateReview() {
  const first = results.value[0]
  creatingReview.value = { curr_result_id: first?.id ?? null, deviation: '', prev_result_id: null }
}
async function saveCreatingReview() {
  if (!creatingReview.value || selectedTopicId.value === null) return
  const { curr_result_id, deviation, prev_result_id } = creatingReview.value
  if (curr_result_id === null || !deviation.trim()) {
    notifyError('请选择关联结果并填写偏差说明')
    return
  }
  const body = prev_result_id !== null
    ? { curr_result_id, deviation_summary: deviation, prev_result_id }
    : { curr_result_id, deviation_summary: deviation }
  const ok = await createReview(selectedTopicId.value, body)
  if (ok) creatingReview.value = null
}

// ── Board data source edit dialog ────────────────────────────────────────
const editingSource = ref<{ source_type: string; config_text: string; enabled: boolean; isNew: boolean } | null>(null)
const SOURCE_TYPE_OPTIONS = ['etf_quote', 'exchange_rate', 'gdelt_event'] as const
function openEditSource(row?: DataSourceRow) {
  if (row) {
    editingSource.value = { source_type: row.source_type, config_text: JSON.stringify(row.config ?? {}, null, 2), enabled: row.enabled, isNew: false }
  } else {
    editingSource.value = { source_type: SOURCE_TYPE_OPTIONS[0], config_text: '{}', enabled: true, isNew: true }
  }
}
async function saveEditingSource() {
  if (!editingSource.value) return
  let config: Record<string, unknown> = {}
  try {
    config = editingSource.value.config_text.trim() ? JSON.parse(editingSource.value.config_text) : {}
  } catch {
    notifyError('config 不是合法 JSON')
    return
  }
  const ok = await saveDataSource(props.boardId, {
    source_type: editingSource.value.source_type,
    config,
    enabled: editingSource.value.enabled,
  })
  if (ok) editingSource.value = null
}
function confirmDeleteSource(row: DataSourceRow) {
  if (!confirm(`删除数据源「${row.source_type}」？`)) return
  void removeDataSource(props.boardId, row.source_type)
}

// ── trigger / regenerate guards ──────────────────────────────────────────
async function handleTrigger() {
  if (selectedTopicId.value === null || triggering.value) return
  if (!confirm('手动触发数据增强？\n（约 10-30 秒，需板块已开启增强开关）')) return
  await triggerEnrichment(selectedTopicId.value)
}
async function handleRegenerate(g: ContextGranularity) {
  if (selectedTopicId.value === null || regenerating.value !== null) return
  if (!confirm(`重生成「${granularityLabel(g)}」上下文？\n（约 10-30 秒，调用 LLM 重读新闻）`)) return
  await regenerateContext(selectedTopicId.value, g)
}

const hasTopic = computed(() => selectedTopicId.value !== null)
// silence unused for selectTopic (used via v-model)
void selectTopic
</script>

<template>
  <div class="bep-panel">
    <!-- ── topic selector toolbar ─────────────────────────────────────── -->
    <div class="bep-toolbar">
      <Icon icon="mdi:database-plus-outline" width="15" class="bep-toolbar-icon" />
      <span class="bep-toolbar-title">数据增强</span>
      <span class="bep-divider" />
      <span class="bep-field-label">话题</span>
      <select
        v-if="!topicsLoading"
        v-model.number="selectedTopicId"
        class="bep-select"
        :disabled="topics.length === 0"
      >
        <option :value="null" disabled>选择话题…</option>
        <option v-for="t in topics" :key="t.id" :value="t.id">
          {{ t.label }}（{{ t.status }}）
        </option>
      </select>
      <span v-else class="bep-muted">加载话题…</span>
      <div class="bep-spacer" />
      <button type="button" class="bep-ghost-btn" title="刷新" @click="bootstrap(boardId)">
        <Icon icon="mdi:refresh" width="14" />
      </button>
    </div>

    <p v-if="topics.length === 0 && !topicsLoading" class="bep-empty-hint">
      该板块暂无持久话题。先在「日报」tab 孵化话题后再做数据增强。
    </p>

    <!-- ── topic-dimension tables (only when a topic is selected) ──────── -->
    <template v-if="hasTopic">
      <!-- Table 1: lifeline contexts -->
      <section class="bep-section">
        <header class="bep-section-head">
          <Icon icon="mdi:layers-outline" width="14" />
          <span class="bep-section-title">分层新闻汇总上下文</span>
          <span class="bep-section-meta">只随新闻更新，分析不回写</span>
        </header>
        <div v-if="contextsLoading" class="bep-loading">加载中…</div>
        <div v-else class="bep-cards">
          <div v-for="g in GRANS" :key="g" class="bep-card">
            <template v-if="contextOf(g)">
              <div class="bep-card-head">
                <span class="bep-card-title">{{ granularityLabel(g) }}</span>
                <span class="bep-card-tag">{{ g }}</span>
                <span class="bep-card-spacer" />
                <span class="bep-card-asof">截止 {{ formatDate(contextOf(g)?.as_of_date) }}</span>
                <span class="bep-card-source">
                  <Icon :icon="sourceIcon(contextOf(g)?.source)" width="11" />
                  {{ sourceLabel(contextOf(g)?.source) }}
                </span>
              </div>
              <p class="bep-card-preview">{{ preview(contextOf(g)?.content) }}</p>
              <div class="bep-card-actions">
                <button type="button" class="bep-link-btn" @click="openEditContext(contextOf(g)!)">
                  <Icon icon="mdi:pencil-outline" width="12" /> 编辑
                </button>
                <button
                  type="button"
                  class="bep-link-btn"
                  :disabled="regenerating !== null"
                  @click="handleRegenerate(g)"
                >
                  <Icon icon="mdi:refresh-auto" width="12" />
                  {{ regenerating === g ? '重生成中…' : '重生成' }}
                </button>
              </div>
            </template>
            <template v-else>
              <div class="bep-card-head">
                <span class="bep-card-title bep-muted">{{ granularityLabel(g) }}</span>
                <span class="bep-card-spacer" />
                <span class="bep-card-source bep-muted">未生成</span>
              </div>
              <p class="bep-card-preview bep-muted">该层尚未生成，可手动重生成。</p>
              <div class="bep-card-actions">
                <button
                  type="button"
                  class="bep-link-btn"
                  :disabled="regenerating !== null"
                  @click="handleRegenerate(g)"
                >
                  <Icon icon="mdi:refresh-auto" width="12" />
                  {{ regenerating === g ? '重生成中…' : '重生成' }}
                </button>
              </div>
            </template>
          </div>
        </div>
      </section>

      <!-- Table 2: enrichment results -->
      <section class="bep-section">
        <header class="bep-section-head">
          <Icon icon="mdi:chart-line-variant" width="14" />
          <span class="bep-section-title">增强结果历史</span>
          <span class="bep-section-meta">快照不可变</span>
          <span class="bep-card-spacer" />
          <button
            type="button"
            class="bep-accent-btn"
            :disabled="triggering"
            :title="triggering ? '增强中…' : '手动触发数据增强（约 10-30 秒）'"
            @click="handleTrigger"
          >
            <Icon icon="mdi:play-circle-outline" width="13" />
            {{ triggering ? '增强中…' : '触发增强' }}
          </button>
        </header>
        <div v-if="resultsLoading" class="bep-loading">加载中…</div>
        <div v-else-if="results.length === 0" class="bep-empty-row">暂无增强结果，点「触发增强」生成第一次。</div>
        <div v-else class="bep-rows">
          <button v-for="r in results" :key="r.id" type="button" class="bep-row" @click="openResultDetail(r.id)">
            <span class="bep-row-date">{{ formatDate(r.created_at) }}</span>
            <span class="bep-row-text">{{ preview(r.evolution_assessment, 120) || '（无演进判断）' }}</span>
            <span class="bep-row-badge">{{ sectorCount(r.sectors) }} 板块</span>
            <span class="bep-row-badge">{{ r.tool_calls_count ?? 0 }} 次工具</span>
            <Icon icon="mdi:chevron-right" width="14" class="bep-row-chev" />
          </button>
        </div>
      </section>

      <!-- Table 3: enrichment reviews -->
      <section class="bep-section">
        <header class="bep-section-head">
          <Icon icon="mdi:compare-horizontal" width="14" />
          <span class="bep-section-title">认知演进史</span>
          <span class="bep-section-meta">偏差记录，不回写表1</span>
          <span class="bep-card-spacer" />
          <button type="button" class="bep-ghost-btn" @click="openCreateReview">
            <Icon icon="mdi:plus" width="13" /> 添加批注
          </button>
        </header>
        <div v-if="reviewsLoading" class="bep-loading">加载中…</div>
        <div v-else-if="reviews.length === 0" class="bep-empty-row">暂无认知偏差记录。</div>
        <div v-else class="bep-rows">
          <div v-for="rv in reviews" :key="rv.id" class="bep-row bep-row--review">
            <div class="bep-row-main">
              <span class="bep-row-date">{{ formatDate(rv.created_at) }}</span>
              <span class="bep-row-text">{{ preview(rv.deviation_summary, 160) }}</span>
              <span class="bep-row-tags">
                <span class="bep-pill" :class="rv.applied ? 'bep-pill--success' : 'bep-pill--muted'">
                  {{ rv.applied ? '已采纳' : '未采纳' }}
                </span>
                <span class="bep-pill bep-pill--muted">
                  <Icon :icon="sourceIcon(rv.source)" width="10" />{{ sourceLabel(rv.source) }}
                </span>
              </span>
            </div>
            <div class="bep-row-actions">
              <button type="button" class="bep-link-btn" @click="openEditReview(rv)">
                <Icon icon="mdi:pencil-outline" width="12" /> 编辑
              </button>
              <button
                v-if="!rv.applied"
                type="button"
                class="bep-link-btn"
                @click="applyReview(selectedTopicId!, rv.id)"
              >
                <Icon icon="mdi:check-circle-outline" width="12" /> 采纳
              </button>
            </div>
          </div>
        </div>
      </section>
    </template>

    <!-- ── board-dimension: data sources ──────────────────────────────── -->
    <section class="bep-section">
      <header class="bep-section-head">
        <Icon icon="mdi:connection" width="14" />
        <span class="bep-section-title">板块数据源</span>
        <span class="bep-section-meta">板块级参数，agent 查询工具</span>
        <span class="bep-card-spacer" />
        <button type="button" class="bep-ghost-btn" @click="openEditSource()">
          <Icon icon="mdi:plus" width="13" /> 绑定数据源
        </button>
      </header>
      <div v-if="dataSourcesLoading" class="bep-loading">加载中…</div>
      <div v-else-if="dataSources.length === 0" class="bep-empty-row">未绑定数据源。</div>
      <div v-else class="bep-rows">
        <div v-for="ds in dataSources" :key="ds.id" class="bep-row bep-row--source">
          <div class="bep-row-main">
            <span class="bep-row-badge">{{ ds.source_type }}</span>
            <span class="bep-row-text bep-row-text--mono">{{ preview(jsonString(ds.config), 120) }}</span>
            <span class="bep-pill" :class="ds.enabled ? 'bep-pill--success' : 'bep-pill--muted'">
              {{ ds.enabled ? '启用' : '停用' }}
            </span>
          </div>
          <div class="bep-row-actions">
            <button type="button" class="bep-link-btn" @click="openEditSource(ds)">
              <Icon icon="mdi:pencil-outline" width="12" /> 编辑
            </button>
            <button type="button" class="bep-link-btn bep-link-btn--danger" @click="confirmDeleteSource(ds)">
              <Icon icon="mdi:trash-can-outline" width="12" /> 删除
            </button>
          </div>
        </div>
      </div>
    </section>

    <!-- ── Dialog: edit context ────────────────────────────────────────── -->
    <AppDialog :model-value="editingContext !== null" title="编辑分层上下文" width="640px" @update:model-value="(v) => { if (!v) editingContext = null }">
      <div v-if="editingContext" class="bep-dialog-form">
        <p class="bep-dialog-hint">编辑后 source 标记为「手动」。原 LLM 汇总将被覆盖。</p>
        <label class="bep-field">
          <span class="bep-field-label">content</span>
          <textarea v-model="editingContext.content" class="bep-textarea" rows="14" />
        </label>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="editingContext = null">取消</AppButton>
        <AppButton variant="primary" size="sm" @click="saveEditingContext">保存</AppButton>
      </template>
    </AppDialog>

    <!-- ── Dialog: result detail ───────────────────────────────────────── -->
    <AppDialog :model-value="viewingResult !== null" title="增强结果详情" width="720px" @update:model-value="(v) => { if (!v) viewingResult = null }">
      <div v-if="viewingResult?.loading" class="bep-loading">加载中…</div>
      <div v-else-if="viewingResult?.detail" class="bep-dialog-form">
        <p class="bep-dialog-section-label">演进判断</p>
        <p class="bep-dialog-text">{{ viewingResult.detail.evolution_assessment || '（无）' }}</p>

        <template v-if="sectorCount(viewingResult.detail.sectors) > 0">
          <p class="bep-dialog-section-label">产业切片（{{ sectorCount(viewingResult.detail.sectors) }}）</p>
          <ul class="bep-sectors">
            <li v-for="(s, i) in (viewingResult.detail.sectors as ResultSector[])" :key="i" class="bep-sector">
              <span class="bep-sector-name">{{ s.sector || '未命名' }}</span>
              <span v-if="s.evolution_role" class="bep-sector-role">{{ s.evolution_role }}</span>
              <span v-if="s.current_signal" class="bep-sector-sig">{{ s.current_signal }}</span>
              <span v-if="typeof s.confidence === 'number'" class="bep-sector-conf">置信 {{ s.confidence.toFixed(2) }}</span>
            </li>
          </ul>
        </template>

        <p v-if="viewingResult.detail.causal_chain" class="bep-dialog-section-label">因果链</p>
        <p v-if="viewingResult.detail.causal_chain" class="bep-dialog-text">{{ viewingResult.detail.causal_chain }}</p>

        <p class="bep-dialog-section-label">session_id（点击复制，可在 ai-call-logs 回放）</p>
        <button type="button" class="bep-session" @click="copyText(viewingResult.detail.session_id)">
          <Icon icon="mdi:content-copy" width="12" />
          <span>{{ viewingResult.detail.session_id }}</span>
        </button>

        <details class="bep-details">
          <summary class="bep-dialog-section-label">input_snapshot / tool_calls（原始 JSON）</summary>
          <pre class="bep-pre">{{ jsonString(viewingResult.detail.input_snapshot) }}</pre>
          <pre class="bep-pre">{{ jsonString(viewingResult.detail.tool_calls) }}</pre>
        </details>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="viewingResult = null">关闭</AppButton>
      </template>
    </AppDialog>

    <!-- ── Dialog: edit review deviation ───────────────────────────────── -->
    <AppDialog :model-value="editingReview !== null" title="编辑偏差说明" width="560px" @update:model-value="(v) => { if (!v) editingReview = null }">
      <div v-if="editingReview" class="bep-dialog-form">
        <label class="bep-field">
          <span class="bep-field-label">deviation_summary</span>
          <textarea v-model="editingReview.deviation" class="bep-textarea" rows="6" />
        </label>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="editingReview = null">取消</AppButton>
        <AppButton variant="primary" size="sm" @click="saveEditingReview">保存</AppButton>
      </template>
    </AppDialog>

    <!-- ── Dialog: create review ───────────────────────────────────────── -->
    <AppDialog :model-value="creatingReview !== null" title="手动添加批注" width="560px" @update:model-value="(v) => { if (!v) creatingReview = null }">
      <div v-if="creatingReview" class="bep-dialog-form">
        <p class="bep-dialog-hint">手动批注 source=manual，applied 默认 true。</p>
        <label class="bep-field">
          <span class="bep-field-label">关联结果（curr_result_id）<span class="bep-req">*</span></span>
          <select v-model.number="creatingReview.curr_result_id" class="bep-select">
            <option :value="null" disabled>选择结果…</option>
            <option v-for="r in results" :key="r.id" :value="r.id">#{{ r.id }} · {{ formatDate(r.created_at) }}</option>
          </select>
        </label>
        <label class="bep-field">
          <span class="bep-field-label">上一次结果（prev_result_id，可选）</span>
          <select v-model.number="creatingReview.prev_result_id" class="bep-select">
            <option :value="null">无</option>
            <option v-for="r in results" :key="r.id" :value="r.id">#{{ r.id }} · {{ formatDate(r.created_at) }}</option>
          </select>
        </label>
        <label class="bep-field">
          <span class="bep-field-label">偏差说明 <span class="bep-req">*</span></span>
          <textarea v-model="creatingReview.deviation" class="bep-textarea" rows="5" placeholder="为什么变了 / 用户观察…" />
        </label>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="creatingReview = null">取消</AppButton>
        <AppButton variant="primary" size="sm" @click="saveCreatingReview">添加</AppButton>
      </template>
    </AppDialog>

    <!-- ── Dialog: edit/upsert data source ─────────────────────────────── -->
    <AppDialog :model-value="editingSource !== null" :title="editingSource?.isNew ? '绑定数据源' : '编辑数据源'" width="560px" @update:model-value="(v) => { if (!v) editingSource = null }">
      <div v-if="editingSource" class="bep-dialog-form">
        <label class="bep-field">
          <span class="bep-field-label">source_type</span>
          <select v-model="editingSource.source_type" class="bep-select" :disabled="!editingSource.isNew">
            <option v-for="opt in SOURCE_TYPE_OPTIONS" :key="opt" :value="opt">{{ opt }}</option>
          </select>
        </label>
        <label class="bep-field">
          <span class="bep-field-label">config（JSON）</span>
          <textarea v-model="editingSource.config_text" class="bep-textarea bep-textarea--mono" rows="8" spellcheck="false" />
        </label>
        <label class="bep-field bep-field--row">
          <AppToggle v-model="editingSource.enabled" />
          <span class="bep-field-label">启用</span>
        </label>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="editingSource = null">取消</AppButton>
        <AppButton variant="primary" size="sm" @click="saveEditingSource">保存</AppButton>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
.bep-panel { display: flex; flex-direction: column; gap: 1rem; }

/* toolbar */
.bep-toolbar { display: flex; align-items: center; gap: 0.5rem; padding: 0.6rem 0.75rem; border: 1px solid var(--color-border-subtle); border-radius: 12px; background: var(--color-bg-elevated); }
.bep-toolbar-icon { color: var(--color-text-muted); }
.bep-toolbar-title { font-size: 0.82rem; font-weight: 600; color: var(--color-text-primary); letter-spacing: 0.02em; }
.bep-divider { width: 1px; height: 14px; background: var(--color-border-medium); margin: 0 0.25rem; }
.bep-field-label { font-size: 0.68rem; color: var(--color-text-secondary); letter-spacing: 0.02em; }
.bep-select { padding: 0.25rem 0.5rem; font-size: 0.78rem; border: 1px solid var(--color-input-border); border-radius: 6px; background: var(--color-input-bg); color: var(--color-text-primary); outline: none; max-width: 240px; }
.bep-select:focus { border-color: var(--color-input-focus); }
.bep-spacer { flex: 1; }
.bep-muted { color: var(--color-text-muted); }

.bep-ghost-btn { display: inline-flex; align-items: center; gap: 0.25rem; padding: 0.2rem 0.5rem; border-radius: 6px; border: 1px solid var(--color-border-medium); background: var(--color-bg-sunken); color: var(--color-text-primary); font-size: 0.68rem; cursor: pointer; transition: all 0.12s ease; font-family: inherit; }
.bep-ghost-btn:hover { background: var(--color-bg-hover); border-color: var(--color-border-strong); }
.bep-ghost-btn:disabled { opacity: 0.5; cursor: not-allowed; }

.bep-accent-btn { display: inline-flex; align-items: center; gap: 0.25rem; padding: 0.25rem 0.6rem; border-radius: 6px; border: 1px solid var(--color-accent); background: var(--color-accent-subtle); color: var(--color-accent); font-size: 0.7rem; cursor: pointer; transition: all 0.12s ease; font-family: inherit; }
.bep-accent-btn:hover:not(:disabled) { border-color: var(--color-accent-hover); }
.bep-accent-btn:disabled { opacity: 0.6; cursor: not-allowed; }

/* sections */
.bep-section { display: flex; flex-direction: column; gap: 0.5rem; padding: 0.85rem 0.95rem; border: 1px solid var(--color-border-subtle); border-radius: 12px; background: var(--color-bg-hover); }
.bep-section-head { display: flex; align-items: center; gap: 0.4rem; color: var(--color-text-secondary); }
.bep-section-title { font-size: 0.78rem; font-weight: 600; color: var(--color-text-secondary); }
.bep-section-meta { font-size: 0.65rem; color: var(--color-text-muted); }

.bep-loading { font-size: 0.72rem; color: var(--color-text-muted); padding: 0.5rem 0; }
.bep-empty-hint { font-size: 0.75rem; color: var(--color-text-muted); padding: 0.5rem 0.75rem; }
.bep-empty-row { font-size: 0.72rem; color: var(--color-text-muted); padding: 0.5rem 0; }

/* context cards */
.bep-cards { display: grid; grid-template-columns: repeat(auto-fit, minmax(220px, 1fr)); gap: 0.5rem; }
.bep-card { display: flex; flex-direction: column; gap: 0.35rem; padding: 0.6rem 0.7rem; border: 1px solid var(--color-border-medium); border-radius: 8px; background: var(--color-bg-elevated); transition: border-color 0.12s ease; }
.bep-card:hover { border-color: var(--color-border-strong); }
.bep-card-head { display: flex; align-items: center; gap: 0.3rem; }
.bep-card-title { font-size: 0.72rem; font-weight: 600; color: var(--color-text-primary); }
.bep-card-tag { font-size: 0.58rem; color: var(--color-text-muted); padding: 0 0.3rem; border-radius: 999px; background: var(--color-bg-sunken); font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
.bep-card-spacer { flex: 1; }
.bep-card-asof { font-size: 0.62rem; color: var(--color-text-muted); }
.bep-card-source { display: inline-flex; align-items: center; gap: 0.2rem; font-size: 0.62rem; color: var(--color-text-muted); }
.bep-card-preview { font-size: 0.72rem; line-height: 1.5; color: var(--color-text-secondary); margin: 0; flex: 1; }
.bep-card-actions { display: flex; gap: 0.4rem; }

.bep-link-btn { display: inline-flex; align-items: center; gap: 0.2rem; padding: 0.15rem 0.35rem; border: none; background: none; color: var(--color-text-muted); font-size: 0.66rem; cursor: pointer; border-radius: 4px; transition: all 0.12s ease; font-family: inherit; }
.bep-link-btn:hover:not(:disabled) { color: var(--color-accent); background: var(--color-accent-subtle); }
.bep-link-btn:disabled { opacity: 0.5; cursor: not-allowed; }
.bep-link-btn--danger:hover:not(:disabled) { color: var(--color-error); background: var(--color-warning-subtle); }

/* rows (results / reviews / data sources) */
.bep-rows { display: flex; flex-direction: column; gap: 0.35rem; }
.bep-row { display: flex; align-items: center; gap: 0.5rem; padding: 0.5rem 0.65rem; border: 1px solid var(--color-border-subtle); border-radius: 8px; background: var(--color-bg-elevated); cursor: pointer; transition: border-color 0.12s ease; text-align: left; width: 100%; font-family: inherit; }
.bep-row:hover { border-color: var(--color-border-medium); }
.bep-row--review, .bep-row--source { flex-wrap: wrap; cursor: default; }
.bep-row--review:hover, .bep-row--source:hover { border-color: var(--color-border-subtle); }
.bep-row-main { display: flex; align-items: center; gap: 0.5rem; flex: 1; min-width: 0; flex-wrap: wrap; }
.bep-row-actions { display: flex; gap: 0.2rem; }
.bep-row-date { font-size: 0.62rem; color: var(--color-text-muted); white-space: nowrap; }
.bep-row-text { font-size: 0.74rem; color: var(--color-text-secondary); flex: 1; min-width: 0; overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bep-row-text--mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.68rem; }
.bep-row-badge { font-size: 0.6rem; color: var(--color-text-muted); padding: 0.05rem 0.4rem; border-radius: 999px; background: var(--color-bg-sunken); white-space: nowrap; }
.bep-row-tags { display: inline-flex; gap: 0.25rem; }
.bep-row-chev { color: var(--color-text-muted); }
.bep-pill { display: inline-flex; align-items: center; gap: 0.2rem; font-size: 0.58rem; padding: 0.05rem 0.4rem; border-radius: 999px; }
.bep-pill--success { color: var(--color-success); background: var(--color-success-subtle); }
.bep-pill--muted { color: var(--color-text-muted); background: var(--color-bg-sunken); }

/* dialogs */
.bep-dialog-form { display: flex; flex-direction: column; gap: 0.75rem; }
.bep-dialog-hint { font-size: 0.68rem; color: var(--color-text-muted); margin: 0; }
.bep-dialog-section-label { font-size: 0.66rem; font-weight: 600; color: var(--color-text-secondary); letter-spacing: 0.04em; text-transform: uppercase; margin: 0.25rem 0 0.2rem; }
.bep-dialog-text { font-size: 0.78rem; line-height: 1.55; color: var(--color-text-primary); margin: 0; white-space: pre-wrap; }
.bep-field { display: flex; flex-direction: column; gap: 0.3rem; }
.bep-field--row { flex-direction: row; align-items: center; gap: 0.5rem; }
.bep-req { color: var(--color-accent); }
.bep-textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--color-input-border); border-radius: 8px; background: var(--color-input-bg); color: var(--color-text-primary); font-size: 0.78rem; padding: 0.5rem 0.7rem; outline: none; resize: vertical; font-family: inherit; line-height: 1.5; }
.bep-textarea:focus { border-color: var(--color-input-focus); }
.bep-textarea--mono { font-family: ui-monospace, SFMono-Regular, Menlo, monospace; font-size: 0.72rem; }
.bep-session { display: inline-flex; align-items: center; gap: 0.3rem; padding: 0.3rem 0.5rem; border: 1px dashed var(--color-border-medium); border-radius: 6px; background: var(--color-bg-sunken); color: var(--color-text-secondary); font-size: 0.68rem; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; cursor: pointer; max-width: 100%; overflow: hidden; }
.bep-session span { overflow: hidden; text-overflow: ellipsis; white-space: nowrap; }
.bep-session:hover { border-color: var(--color-accent); color: var(--color-accent); }
.bep-sectors { list-style: none; margin: 0; padding: 0; display: flex; flex-direction: column; gap: 0.3rem; }
.bep-sector { display: flex; flex-wrap: wrap; align-items: center; gap: 0.4rem; padding: 0.35rem 0.5rem; border: 1px solid var(--color-border-subtle); border-radius: 6px; }
.bep-sector-name { font-size: 0.74rem; font-weight: 600; color: var(--color-text-primary); }
.bep-sector-role { font-size: 0.62rem; color: var(--color-accent); }
.bep-sector-sig { font-size: 0.66rem; color: var(--color-text-secondary); }
.bep-sector-conf { font-size: 0.6rem; color: var(--color-text-muted); margin-left: auto; }
.bep-details { border: 1px solid var(--color-border-subtle); border-radius: 6px; padding: 0.4rem 0.55rem; }
.bep-details summary { cursor: pointer; }
.bep-pre { font-size: 0.66rem; line-height: 1.5; color: var(--color-text-secondary); background: var(--color-bg-sunken); padding: 0.4rem 0.55rem; border-radius: 6px; overflow-x: auto; margin: 0.3rem 0 0; max-height: 200px; font-family: ui-monospace, SFMono-Regular, Menlo, monospace; }
</style>
