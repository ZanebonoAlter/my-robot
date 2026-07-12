<script setup lang="ts">
import { ref, computed, watch } from 'vue'
import { Icon } from '@iconify/vue'
import { useBoardEnrichment } from '~/features/tags/composables/useBoardEnrichment'
import type { ContextGranularity, DataSourceRow } from '~/api/boardEnrichment'
import { useNotify } from '~/composables/useNotify'
import EvolutionReport from './EvolutionReport.vue'
import DebateSection from './DebateSection.vue'

/**
 * 数据增强 · 认知工作台。
 *
 * 主线重定位后（金融涨跌 → 演进定位），面板分两个子区块：
 *  - 「新闻背景」：循环A 新闻记忆（周期分层 contexts），只随新闻变，分析不回写。
 *  - 「演进报告」：EvolutionReport，消费最新 result 详情 + reviews，报刊式呈现
 *    四档定位（强化/转折/扩散/衰减）+ 跨泳道信号 + 双类引用。
 *
 * 旧的「走向预测卡片」（sectors direction/trigger）与「兑现复盘」（review verdict
 * hit/part/miss）已被演进报告取代并移除。DebateSection（FinGenius 个股辩论）作为
 * 「金融可选模块 · 独立于演进主线」默认折叠保留（tasks 4b.7 降级标注）。
 */
const props = defineProps<{
  boardId: number
}>()

const {
  // topic selector
  topics, topicsLoading, selectedTopicId, loadTopics,
  // table 1
  contexts, contextsLoading, regenerating, saveContext, regenerateContext,
  // table 2
  results, triggering, triggerEnrichment,
  latestResultId, latestResultDetail, latestResultDetailLoading,
  // table 3
  reviews,
  // data sources
  dataSources, loadDataSources, saveDataSource, removeDataSource,
  // stock debates
  debates, debateTriggering, debateError, debateStage, loadDebates, triggerDebate,
  // workbench UI
  selectedGran, selectedPeriodIdx, periodList, currentContext,
  setGran, shiftPeriod,
  // misc
  loadAllTopicTables,
} = useBoardEnrichment()

const { success: notifySuccess, error: notifyError } = useNotify()

// ── 子区块切换：新闻背景 | 演进报告 ─────────────────────────────────────
const subtab = ref<'news' | 'evolution'>('evolution')

// ── lifecycle: board switch ──────────────────────────────────────────────
async function bootstrap(boardId: number) {
  await Promise.all([loadTopics(boardId), loadDataSources(boardId)])
  if (selectedTopicId.value !== null) {
    await loadAllTopicTables(selectedTopicId.value)
  }
}
void bootstrap(props.boardId)
watch(() => props.boardId, (id) => { void bootstrap(id) })
watch(selectedTopicId, (id) => { if (id !== null) void loadAllTopicTables(id) })

const selectedTopic = computed(() => topics.value.find((t) => t.id === selectedTopicId.value) ?? null)
const hasTopic = computed(() => selectedTopicId.value !== null)

// ── ① 周期筛选器 helpers ─────────────────────────────────────────────────
type PickerGran = 'week' | 'month' | 'year'
const GRANS: { id: PickerGran; label: string }[] = [
  { id: 'week', label: '按周' },
  { id: 'month', label: '按月' },
  { id: 'year', label: '按年' },
]

function formatPeriod(period: string | undefined, gran: string): string {
  if (!period) return '—'
  if (gran === 'week') {
    const m = /^(\d{4})-W(\d{1,2})$/.exec(period)
    if (m) return `${m[1] ?? ''} 第 ${parseInt(m[2] ?? '', 10)} 周`
    return period
  }
  if (gran === 'month') return period
  if (gran === 'year') return period
  return period
}
const periodLabel = computed(() => formatPeriod(currentContext.value?.period, selectedGran.value))
const isLatest = computed(() => selectedPeriodIdx.value === 0)

// ── ① narrative 内联编辑 ─────────────────────────────────────────────────
const editingNarrative = ref(false)
const narrativeDraft = ref('')
function startEditNarrative() {
  narrativeDraft.value = currentContext.value?.content ?? ''
  editingNarrative.value = true
}
async function saveNarrative() {
  if (selectedTopicId.value === null || !currentContext.value) return
  const ok = await saveContext(selectedTopicId.value, selectedGran.value as ContextGranularity, currentContext.value.period, narrativeDraft.value)
  if (ok) editingNarrative.value = false
}
function cancelNarrative() { editingNarrative.value = false }

