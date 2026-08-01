<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import AppButton from '~/components/ui/AppButton.vue'
import AppDialog from '~/components/ui/AppDialog.vue'
import AppInput from '~/components/ui/AppInput.vue'
import { useTopicWatchesApi, type TopicWatch, type TopicWatchHit, type TopicWatchStatus } from '~/api/topicWatches'
import type { DailyReportSection } from '~/api/dailyReports'
import {
  WATCH_COLLAPSE_THRESHOLD,
  buildSectionTitleLookup,
  formatMoreLabel,
  groupHitsByWatch,
  partitionByStatus,
  type WatchHitGroup,
} from './topicWatchGrouping'

/**
 * 日报顶部「关注标记」独立栏。
 * - 自包含：按 boardId 拉关注列表、按 reportId 拉当期命中。
 * - 命中按关注分组展示 section 标题 + AI 一句话理由；无命中显空态。
 * - 与正文 persistent_topic 分区语义区分：eyebrow「你在追踪 · Watchlist」+ accent 竖条。
 * - 关注命中是只读叠加标记，不侵入正文、不改 section 归属。
 * - 管理（新建/暂停/恢复/删除）内联；删除走 AppDialog 二次确认，零 window.*。
 */
const props = defineProps<{
  boardId: number
  reportId: number
  /** 当期日报的 sections，仅取 id + cluster_label 用于命中标题回填。 */
  sections: Array<Pick<DailyReportSection, 'id' | 'cluster_label'>>
}>()

const api = useTopicWatchesApi()

const watches = ref<TopicWatch[]>([])
const hits = ref<TopicWatchHit[]>([])
const loading = ref(true)
const errorMsg = ref<string | null>(null)

// 分组展开态：哪些 group 的命中列表整体可见。默认首组展开（其余折叠），贴合 mockup。
const openGroupIds = ref<Set<string>>(new Set())
// 组内「还有 N 条」展开态：哪些 group 展示超过阈值的全部命中。
const fullyExpandedIds = ref<Set<string>>(new Set())

// —— 管理态 ——
const createDialogOpen = ref(false)
const createLabel = ref('')
const creating = ref(false)
const deleteTarget = ref<TopicWatch | null>(null)
const deleting = ref(false)
const busyId = ref<string | null>(null) // 暂停/恢复进行中的 watch id，防重复点击

const sectionTitleById = computed(() => buildSectionTitleLookup(props.sections))
const partitioned = computed(() => partitionByStatus(watches.value))
const activeWatches = computed(() => partitioned.value.active)
const pausedWatches = computed(() => partitioned.value.paused)

const groups = computed<WatchHitGroup[]>(() =>
  groupHitsByWatch(hits.value, activeWatches.value, sectionTitleById.value),
)
const hasHits = computed(() => groups.value.length > 0)

function visibleItems(g: WatchHitGroup) {
  if (fullyExpandedIds.value.has(g.watch.id)) return g.items
  return g.items.slice(0, WATCH_COLLAPSE_THRESHOLD)
}
function hiddenCount(g: WatchHitGroup): number {
  return Math.max(0, g.items.length - WATCH_COLLAPSE_THRESHOLD)
}
function hasMore(g: WatchHitGroup): boolean {
  return !fullyExpandedIds.value.has(g.watch.id) && g.items.length > WATCH_COLLAPSE_THRESHOLD
}
function moreLabel(g: WatchHitGroup): string {
  return formatMoreLabel(hiddenCount(g))
}

