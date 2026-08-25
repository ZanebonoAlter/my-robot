<script setup lang="ts">
import { computed, onMounted, ref, watch } from 'vue'
import { Icon } from '@iconify/vue'
import AppButton from '~/components/ui/AppButton.vue'
import AppDialog from '~/components/ui/AppDialog.vue'
import { useTopicWatchesApi, type TopicWatch, type TopicWatchStatus } from '~/api/topicWatches'
import TopicWatchCreateDialog from './TopicWatchCreateDialog.vue'

/**
 * 版块级关注管理面板（创建/管理唯一入口，design.md §4.5）。
 *
 * - 挂载于 TagsPage（tab 栏右端「我在追踪 (N)」chip 打开），v-model 控制开关。
 * - 列出该版块全部关注（四类型徽标——存量 label/keyword 提示轨继续展示可管理，
 *   物化轨 keyword_topic/sentence_topic 为主力；创建入口仅物化轨双选）。
 * - 暂停/恢复（PATCH）、删除（AppDialog 二次确认，零 window.*；sentence_topic
 *   删除确认明示将归档专属话题）。
 * - 「新建关注」开 TopicWatchCreateDialog（旧提示轨创建入口退役隐藏）。
 * - 计数变更通过 changed 事件通知宿主（TagsPage 刷新 chip N = active+paused）。
 */
const props = defineProps<{
  modelValue: boolean
  boardId: number
}>()

const emit = defineEmits<{
  'update:modelValue': [value: boolean]
  /** 列表发生增删/状态变化（宿主据此刷新入口计数）。 */
  changed: []
}>()

const api = useTopicWatchesApi()

const watches = ref<TopicWatch[]>([])
const loading = ref(false)
const errorMsg = ref<string | null>(null)

// —— 删除二次确认 ——
const deleteTarget = ref<TopicWatch | null>(null)
const deleting = ref(false)
// —— 暂停/恢复进行中 ——
const busyId = ref<string | null>(null)

const total = computed(() => watches.value.length)

async function refresh() {
  loading.value = true
  errorMsg.value = null
  const res = await api.listWatches(props.boardId)
  watches.value = res.success && res.data ? res.data : []
  if (!res.success) errorMsg.value = res.error ?? '加载关注失败'
  loading.value = false
  emit('changed')
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
  // sentence_topic：确认框已明示将归档专属话题，携带 confirm 让后端执行联动归档。
  const res = await api.deleteWatch(target.id, target.type === 'sentence_topic')
  deleting.value = false
  if (res.success) {
    deleteTarget.value = null
    await refresh()
  }
  else {
    errorMsg.value = res.error ?? '删除失败'
    deleteTarget.value = null // 归档失败等错误留在主面板 errorMsg，不再卡确认框
  }
}

// —— 新建 ——
const createOpen = ref(false)
function openCreate() {
  createOpen.value = true
}
function onCreated() {
  void refresh()
}

function close() {
  emit('update:modelValue', false)
}

// —— 四类型徽标（watch-materialized-topic） ——
const TYPE_META: Record<string, { name: string, icon: string, cls: string, title: string }> = {
  label: { name: '话题', icon: 'mdi:eye-outline', cls: 'wmp-row__type--label', title: '话题语义匹配（提示）' },
  keyword: { name: '关键字', icon: 'mdi:pound', cls: 'wmp-row__type--kw', title: '关键字文本匹配（提示）' },
  keyword_topic: { name: '关键字话题', icon: 'mdi:text-box-search', cls: 'wmp-row__type--kwt', title: '关键字物化话题' },
  sentence_topic: { name: '一句话话题', icon: 'mdi:cube-scan', cls: 'wmp-row__type--st', title: '一句话物化话题（持久）' },
}
function typeBadgeMeta(t: string) {
  return TYPE_META[t] ?? TYPE_META.label!
}
function typeBadgeClass(t: string) { return typeBadgeMeta(t).cls }
function typeBadgeIcon(t: string) { return typeBadgeMeta(t).icon }
function typeBadgeName(t: string) { return typeBadgeMeta(t).name }
function typeBadgeTitle(t: string) { return typeBadgeMeta(t).title }

watch(() => props.modelValue, (open) => {
  if (open) void refresh()
})
watch(() => props.boardId, () => {
  if (props.modelValue) void refresh()
})
onMounted(() => {
  if (props.modelValue) void refresh()
})
</script>