async function handleRegenerate() {
  if (selectedTopicId.value === null || regenerating.value) return
  const period = currentContext.value?.period
  if (!confirm(`重新汇总「${periodLabel.value}」？\n（约 10-30 秒，调用 LLM 重读新闻）`)) return
  await regenerateContext(selectedTopicId.value, selectedGran.value as ContextGranularity, period)
}

// ①narrative 被演进报告引用次数：统计 result.sectors.evidence 里命中当前 context 的条数
const narrativeCiteCount = computed(() => {
  const ctx = currentContext.value
  if (!ctx) return 0
  const evList = latestResultDetail.value?.sectors?.evidence
  if (!Array.isArray(evList)) return 0
  let n = 0
  for (const ev of evList) {
    if (ev.context_id != null && String(ev.context_id) === String(ctx.id)) n++
    else if (ev.period && ev.period === ctx.period) n++
  }
  return n
})

// ── dialogs：数据源 ──────────────────────────────────────────────────────
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
  try { config = editingSource.value.config_text.trim() ? JSON.parse(editingSource.value.config_text) : {} }
  catch { notifyError('config 不是合法 JSON'); return }
  const ok = await saveDataSource(props.boardId, { source_type: editingSource.value.source_type, config, enabled: editingSource.value.enabled })
  if (ok) editingSource.value = null
}
function confirmDeleteSource(sourceType: string) {
  if (!confirm(`删除数据源「${sourceType}」？`)) return
  void removeDataSource(props.boardId, sourceType)
}

// ── 触发增强 / session_id 复制 ───────────────────────────────────────────
async function handleTrigger() {
  if (selectedTopicId.value === null || triggering.value) return
  if (!confirm('手动触发数据增强？\n（约 10-30 秒，需板块已开启增强开关）')) return
  await triggerEnrichment(selectedTopicId.value)
}
async function copySession() {
  const sid = latestResultDetail.value?.session_id
  if (!sid) return
  try { await navigator.clipboard.writeText(sid); notifySuccess('session_id 已复制') }
  catch { notifyError('复制失败') }
}

// ── ④ debate 触发（挂在最新 result） ─────────────────────────────────────
async function handleDebateTrigger() {
  if (selectedTopicId.value === null || latestResultId.value === null) return
  await triggerDebate(latestResultId.value)
}
async function handleDebateRetry() {
  if (latestResultId.value === null) return
  await loadDebates(latestResultId.value)
}
</script>