function toggleGroup(id: string) {
  const next = new Set(openGroupIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  openGroupIds.value = next
}
function toggleExpand(id: string) {
  const next = new Set(fullyExpandedIds.value)
  if (next.has(id)) next.delete(id)
  else next.add(id)
  fullyExpandedIds.value = next
}

async function refresh() {
  loading.value = true
  errorMsg.value = null
  const [wRes, hRes] = await Promise.all([
    api.listWatches(props.boardId),
    api.getWatchHits(props.reportId),
  ])
  watches.value = wRes.success && wRes.data ? wRes.data : []
  hits.value = hRes.success && hRes.data ? hRes.data : []
  if (!wRes.success) errorMsg.value = wRes.error ?? '加载关注失败'
  // 默认展开首个有命中的分组
  const firstHitWatchId = hits.value.length
    ? watches.value.find(w => hits.value.some(h => h.watchId === w.id))?.id
    : undefined
  openGroupIds.value = firstHitWatchId ? new Set([firstHitWatchId]) : new Set()
  fullyExpandedIds.value = new Set()
  loading.value = false
}

function openCreate() {
  createLabel.value = ''
  createDialogOpen.value = true
}
async function submitCreate() {
  const label = createLabel.value.trim()
  if (!label || creating.value) return
  creating.value = true
  const res = await api.createWatch(props.boardId, label)
  creating.value = false
  if (res.success) {
    createDialogOpen.value = false
    createLabel.value = ''
    await refresh()
  }
  else {
    errorMsg.value = res.error ?? '创建失败'
  }
}

async function togglePause(w: TopicWatch) {
  if (busyId.value === w.id) return
  const next: TopicWatchStatus = w.status === 'paused' ? 'active' : 'paused'
  busyId.value = w.id
  const res = await api.updateWatch(w.id, { status: next })
  if (busyId.value === w.id) busyId.value = null
  if (res.success) await refresh()
  else errorMsg.value = res.error ?? '更新失败'
}

function requestDelete(w: TopicWatch) {
  deleteTarget.value = w
}
function cancelDelete() {
  deleteTarget.value = null
}
async function confirmDelete() {
  const target = deleteTarget.value
  if (!target || deleting.value) return
  deleting.value = true
  const res = await api.deleteWatch(target.id)
  deleting.value = false
  if (res.success) {
    deleteTarget.value = null
    await refresh()
  }
  else {
    errorMsg.value = res.error ?? '删除失败'
  }
}

onMounted(refresh)
watch(() => props.reportId, () => { void refresh() })
</script>

<template>
  <section
    v-if="!loading && watches.length"
    class="dwb-watch"
    data-testid="watch-bar"
    data-watchlist
    aria-label="关注标记命中"
  >
    <header class="dwb-watch__head">
      <span class="dwb-watch__eye">
        <Icon icon="mdi:eye-outline" width="14" aria-hidden="true" />
        你在追踪 · Watchlist
      </span>
      <span class="dwb-watch__sub">命中你主动标记的动态</span>
      <AppButton
        variant="ghost"
        size="sm"
        class="dwb-watch__add"
        data-testid="watch-create-open"
        @click="openCreate"
      >
        <Icon icon="mdi:plus" width="12" aria-hidden="true" />
        新建关注
      </AppButton>
    </header>

    <!-- 命中分组 -->
    <div v-if="hasHits" class="dwb-groups">
      <article
        v-for="g in groups"
        :key="g.watch.id"
        class="dwb-group"
        :class="{ 'is-open': openGroupIds.has(g.watch.id) }"
        data-testid="watch-group"
      >
        <span class="dwb-group__stripe" aria-hidden="true" />
        <div class="dwb-group__bar">
          <button
            type="button"
            class="dwb-group__toggle"
            data-testid="watch-group-toggle"
            :aria-expanded="openGroupIds.has(g.watch.id)"
            @click="toggleGroup(g.watch.id)"
          >
            <Icon
              icon="mdi:chevron-right"
              width="12"
              class="dwb-group__chev"
              aria-hidden="true"
            />
            <span class="dwb-group__label">{{ g.watch.label }}</span>
            <span class="dwb-group__count">命中 {{ g.items.length }}</span>
          </button>
          <span class="dwb-group__ops">
            <AppButton
              variant="ghost"
              size="sm"
              class="dwb-wop"
              :data-testid="`watch-toggle-status-${g.watch.id}`"
              :disabled="busyId === g.watch.id"
              :title="g.watch.status === 'paused' ? '恢复监控' : '暂停监控'"
              @click="togglePause(g.watch)"
            >
              <Icon
                :icon="g.watch.status === 'paused' ? 'mdi:play' : 'mdi:pause'"
                width="14"
                aria-hidden="true"
              />
            </AppButton>
            <AppButton
              variant="ghost"
              size="sm"
              class="dwb-wop"
              :data-testid="`watch-delete-${g.watch.id}`"
              title="删除关注"
              @click="requestDelete(g.watch)"
            >
              <Icon icon="mdi:trash-can-outline" width="14" aria-hidden="true" />
            </AppButton>
          </span>
        </div>
        <div v-if="openGroupIds.has(g.watch.id)" class="dwb-group__body">
          <p
            v-for="item in visibleItems(g)"
            :key="item.hit.id"
            class="dwb-hit"
            data-testid="watch-hit"
          >
            <span class="dwb-hit__title">{{ item.sectionTitle || '（已归档动态）' }}</span>
            <span v-if="item.hit.reason" class="dwb-hit__reason">↳ {{ item.hit.reason }}</span>
          </p>
          <button
            v-if="hasMore(g)"
            type="button"
            class="dwb-hit__more"
            data-testid="watch-hit-more"
            @click="toggleExpand(g.watch.id)"
          >
            {{ moreLabel(g) }}
          </button>
        </div>
      </article>
    </div>

    <!-- 空态：无任何 active 关注命中 -->
    <div v-else class="dwb-empty" data-testid="watch-empty">
      <p>今天没有命中你关注的动态。</p>
      <p v-if="activeWatches.length">
        <b>{{ activeWatches.length }} 个关注仍在监控中</b>，命中后这里会第一时间提示。
      </p>
    </div>

    <!-- 已暂停关注（管理面：灰显 + 恢复/删除） -->
    <div v-if="pausedWatches.length" class="dwb-paused">
      <div class="dwb-paused__head">已暂停的关注</div>
      <div
        v-for="w in pausedWatches"
        :key="w.id"
        class="dwb-paused-row"
        data-testid="watch-paused"
      >
        <span class="dwb-paused-row__label">{{ w.label }}</span>
        <span class="dwb-paused-row__tag">已暂停</span>
        <span class="dwb-paused-row__ops">
          <AppButton
            variant="ghost"
            size="sm"
            :data-testid="`watch-toggle-status-${w.id}`"
            :disabled="busyId === w.id"
            @click="togglePause(w)"
          >
            <Icon icon="mdi:play" width="14" aria-hidden="true" />
            恢复
          </AppButton>
          <AppButton
            variant="ghost"
            size="sm"
            class="dwb-wop"
            :data-testid="`watch-delete-${w.id}`"
            title="删除关注"
            @click="requestDelete(w)"
          >
            <Icon icon="mdi:trash-can-outline" width="14" aria-hidden="true" />
          </AppButton>
        </span>
      </div>
    </div>

    <p v-if="errorMsg" class="dwb-error" role="alert">{{ errorMsg }}</p>

    <!-- 新建关注对话框 -->
    <AppDialog v-model="createDialogOpen" title="新建关注" width="420px" :close-on-overlay="false">
      <p class="dwb-dialog__hint">用一句话描述你想盯的动态，系统会在每期日报里替你判定命中。</p>
      <AppInput
        v-model="createLabel"
        placeholder="例如：美伊会不会真打起来"
        data-testid="watch-create-input"
        @keydown.enter="submitCreate"
      />
      <template #footer>
        <AppButton variant="ghost" size="sm" data-testid="watch-create-cancel" @click="createDialogOpen = false">
          取消
        </AppButton>
        <AppButton
          variant="primary"
          size="sm"
          :loading="creating"
          :disabled="!createLabel.trim()"
          data-testid="watch-create-submit"
          @click="submitCreate"
        >
          创建
        </AppButton>
      </template>
    </AppDialog>

    <!-- 删除确认对话框（替代原生 confirm，零原生弹窗） -->
    <AppDialog
      :model-value="deleteTarget !== null"
      title="删除关注"
      width="420px"
      :close-on-overlay="true"
      @update:model-value="(v: boolean) => { if (!v) cancelDelete() }"
    >
      <p v-if="deleteTarget" class="dwb-dialog__confirm">
        确定删除关注「<b>{{ deleteTarget.label }}</b>」？其全部命中记录将一并清理，此操作不可撤销。
      </p>
      <template #footer>
        <AppButton variant="ghost" size="sm" data-testid="watch-delete-cancel" @click="cancelDelete">
          取消
        </AppButton>
        <AppButton
          variant="danger"
          size="sm"
          :loading="deleting"
          data-testid="watch-delete-confirm"
          @click="confirmDelete"
        >
          删除
        </AppButton>
      </template>
    </AppDialog>
  </section>
</template>

<style scoped>
/* 关注标记栏 —— 与正文 persistent_topic 分区语义区分：
   独立 eyebrow「你在追踪 · Watchlist」+ accent 竖条；颜色全语义 token。 */
.dwb-watch {
  max-width: 82rem;
  margin: 0 auto;
  padding: 1.5rem clamp(1rem, 4vw, 4rem) 0;
  font-family: "Noto Serif SC", serif;
  animation: dwbFade 0.5s cubic-bezier(0.2, 0.7, 0.3, 1) both;
}

.dwb-watch__head {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding-bottom: 0.6rem;
  flex-wrap: wrap;
}

.dwb-watch__eye {
  display: inline-flex;
  align-items: center;
  gap: 0.35rem;
  color: var(--color-accent);
  font-size: 0.72rem;
  font-style: italic;
  letter-spacing: 0.16em;
  text-transform: uppercase;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}

.dwb-watch__sub {
  color: var(--color-text-secondary);
  font-size: 0.82rem;
  font-style: italic;
}

.dwb-watch__add {
  margin-left: auto;
}

.dwb-groups {
  display: grid;
  gap: 0.6rem;
}

.dwb-group {
  position: relative;
  border: 1px solid var(--color-border-medium);
  /* accent 竖条：与正文话题分区视觉区分的硬标识 */
  border-left: 3px solid var(--color-accent);
  background: var(--color-bg-elevated);
  border-radius: 0 6px 6px 0;
  overflow: hidden;
}

.dwb-group__stripe {
  display: none; /* border-left 即竖条；占位以便测试与未来改样 */
}

.dwb-group__bar {
  display: flex;
  align-items: center;
  gap: 0.4rem;
  padding: 0.55rem 0.6rem 0.55rem 0.85rem;
}

.dwb-group__toggle {
  display: inline-flex;
  align-items: center;
  gap: 0.45rem;
  flex: 1 1 auto;
  min-width: 0;
  border: 0;
  background: transparent;
  color: var(--color-text-primary);
  font-family: inherit;
  font-size: 0.95rem;
  font-weight: 700;
  cursor: pointer;
  padding: 0;
  text-align: left;
}

.dwb-group__toggle:focus-visible {
  outline: 2px solid var(--color-input-focus);
  outline-offset: 2px;
  border-radius: 4px;
}

.dwb-group__chev {
  color: var(--color-text-muted);
  flex: none;
  transition: transform 0.2s;
}

.dwb-group.is-open .dwb-group__chev {
  transform: rotate(90deg);
}

.dwb-group__label {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
  line-height: 1.35;
}

.dwb-group__count {
  flex: none;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.66rem;
  color: var(--color-text-muted);
  background: var(--color-accent-subtle);
  padding: 0.1rem 0.45rem;
  border-radius: 999px;
}

.dwb-group__ops {
  display: inline-flex;
  gap: 0.15rem;
  flex: none;
}

.dwb-wop {
  padding: 2px 6px !important;
}

.dwb-group__body {
  padding: 0.1rem 0.85rem 0.65rem;
  border-top: 1px solid var(--color-border-subtle);
}

.dwb-hit {
  display: block;
  margin: 0;
  padding: 0.5rem 0.2rem;
  border-top: 1px dashed var(--color-border-subtle);
}

.dwb-hit:first-child {
  border-top: 0;
}

.dwb-hit__title {
  font-size: 0.9rem;
  font-weight: 600;
  color: var(--color-text-primary);
  line-height: 1.5;
}

.dwb-hit__reason {
  display: block;
  margin-top: 0.2rem;
  font-size: 0.78rem;
  font-style: italic;
  color: var(--color-text-secondary);
  line-height: 1.6;
}

.dwb-hit__more {
  display: block;
  width: 100%;
  margin-top: 0.4rem;
  padding: 0.3rem;
  border: 0;
  background: transparent;
  color: var(--color-text-muted);
  font-size: 0.74rem;
  font-style: italic;
  font-family: inherit;
  cursor: pointer;
  text-align: center;
}

.dwb-hit__more:hover {
  color: var(--color-accent);
}

/* 空态 */
.dwb-empty {
  margin-top: 0.6rem;
  padding: 1.1rem;
  text-align: center;
  border: 1px dashed var(--color-border-medium);
  border-radius: 6px;
  color: var(--color-text-muted);
  font-style: italic;
}

.dwb-empty p {
  margin: 0.15rem 0;
}

.dwb-empty b {
  color: var(--color-text-secondary);
  font-style: normal;
}

/* 已暂停关注（灰显） */
.dwb-paused {
  margin-top: 0.9rem;
  padding-top: 0.6rem;
  border-top: 1px dashed var(--color-border-subtle);
}

.dwb-paused__head {
  color: var(--color-text-muted);
  font-size: 0.7rem;
  letter-spacing: 0.1em;
  text-transform: uppercase;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
  margin-bottom: 0.4rem;
}

.dwb-paused-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.35rem 0.2rem;
  opacity: 0.6;
  color: var(--color-text-secondary);
}