<template>
  <AppDialog
    :model-value="modelValue"
    title="我在追踪"
    width="520px"
    :close-on-overlay="true"
    @update:model-value="close"
  >
    <div class="wmp" data-testid="watch-manage-panel">
      <p class="wmp__sub">本版块 · {{ total }} 个关注（active + paused）</p>

      <!-- 列表 -->
      <div v-if="loading" class="wmp__hint">加载中…</div>
      <div v-else-if="watches.length" class="wmp-list">
        <div
          v-for="w in watches"
          :key="w.id"
          class="wmp-row"
          :class="{ 'is-paused': w.status === 'paused' }"
          data-testid="watch-row"
        >
          <span class="wmp-row__type" :class="typeBadgeClass(w.type)" :title="typeBadgeTitle(w.type)">
            <Icon :icon="typeBadgeIcon(w.type)" width="11" aria-hidden="true" />
            {{ typeBadgeName(w.type) }}
          </span>
          <span
            class="wmp-row__label"
            :class="{ 'wmp-row__label--kw': w.type === 'keyword' }"
            :title="w.label"
          >{{ w.label }}</span>
          <span v-if="w.status === 'paused'" class="wmp-row__paused">已暂停</span>
          <span class="wmp-row__ops">
            <AppButton
              variant="ghost"
              size="sm"
              class="wmp-op"
              :data-testid="`watch-toggle-status-${w.id}`"
              :disabled="busyId === w.id"
              :title="w.status === 'paused' ? '恢复监控' : '暂停监控'"
              @click="togglePause(w)"
            >
              <Icon
                :icon="w.status === 'paused' ? 'mdi:play' : 'mdi:pause'"
                width="14"
                aria-hidden="true"
              />
            </AppButton>
            <AppButton
              variant="ghost"
              size="sm"
              class="wmp-op"
              :data-testid="`watch-delete-${w.id}`"
              title="删除关注"
              @click="requestDelete(w)"
            >
              <Icon icon="mdi:trash-can-outline" width="14" aria-hidden="true" />
            </AppButton>
          </span>
        </div>
      </div>
      <div v-else class="wmp-empty" data-testid="watch-empty">
        还没有关注。<br>
        <b>点下方「新建关注」，盯一个话题或一个词。</b>
      </div>

      <p v-if="errorMsg" class="wmp-error" role="alert">{{ errorMsg }}</p>
    </div>

    <template #footer>
      <AppButton
        variant="secondary"
        size="md"
        class="wmp-new"
        data-testid="watch-manage-create"
        @click="openCreate"
      >
        <Icon icon="mdi:plus" width="14" aria-hidden="true" />
        新建关注
      </AppButton>
    </template>

    <!-- 新建对话框（类型双选 + 解析预览） -->
    <TopicWatchCreateDialog
      v-model="createOpen"
      :board-id="boardId"
      @created="onCreated"
    />

    <!-- 删除二次确认（替代原生 confirm） -->
    <AppDialog
      :model-value="deleteTarget !== null"
      title="删除关注"
      width="420px"
      :close-on-overlay="true"
      @update:model-value="(v: boolean) => { if (!v) cancelDelete() }"
    >
      <p v-if="deleteTarget" class="wmp-confirm">
        <template v-if="deleteTarget.type === 'sentence_topic'">
          确定删除关注「<b>{{ deleteTarget.label }}</b>」？其专属持久话题将一并<b>归档</b>（历史日报中的话题板块保留），此操作不可撤销。
        </template>
        <template v-else-if="deleteTarget.type === 'keyword_topic'">
          确定删除关注「<b>{{ deleteTarget.label }}</b>」？后续日报不再生成该关键字板块，历史板块保留，此操作不可撤销。
        </template>
        <template v-else>
          确定删除关注「<b>{{ deleteTarget.label }}</b>」？其全部命中记录将一并清理，此操作不可撤销。
        </template>
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
  </AppDialog>
</template>

<style scoped>
.wmp {
  display: flex;
  flex-direction: column;
  gap: 0.6rem;
  font-family: system-ui, -apple-system, "PingFang SC", "Microsoft YaHei", sans-serif;
}

.wmp__sub {
  margin: 0;
  font-size: 0.72rem;
  color: var(--color-text-muted);
}

/* —— 回扫反馈 banner —— */

.wmp-scan svg {
  flex: none;
  margin-top: 0.1rem;
  color: var(--color-success);
}

.wmp-scan b {
  color: var(--color-text-primary);
}

.wmp-scan__link {
  border: 0;
  background: transparent;
  padding: 0;
  color: var(--color-success);
  text-decoration: underline;
  cursor: pointer;
  font: inherit;
}

.wmp__hint {
  font-size: 0.78rem;
  color: var(--color-text-muted);
}

/* —— 列表行 —— */
.wmp-list {
  display: flex;
  flex-direction: column;
}

.wmp-row {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.55rem 0.1rem;
  border-top: 1px dashed var(--color-border-subtle);
}

.wmp-row:first-child {
  border-top: 0;
}

.wmp-row.is-paused {
  opacity: 0.55;
}

.wmp-row__type {
  flex: none;
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.64rem;
  font-weight: 600;
  border-radius: 4px;
  padding: 0.1rem 0.4rem;
}

.wmp-row__type--label {
  color: var(--color-accent);
  background: var(--color-accent-subtle);
}

.wmp-row__type--kw {
  color: var(--color-tag-keyword);
  background: var(--color-tag-keyword-bg);
}
.wmp-row__type--kwt {
  color: var(--color-tag-keyword);
  background: var(--color-tag-keyword-bg);
}
.wmp-row__type--st {
  color: var(--color-tag-keyword);
  background: var(--color-tag-keyword-bg);
}

.wmp-row__label {
  flex: 1 1 auto;
  min-width: 0;
  font-family: "Noto Serif SC", serif;
  font-size: 0.84rem;
  color: var(--color-text-primary);
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.wmp-row__label--kw {
  font-family: ui-monospace, SFMono-Regular, Consolas, monospace;
  font-size: 0.76rem;
}

.wmp-row__paused {
  flex: none;
  font-size: 0.62rem;
  font-style: italic;
  color: var(--color-text-muted);
}

.wmp-row__ops {
  margin-left: auto;
  display: inline-flex;
  gap: 0.15rem;
  flex: none;
}

.wmp-op {
  padding: 2px 6px !important;
}

.wmp-empty {
  padding: 1.2rem;
  text-align: center;
  border: 1px dashed var(--color-border-medium);
  border-radius: 8px;
  color: var(--color-text-muted);
  font-style: italic;
  font-size: 0.8rem;
  line-height: 1.8;
}

.wmp-empty b {
  color: var(--color-text-secondary);
  font-style: normal;
}

.wmp-error {
  margin: 0;
  color: var(--color-error);
  font-size: 0.78rem;
}

.wmp-confirm {
  margin: 0;
  color: var(--color-text-primary);
  font-size: 0.88rem;
  line-height: 1.7;
}

.wmp-new {
  width: 100%;
}
</style>