<template>
  <div class="ew-panel">
    <!-- ── topic 选择条（板下多话题，挑一个进工作台） ───────────────────── -->
    <div class="ew-toolbar">
      <Icon icon="mdi:database-plus-outline" width="15" class="ew-toolbar-icon" />
      <span class="ew-toolbar-title">数据增强 · 认知工作台</span>
      <span class="ew-divider" />
      <span class="ew-field-label">话题</span>
      <select
        v-if="!topicsLoading"
        v-model.number="selectedTopicId"
        class="ew-select"
        :disabled="topics.length === 0"
      >
        <option :value="null" disabled>选择话题…</option>
        <option v-for="t in topics" :key="t.id" :value="t.id">{{ t.label }}（{{ t.status }}）</option>
      </select>
      <span v-else class="ew-muted">加载话题…</span>
      <span class="ew-spacer" />
      <button type="button" class="ew-ghost-btn" title="刷新" @click="bootstrap(boardId)">
        <Icon icon="mdi:refresh" width="14" />
      </button>
    </div>

    <p v-if="topics.length === 0 && !topicsLoading" class="ew-empty-hint">
      该板块暂无持久话题。先在「日报」tab 孵化话题后再做数据增强。
    </p>

    <template v-if="hasTopic">
      <!-- 紧凑刊头：topic + 状态 + 触发按钮（演进报告自带完整 masthead，此处只作操作入口） -->
      <header class="masthead">
        <div class="eyebrow">持久话题 #{{ selectedTopicId }}</div>
        <h1 class="serif masthead-title">{{ selectedTopic?.label ?? '未命名话题' }}</h1>
        <div class="lede">
          <span v-if="selectedTopic?.status === 'active'" class="status-pill evolving"><span class="dot" />演进中</span>
          <span v-else class="status-pill static">{{ selectedTopic?.status ?? '—' }}</span>
          <span>最近一次分析 <b>{{ latestResultDetail?.created_at ? new Date(latestResultDetail.created_at).toISOString().slice(0, 10) : '—' }}</b></span>
          <span class="muted">·</span>
          <span class="muted">第 {{ results.length }} 轮 · 已复盘 {{ reviews.length }} 条</span>
          <span class="trigger-wrap">
            <button type="button" class="btn btn-primary" :disabled="triggering" @click="handleTrigger">
              <Icon icon="mdi:play" width="13" />
              {{ triggering ? '分析中…' : '▶ 重新分析' }}
            </button>
          </span>
        </div>
      </header>

      <!-- 子区块切换：新闻背景 | 演进报告 -->
      <nav class="subtabs">
        <button type="button" class="subtab" :class="{ active: subtab === 'evolution' }" @click="subtab = 'evolution'">
          <Icon icon="mdi:newspaper-variant-multiple-outline" width="14" /> 演进报告
        </button>
        <button type="button" class="subtab" :class="{ active: subtab === 'news' }" @click="subtab = 'news'">
          <Icon icon="mdi:archive-outline" width="14" /> 新闻背景
        </button>
      </nav>

      <!-- ── 演进报告（主线） ─────────────────────────────────────────── -->
      <section v-if="subtab === 'evolution'" class="block">
        <EvolutionReport
          :result="latestResultDetail"
          :reviews="reviews"
          :topic-label="selectedTopic?.label ?? undefined"
          :loading="latestResultDetailLoading"
        />
        <div v-if="latestResultDetail" class="outlook-meta">
          <span class="link-btn" @click="copySession">
            <Icon icon="mdi:content-copy" width="12" /> session_id：{{ latestResultDetail.session_id ?? '—' }}
          </span>
        </div>
      </section>

      <!-- ── 新闻背景（循环A 新闻记忆） ──────────────────────────────── -->
      <section v-else-if="subtab === 'news'" id="sec1" class="block">
        <div class="block-head">
          <h2 class="serif">新闻背景</h2>
          <span class="helper">新闻记忆 · 只随新闻变，分析不回写</span>
        </div>

        <!-- 周期筛选器 -->
        <div class="period-picker">
          <div class="gran-select">
            <button
              v-for="g in GRANS"
              :key="g.id"
              type="button"
              :class="{ on: selectedGran === g.id }"
              @click="setGran(g.id)"
            >{{ g.label }}</button>
          </div>
          <div class="period-nav">
            <button type="button" title="上一周期" :disabled="selectedPeriodIdx >= periodList.length - 1" @click="shiftPeriod(1)">‹</button>
            <span class="cur">{{ periodList.length ? periodLabel : '无历史周期' }}</span>
            <button type="button" title="下一周期" :disabled="selectedPeriodIdx <= 0" @click="shiftPeriod(-1)">›</button>
          </div>
          <span v-if="periodList.length" class="fresh-tag" :class="isLatest ? 'fresh-latest' : 'fresh-stale'">
            {{ isLatest ? '最新周期' : '历史周期' }}
          </span>
          <span class="muted ew-period-count">共 {{ periodList.length }} 个历史周期可翻</span>
        </div>

        <!-- 叙事 -->
        <div class="narrative">
          <div id="seg-narrative" class="seg">
            <template v-if="contextsLoading">
              <div class="seg-h-row"><div class="seg-h serif">叙事脉络</div></div>
              <div class="seg-b muted">加载中…</div>
            </template>
            <template v-else-if="!currentContext">
              <div class="seg-h-row"><div class="seg-h serif">该周期尚未生成汇总</div></div>
              <div class="seg-b muted">没有这一周期的新闻汇总。点「重新汇总」让 AI 重读新闻生成。</div>
            </template>
            <template v-else-if="!editingNarrative">
              <div class="seg-h-row">
                <div class="seg-h serif">叙事脉络</div>
                <span class="seg-cited" :class="{ empty: narrativeCiteCount === 0 }">
                  {{ narrativeCiteCount > 0 ? `被演进报告引用 ${narrativeCiteCount} 处` : '未被报告引用' }}
                </span>
              </div>
              <div class="seg-b">{{ currentContext.content }}</div>
              <span class="seg-edit" title="编辑" @click="startEditNarrative">✎</span>
            </template>
            <template v-else>
              <div class="seg-h-row"><div class="seg-h serif">编辑叙事</div></div>
              <textarea v-model="narrativeDraft" class="seg-textarea" />
              <div class="seg-actions">
                <button type="button" class="btn btn-primary btn-sm" @click="saveNarrative">保存</button>
                <button type="button" class="btn btn-ghost btn-sm" @click="cancelNarrative">取消</button>
              </div>
            </template>
          </div>

          <div v-if="currentContext" class="narrative-foot">
            <span>来源：{{ currentContext.source === 'manual' ? '手动' : (currentContext.source === 'llm_assisted' ? 'AI 汇总' : currentContext.source) }}</span>
            <span>·</span>
            <span>截止 {{ currentContext.as_of_date ? new Date(currentContext.as_of_date).toISOString().slice(0, 10) : '—' }}</span>
            <span style="margin-left:auto">
              <button type="button" class="btn btn-ghost btn-sm" :disabled="regenerating !== null" @click="handleRegenerate">
                <Icon icon="mdi:refresh" width="12" />
                {{ regenerating !== null ? '汇总中…' : '↻ 重新汇总' }}
              </button>
            </span>
          </div>
        </div>
      </section>

      <!-- ── 金融可选模块：FinGenius 个股辩论（默认折叠 · 独立于演进主线） ── -->
      <section class="block">
        <details class="debate-fold">
          <summary>
            <span class="df-tag">金融可选模块</span>
            <span class="df-title">FinGenius 个股辩论</span>
            <span class="df-note-inline">独立于演进主线 · 仅金融话题</span>
          </summary>
          <div class="df-body">
            <p class="df-hint">该模块接 FinGenius 对个股做 6 角色辩论，属金融可选能力，<b>不参与演进主线定位</b>。仅金融话题有意义。</p>
            <DebateSection
              :stage="debateStage"
              :debates="debates"
              :sectors="null"
              :triggering="debateTriggering"
              :error-msg="debateError"
              @trigger="handleDebateTrigger"
              @retry="handleDebateRetry"
            />
          </div>
        </details>
      </section>

      <!-- ── 数据源与参数（折叠高级） ─────────────────────────────────── -->
      <section class="block">
        <details class="adv">
          <summary>数据源与参数（高级 · 一般不用动）</summary>
          <div class="adv-body">
            <div class="adv-row">
              <span class="kv" style="margin-right:.4rem">已绑数据源：</span>
              <template v-if="dataSources.length">
                <span v-for="ds in dataSources" :key="ds.id" class="src-chip" @click="openEditSource(ds)">
                  {{ ds.source_type }} <span :class="ds.enabled ? 'ok' : 'muted'">{{ ds.enabled ? '✓' : '✕' }}</span>
                </span>
              </template>
              <span v-else class="muted">未绑定</span>
              <button type="button" class="btn btn-ghost btn-sm" @click="openEditSource()">+ 绑定</button>
            </div>
            <div class="kv">话题状态：<b>{{ selectedTopic?.status ?? '—' }}</b> · 历史上下文：周 {{ contexts.filter(c => c.granularity==='week').length }} / 月 {{ contexts.filter(c => c.granularity==='month').length }} / 年 {{ contexts.filter(c => c.granularity==='year').length }}</div>
          </div>
        </details>
      </section>
    </template>

    <!-- ── Dialog: 绑定/编辑数据源 ─────────────────────────────────────── -->
    <AppDialog :model-value="editingSource !== null" :title="editingSource?.isNew ? '绑定数据源' : '编辑数据源'" width="560px" @update:model-value="(v) => { if (!v) editingSource = null }">
      <div v-if="editingSource" class="ew-dialog-form">
        <label class="ew-field">
          <span class="ew-field-label">source_type</span>
          <select v-model="editingSource.source_type" class="ew-select" :disabled="!editingSource.isNew">
            <option v-for="opt in SOURCE_TYPE_OPTIONS" :key="opt" :value="opt">{{ opt }}</option>
          </select>
        </label>
        <label class="ew-field">
          <span class="ew-field-label">config（JSON）</span>
          <textarea v-model="editingSource.config_text" class="ew-textarea ew-textarea--mono" rows="8" spellcheck="false" />
        </label>
        <label class="ew-field ew-field--row">
          <AppToggle v-model="editingSource.enabled" />
          <span class="ew-field-label">启用</span>
        </label>
        <button v-if="!editingSource.isNew" type="button" class="btn btn-ghost btn-sm" @click="confirmDeleteSource(editingSource.source_type)">删除该数据源</button>
      </div>
      <template #footer>
        <AppButton variant="ghost" size="sm" @click="editingSource = null">取消</AppButton>
        <AppButton variant="primary" size="sm" @click="saveEditingSource">保存</AppButton>
      </template>
    </AppDialog>
  </div>
