<script setup lang="ts">
/**
 * 话题态势版图容器（design §5 + §6）。
 *
 * 职责：
 *  - 拉取 GET /semantic-boards/:id/topic-landscape（identity 轨，只读）。
 *  - loading / error / 空态分发：
 *    · topics=[] && trend=[] → 板块无日报 → 空态卡「[生成日报]」（task 2.5）。
 *    · topics=[] && trend.length>0 → 有日报但暂无锚定话题 → 提示。
 *    · 否则 → VitalityBar + StanceCardWall。
 *  - 生成日报：复用既有 generateDailyReport + useDailyReportProgress（WS 进度），完成后重载。
 *
 * 卡片 click → 向上 emit selectTopic，由 TagsPage 切「话题总览」tab 并聚焦该 topic。
 */
import { ref, watch, computed } from 'vue'
import { Icon } from '@iconify/vue'
import { useSemanticBoardsApi, type TopicLandscapeResponse } from '~/api/semanticBoards'
import { useDailyReportsApi } from '~/api/dailyReports'
import { useDailyReportProgress } from '~/composables/useDailyReportProgress'
import { useNotify } from '~/composables/useNotify'
import VitalityBar from './VitalityBar.vue'
import TopicRhythmChart from './TopicRhythmChart.vue'
import StanceCardWall from './StanceCardWall.vue'

const props = defineProps<{
  boardId: number
}>()

const emit = defineEmits<{
  selectTopic: [topicId: number]
  generated: []
}>()

const api = useSemanticBoardsApi()
const reportsApi = useDailyReportsApi()
const { success: notifySuccess, error: notifyError } = useNotify()
const { progress, done, totalSaved, reset } = useDailyReportProgress()

const DEFAULT_DAYS = 30

const data = ref<TopicLandscapeResponse | null>(null)
const loading = ref(false)
const error = ref<string | null>(null)

/** 空态判定：板块无日报（design §6 + spec「空态处理」）。 */
const isNoReports = computed(
  () => (data.value?.topics.length ?? 0) === 0 && (data.value?.vitality.trend.length ?? 0) === 0,
)
/** 有日报但无锚定话题（与「无日报」区分开）。 */
const isNoTopics = computed(
  () => (data.value?.topics.length ?? 0) === 0 && (data.value?.vitality.trend.length ?? 0) > 0,
)

async function load(boardId: number) {
  loading.value = true
  error.value = null
  const res = await api.getTopicLandscape(boardId, DEFAULT_DAYS)
  if (res.success && res.data) {
    data.value = res.data
  } else {
    data.value = null
    error.value = res.error || '加载话题态势失败'
  }
  loading.value = false
}

void load(props.boardId)
watch(() => props.boardId, (id) => { void load(id) })

// ── 空态：生成日报（复用既有端点 + WS 进度） ───────────────────────────────
const generating = ref(false)
const progressEntries = computed(() => Array.from(progress.value.values()))

async function handleGenerate() {
  if (generating.value) return
  generating.value = true
  reset()
  const today = new Date().toISOString().slice(0, 10)
  const res = await reportsApi.generateDailyReport({ date: today, board_id: props.boardId })
  if (!res.success || !res.data) {
    notifyError(res.error || '触发生成失败')
    generating.value = false
    return
  }
  // job 已触发，WS 进度由 useDailyReportProgress 持续写入；done 变 true 后重载。
}

// done 由 WS daily_report_done 置位 → 重载态势 + 通知
watch(done, (isDone) => {
  if (!isDone) return
  generating.value = false
  notifySuccess(`日报已生成（共 ${totalSaved.value} 篇）`)
  emit('generated')
  void load(props.boardId)
})

function onSelectTopic(id: number) {
  emit('selectTopic', id)
}
</script>

<template>
  <section class="tlp">
    <header class="tlp-head">
      <Icon icon="mdi:chart-timeline-variant" width="15" class="tlp-head-icon" />
      <span class="tlp-title">话题态势版图</span>
      <span class="tlp-sub">基于持久话题 identity 轨 · 只读</span>
      <div class="tlp-spacer" />
      <button type="button" class="tlp-refresh" title="刷新" @click="load(boardId)">
        <Icon icon="mdi:refresh" width="13" />
      </button>
    </header>

    <!-- loading -->
    <div v-if="loading" class="tlp-loading">
      <div v-for="i in 4" :key="i" class="tlp-skeleton" />
    </div>

    <!-- error -->
    <div v-else-if="error" class="tlp-error">
      <Icon icon="mdi:alert-circle-outline" width="16" />
      <span>{{ error }}</span>
      <button type="button" class="tlp-retry" @click="load(boardId)">重试</button>
    </div>

    <!-- 空态：板块无日报（task 2.5） -->
    <div v-else-if="isNoReports" class="tlp-empty">
      <Icon icon="mdi:file-document-outline" width="22" class="tlp-empty-icon" />
      <p class="tlp-empty-text">该板块还没有日报，话题态势需要日报数据。</p>
      <button
        type="button"
        class="tlp-empty-btn"
        :disabled="generating"
        @click="handleGenerate"
      >
        <Icon icon="mdi:play" width="13" />
        {{ generating ? '生成中…' : '生成日报' }}
      </button>
      <!-- WS 进度（复用 useDailyReportProgress） -->
      <div v-if="generating && progressEntries.length" class="tlp-progress">
        <div
          v-for="entry in progressEntries"
          :key="entry.board_id"
          class="tlp-progress-row"
        >
          <span class="tlp-progress-board">{{ entry.board_name }}</span>
          <span
            class="tlp-progress-status"
            :class="{
              'tlp-progress-status--generating': entry.status === 'generating',
              'tlp-progress-status--completed': entry.status === 'completed',
              'tlp-progress-status--failed': entry.status === 'failed',
            }"
          >
            <template v-if="entry.status === 'waiting'">等待中</template>
            <template v-else-if="entry.status === 'generating'">生成中 {{ entry.progress }}</template>
            <template v-else-if="entry.status === 'completed'">完成 ({{ entry.saved }})</template>
            <template v-else-if="entry.status === 'failed'">失败</template>
            <template v-else>{{ entry.status }}</template>
          </span>
        </div>
      </div>
    </div>

    <!-- 空态：有日报但无锚定话题 -->
    <div v-else-if="isNoTopics" class="tlp-empty">
      <Icon icon="mdi:tag-off-outline" width="22" class="tlp-empty-icon" />
      <p class="tlp-empty-text">该板块已有日报，但尚未锚定持久话题。</p>
      <p class="tlp-empty-hint">在「日报」tab 孵化话题后，此处将出现话题态势。</p>
    </div>

    <!-- 正常态 -->
    <template v-else-if="data">
      <VitalityBar :vitality="data.vitality" />
      <TopicRhythmChart :topics="data.topics" @select-topic="onSelectTopic" />
      <StanceCardWall :topics="data.topics" @select-topic="onSelectTopic" />
    </template>
  </section>