.dwb-paused-row__label {
  flex: 1 1 auto;
  font-size: 0.86rem;
  text-decoration: line-through;
  text-decoration-color: var(--color-text-muted);
}

.dwb-paused-row__tag {
  flex: none;
  font-size: 0.62rem;
  letter-spacing: 0.08em;
  color: var(--color-text-muted);
  border: 1px solid var(--color-border-medium);
  border-radius: 4px;
  padding: 0.05rem 0.35rem;
}

.dwb-paused-row__ops {
  display: inline-flex;
  gap: 0.15rem;
  flex: none;
}

.dwb-error {
  margin-top: 0.6rem;
  color: var(--color-error);
  font-size: 0.78rem;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}

.dwb-dialog__hint {
  margin: 0 0 0.6rem;
  color: var(--color-text-secondary);
  font-size: 0.82rem;
  line-height: 1.6;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}

.dwb-dialog__confirm {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 0.88rem;
  line-height: 1.7;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}

@keyframes dwbFade {
  from { opacity: 0; transform: translateY(6px); }
  to { opacity: 1; transform: translateY(0); }
}

@media (prefers-reduced-motion: reduce) {
  .dwb-watch {
    animation: none;
  }
}

@media (max-width: 720px) {
  .dwb-watch__sub {
    display: none;
  }
}
</style>