</template>

<style scoped>
.ew-panel { display: flex; flex-direction: column; gap: 1rem; max-width: 960px; margin: 0 auto; }

/* topic 工具条 */
.ew-toolbar { display: flex; align-items: center; gap: 0.5rem; padding: 0.6rem 0.75rem; border: 1px solid var(--color-border-subtle); border-radius: 12px; background: var(--color-bg-elevated); }
.ew-toolbar-icon { color: var(--color-text-muted); }
.ew-toolbar-title { font-size: 0.82rem; font-weight: 600; color: var(--color-text-primary); }
.ew-divider { width: 1px; height: 14px; background: var(--color-border-medium); margin: 0 0.25rem; }
.ew-field-label { font-size: 0.68rem; color: var(--color-text-secondary); }
.ew-select { padding: 0.25rem 0.5rem; font-size: 0.78rem; border: 1px solid var(--color-input-border); border-radius: 6px; background: var(--color-input-bg); color: var(--color-text-primary); outline: none; max-width: 320px; }
.ew-select:focus { border-color: var(--color-input-focus); }
.ew-spacer { flex: 1; }
.ew-muted { color: var(--color-text-muted); }
.ew-ghost-btn { display: inline-flex; align-items: center; gap: 0.25rem; padding: 0.2rem 0.5rem; border-radius: 6px; border: 1px solid var(--color-border-medium); background: var(--color-bg-sunken); color: var(--color-text-primary); font-size: 0.68rem; cursor: pointer; font-family: inherit; }
.ew-ghost-btn:hover { background: var(--color-bg-hover); }
.ew-empty-hint { font-size: 0.78rem; color: var(--color-text-muted); padding: 0.5rem 0.75rem; }