</template>

<style scoped>
.tlp {
  display: flex;
  flex-direction: column;
  gap: 0.75rem;
}

.tlp-head {
  display: flex;
  align-items: center;
  gap: 0.4rem;
}

.tlp-head-icon {
  color: var(--color-text-muted);
}

.tlp-title {
  font-size: 0.78rem;
  font-weight: 600;
  color: var(--color-text-secondary);
}

.tlp-sub {
  font-size: 0.65rem;
  color: var(--color-text-muted);
}

.tlp-spacer {
  flex: 1;
}

.tlp-refresh {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  width: 22px;
  height: 22px;
  border-radius: 6px;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-sunken);
  color: var(--color-text-muted);
  cursor: pointer;
  transition: all 0.12s ease;
}

.tlp-refresh:hover {
  color: var(--color-text-secondary);
  border-color: var(--color-border-strong);
  background: var(--color-bg-hover);
}

.tlp-loading {
  display: grid;
  grid-template-columns: repeat(auto-fill, minmax(180px, 1fr));
  gap: 0.5rem;
}

.tlp-skeleton {
  height: 96px;
  border-radius: 10px;
  background: var(--color-bg-hover);
  animation: tlpPulse 1.5s ease-in-out infinite;
}

@keyframes tlpPulse {
  0%, 100% { opacity: 0.4; }
  50% { opacity: 0.8; }
}

.tlp-error {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.75rem;
  border-radius: 10px;
  border: 1px solid var(--color-border-subtle);
  background: var(--color-bg-elevated);
  color: var(--color-text-muted);
  font-size: 0.72rem;
}

.tlp-retry {
  margin-left: auto;
  font-size: 0.68rem;
  padding: 0.15rem 0.5rem;
  border-radius: 6px;
  border: 1px solid var(--color-border-medium);
  background: var(--color-bg-sunken);
  color: var(--color-text-secondary);
  cursor: pointer;
}

.tlp-retry:hover {
  background: var(--color-bg-hover);
}

.tlp-empty {
  display: flex;
  flex-direction: column;
  align-items: center;
  gap: 0.5rem;
  padding: 1.4rem 1rem;
  border-radius: 10px;
  border: 1px dashed var(--color-border-medium);
  background: var(--color-bg-elevated);
  text-align: center;
}

.tlp-empty-icon {
  color: var(--color-text-muted);
  opacity: 0.5;
}

.tlp-empty-text {
  margin: 0;
  font-size: 0.78rem;
  color: var(--color-text-secondary);
}

.tlp-empty-hint {
  margin: 0;
  font-size: 0.68rem;
  color: var(--color-text-muted);
}

.tlp-empty-btn {
  display: inline-flex;
  align-items: center;
  gap: 0.3rem;
  padding: 0.35rem 0.85rem;
  border-radius: 8px;
  border: 1px solid var(--color-accent);
  background: var(--color-accent-subtle);
  color: var(--color-accent);
  font-size: 0.74rem;
  font-weight: 600;
  cursor: pointer;
  transition: all 0.12s ease;
}

.tlp-empty-btn:hover:not(:disabled) {
  background: var(--color-accent);
  color: #fff;
}

.tlp-empty-btn:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}

/* WS 进度（mirror DailyReportGenerateDialog 风格） */
.tlp-progress {
  display: flex;
  flex-direction: column;
  gap: 0.2rem;
  width: 100%;
  max-width: 320px;
  margin-top: 0.35rem;
}

.tlp-progress-row {
  display: flex;
  align-items: center;
  justify-content: space-between;
  padding: 0.15rem 0;
}

.tlp-progress-board {
  font-size: 0.68rem;
  color: var(--color-text-secondary);
}

.tlp-progress-status {
  font-size: 0.62rem;
  color: var(--color-text-muted);
}

.tlp-progress-status--generating { color: #facc15; }
.tlp-progress-status--completed { color: #4ade80; }
.tlp-progress-status--failed { color: #f87171; }
</style>