/* 紧凑刊头 */
.masthead { border-bottom: 2px solid var(--color-text-primary); padding-bottom: 1rem; margin-bottom: 0.5rem; }
.masthead .eyebrow { font-size: 11px; letter-spacing: 0.18em; text-transform: uppercase; color: var(--color-accent); font-weight: 600; margin-bottom: 0.4rem; }
.masthead-title { font-size: 1.6rem; line-height: 1.2; margin: 0 0 0.6rem; font-weight: 700; letter-spacing: -0.01em; color: var(--color-text-primary); }
.lede { display: flex; align-items: center; gap: 0.6rem; flex-wrap: wrap; font-size: 13px; color: var(--color-text-secondary); }
.lede b { color: var(--color-text-primary); }
.status-pill { display: inline-flex; align-items: center; gap: 5px; padding: 2px 9px; border-radius: 999px; font-size: 11px; font-weight: 600; }
.status-pill.evolving { background: var(--color-warning-subtle); color: var(--color-warning); }
.status-pill.static { background: var(--color-bg-sunken); color: var(--color-text-muted); }
.status-pill .dot { width: 6px; height: 6px; border-radius: 50%; background: currentColor; animation: ew-pulse 1.6s infinite; }
@keyframes ew-pulse { 0%,100%{opacity:1;} 50%{opacity:.35;} }
.trigger-wrap { margin-left: auto; }

/* 子区块切换 */
.subtabs { display: flex; gap: 0.25rem; padding: 0.35rem; background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-radius: 10px; box-shadow: var(--shadow-subtle); }
.subtab { flex: 1; display: inline-flex; align-items: center; justify-content: center; gap: 0.35rem; padding: 0.5rem 0.8rem; border: none; background: none; color: var(--color-text-muted); font-size: 13px; font-weight: 600; cursor: pointer; border-radius: 7px; transition: background 0.15s, color 0.15s; font-family: inherit; }
.subtab:hover { background: var(--color-bg-hover); color: var(--color-text-secondary); }
.subtab.active { background: var(--color-accent); color: #fff; }

/* 通用 btn */
.btn { display: inline-flex; align-items: center; gap: 6px; font-family: inherit; cursor: pointer; border: none; border-radius: 8px; font-weight: 600; font-size: 13px; padding: 7px 14px; transition: background 0.15s, opacity 0.15s, transform 0.1s; }
.btn:active { transform: translateY(1px); }
.btn-primary { background: var(--color-accent); color: #fff; }
.btn-primary:hover { background: var(--color-accent-hover); }
.btn-ghost { background: transparent; color: var(--color-text-secondary); border: 1px solid var(--color-border-medium); }
.btn-ghost:hover { background: var(--color-bg-hover); color: var(--color-text-primary); }
.btn-sm { padding: 4px 10px; font-size: 12px; }
.btn:disabled { opacity: 0.4; cursor: not-allowed; }

/* 区块 */
.block { margin-bottom: 1.5rem; }
.block-head { display: flex; align-items: baseline; gap: 0.6rem; margin-bottom: 0.9rem; padding-bottom: 0.4rem; border-bottom: 1px solid var(--color-border-subtle); }
.block-head h2 { font-size: 1.1rem; margin: 0; font-weight: 700; color: var(--color-text-primary); }
.block-head .helper { font-size: 12px; color: var(--color-text-muted); }

/* ① 周期筛选器 */
.period-picker { display: flex; align-items: center; gap: 0.75rem; flex-wrap: wrap; background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-radius: 12px; padding: 0.6rem 0.9rem; margin-bottom: 1.1rem; box-shadow: var(--shadow-subtle); }
.gran-select { display: inline-flex; border: 1px solid var(--color-border-medium); border-radius: 8px; overflow: hidden; }
.gran-select button { border: none; background: var(--color-bg-base); color: var(--color-text-secondary); font-size: 12.5px; padding: 5px 14px; cursor: pointer; font-family: inherit; font-weight: 600; }
.gran-select button.on { background: var(--color-accent); color: #fff; }
.period-nav { display: inline-flex; align-items: center; gap: 0.5rem; }
.period-nav .cur { font-size: 14px; font-weight: 600; min-width: 120px; text-align: center; color: var(--color-text-primary); }
.period-nav button { width: 26px; height: 26px; border: 1px solid var(--color-border-medium); background: var(--color-bg-base); color: var(--color-text-secondary); border-radius: 6px; cursor: pointer; font-family: inherit; }
.period-nav button:hover:not(:disabled) { border-color: var(--color-accent); color: var(--color-accent); }
.period-nav button:disabled { opacity: 0.3; cursor: not-allowed; }
.fresh-tag { font-size: 11px; padding: 2px 8px; border-radius: 999px; }
.fresh-latest { background: var(--color-success-subtle); color: var(--color-success); }
.fresh-stale { background: var(--color-warning-subtle); color: var(--color-warning); }
.ew-period-count { font-size: 11.5px; }

/* 叙事 */
.narrative { background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-left: 3px solid var(--color-accent); border-radius: 0 8px 8px 0; padding: 1.1rem 1.3rem; box-shadow: var(--shadow-print); }
.seg { position: relative; padding: 0.35rem 0.5rem 0.35rem 0; margin: 0 -0.5rem; border-radius: 6px; transition: background 0.12s; }
.seg:hover { background: var(--color-bg-hover); }
.seg .seg-h { font-weight: 600; font-size: 13.5px; margin-bottom: 0.2rem; color: var(--color-text-primary); }
.seg .seg-b { font-size: 13px; color: var(--color-text-secondary); line-height: 1.65; white-space: pre-wrap; }
.seg .seg-b.muted { color: var(--color-text-muted); }
.seg .seg-edit { position: absolute; right: 6px; top: 6px; opacity: 0; transition: opacity 0.12s; cursor: pointer; padding: 3px 6px; border-radius: 5px; color: var(--color-text-muted); }
.seg:hover .seg-edit { opacity: 1; }
.seg .seg-edit:hover { background: var(--color-accent-subtle); color: var(--color-accent); }
.seg-textarea { width: 100%; min-height: 90px; resize: vertical; font-family: inherit; font-size: 13px; line-height: 1.6; background: var(--color-input-bg); border: 1px solid var(--color-input-border); border-radius: 6px; color: var(--color-text-primary); padding: 0.5rem 0.6rem; outline: none; box-sizing: border-box; }
.seg-textarea:focus { border-color: var(--color-input-focus); }
.seg-actions { display: flex; gap: 0.4rem; margin-top: 0.4rem; }
.seg-h-row { display: flex; align-items: center; gap: 0.5rem; margin-bottom: 0.2rem; }
.seg-cited { font-size: 10px; color: var(--color-accent); background: var(--color-accent-subtle); padding: 1px 7px; border-radius: 999px; font-weight: 600; white-space: nowrap; }
.seg-cited.empty { color: var(--color-text-muted); background: var(--color-bg-sunken); }
.narrative-foot { display: flex; align-items: center; gap: 0.6rem; margin-top: 0.9rem; padding-top: 0.7rem; border-top: 1px solid var(--color-dialog-divider); font-size: 11.5px; color: var(--color-text-muted); }

/* 演进报告 meta */
.outlook-meta { margin-top: 0.8rem; font-size: 11.5px; }
.link-btn { display: inline-flex; align-items: center; gap: 3px; padding: 2px 7px; border: none; background: none; color: var(--color-text-muted); font-size: 11.5px; cursor: pointer; border-radius: 5px; font-family: inherit; }
.link-btn:hover { color: var(--color-accent); background: var(--color-accent-subtle); }

/* 金融可选模块 · DebateSection 折叠 */
.debate-fold { background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-radius: 12px; padding: 0.7rem 0.95rem; }
.debate-fold > summary { cursor: pointer; font-size: 13px; font-weight: 600; color: var(--color-text-secondary); list-style: none; display: flex; align-items: center; gap: 0.5rem; flex-wrap: wrap; }
.debate-fold > summary::-webkit-details-marker { display: none; }
.debate-fold > summary::before { content: '▸ '; color: var(--color-text-muted); }
.debate-fold[open] > summary::before { content: '▾ '; }
.df-tag { font-size: 10px; font-weight: 700; letter-spacing: 0.06em; text-transform: uppercase; color: var(--color-warning); background: var(--color-warning-subtle); padding: 2px 8px; border-radius: 4px; }
.df-title { color: var(--color-text-primary); }
.df-note-inline { font-size: 11px; color: var(--color-text-muted); font-weight: 400; }
.df-body { padding: 0.7rem 0 0; }
.df-hint { font-size: 12px; color: var(--color-text-muted); margin: 0 0 0.8rem; padding: 0.5rem 0.7rem; background: var(--color-bg-sunken); border-radius: 6px; line-height: 1.6; }
.df-hint b { color: var(--color-text-secondary); }

/* 数据源折叠高级 */
details.adv { background: var(--color-bg-elevated); border: 1px solid var(--color-border-subtle); border-radius: 10px; padding: 0.6rem 0.9rem; }
details.adv > summary { cursor: pointer; font-size: 13px; font-weight: 600; color: var(--color-text-secondary); list-style: none; }
details.adv > summary::-webkit-details-marker { display: none; }
details.adv > summary::before { content: "▸ "; color: var(--color-text-muted); }
details.adv[open] > summary::before { content: "▾ "; }
.adv-body { padding: 0.6rem 0 0; font-size: 12.5px; color: var(--color-text-secondary); }
.adv-row { display: flex; gap: 0.5rem; flex-wrap: wrap; align-items: center; margin-bottom: 0.5rem; }
.src-chip { display: inline-flex; align-items: center; gap: 6px; padding: 4px 10px; border-radius: 8px; background: var(--color-bg-base); border: 1px solid var(--color-border-medium); font-size: 12px; cursor: pointer; }
.src-chip .ok { color: var(--color-success); }
.kv { font-size: 12px; color: var(--color-text-secondary); }
.kv b { color: var(--color-text-primary); }

/* dialogs */
.ew-dialog-form { display: flex; flex-direction: column; gap: 0.75rem; }
.ew-field { display: flex; flex-direction: column; gap: 0.3rem; }
.ew-field--row { flex-direction: row; align-items: center; gap: 0.5rem; }
.ew-textarea { width: 100%; box-sizing: border-box; border: 1px solid var(--color-input-border); border-radius: 8px; background: var(--color-input-bg); color: var(--color-text-primary); font-size: 0.78rem; padding: 0.5rem 0.7rem; outline: none; resize: vertical; font-family: inherit; line-height: 1.5; }
.ew-textarea:focus { border-color: var(--color-input-focus); }
.ew-textarea--mono { font-family: ui-monospace, "Cascadia Code", Menlo, monospace; font-size: 0.72rem; }

.muted { color: var(--color-text-muted); }
.serif { font-family: Georgia, "Songti SC", "SimSun", "Source Serif 4", serif; }
</style>
