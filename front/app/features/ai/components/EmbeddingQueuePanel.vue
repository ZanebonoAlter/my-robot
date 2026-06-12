<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { useEmbeddingQueueApi, type EmbeddingQueueStatus, type EmbeddingQueueTask } from '~/api'

const loading = ref(false)
const error = ref<string | null>(null)
const status = ref<EmbeddingQueueStatus>({
  pending: 0,
  processing: 0,
  completed: 0,
  failed: 0,
  total: 0,
})
const tasks = ref<EmbeddingQueueTask[]>([])
const totalTasks = ref(0)
const statusFilter = ref('')
const currentPage = ref(1)
const pageSize = 20
const retrying = ref(false)

let refreshTimer: ReturnType<typeof setInterval> | null = null

const api = useEmbeddingQueueApi()

async function loadStatus() {
  try {
    const response = await api.getStatus()
    if (response.success && response.data) {
      status.value = response.data
    }
  } catch (err) {
    console.error('Failed to load queue status:', err)
  }
}

async function loadTasks() {
  loading.value = true
  error.value = null
  try {
    const response = await api.getTasks({
      status: statusFilter.value || undefined,
      limit: pageSize,
      offset: (currentPage.value - 1) * pageSize,
    })
    if (response.success && response.data) {
      tasks.value = response.data.tasks
      totalTasks.value = response.data.total
    } else {
      throw new Error(response.error || '加载任务列表失败')
    }
  } catch (err) {
    error.value = err instanceof Error ? err.message : '加载失败'
  } finally {
    loading.value = false
  }
}

async function retryFailed() {
  retrying.value = true
  try {
    const response = await api.retryFailed()
    if (response.success) {
      await Promise.all([loadStatus(), loadTasks()])
    }
  } catch (err) {
    console.error('Failed to retry:', err)
  } finally {
    retrying.value = false
  }
}

function getStatusStyle(s: string) {
  switch (s) {
    case 'pending': return { background: 'var(--color-warning-bg, rgba(196, 136, 60, 0.1))', color: 'var(--color-warning)' }
    case 'processing': return { background: 'var(--color-link-subtle)', color: 'var(--color-link)' }
    case 'completed': return { background: 'var(--color-success-bg, rgba(61, 138, 74, 0.1))', color: 'var(--color-success)' }
    case 'failed': return { background: 'var(--color-error-bg, rgba(196, 47, 60, 0.1))', color: 'var(--color-error)' }
    default: return { background: 'var(--color-bg-sunken)', color: 'var(--color-text-muted)' }
  }
}

function getStatusLabel(s: string) {
  switch (s) {
    case 'pending': return '待处理'
    case 'processing': return '处理中'
    case 'completed': return '已完成'
    case 'failed': return '失败'
    default: return s
  }
}

function formatDate(dateStr: string | null) {
  if (!dateStr) return '-'
  return new Date(dateStr).toLocaleString('zh-CN')
}

const progressPercent = computed(() => {
  if (status.value.total === 0) return 0
  return Math.round((status.value.completed / status.value.total) * 100)
})

const totalPages = computed(() => Math.ceil(totalTasks.value / pageSize))

function changePage(page: number) {
  currentPage.value = page
  loadTasks()
}

function changeFilter(value: string) {
  statusFilter.value = value
  currentPage.value = 1
  loadTasks()
}

async function refreshAll() {
  await Promise.all([loadStatus(), loadTasks()])
}

onMounted(async () => {
  await refreshAll()
  refreshTimer = setInterval(loadStatus, 5000)
})

onUnmounted(() => {
  if (refreshTimer) {
    clearInterval(refreshTimer)
  }
})
</script>

<template>
  <div class="space-y-4">
    <div class="flex items-center justify-between gap-4">
      <AppSectionHeader title="Embedding 队列" description="实时追踪embedding生成进度" icon-name="mdi:playlist-check" />
      <div class="flex items-center gap-2">
        <button
          class="px-3 py-1.5 text-sm transition-colors"
          style="color: var(--color-text-secondary)"
          @click="refreshAll"
        >
          <Icon icon="mdi:refresh" width="16" height="16" />
        </button>
        <button
          v-if="status.failed > 0"
          class="px-4 py-2 text-sm font-medium text-white rounded-lg transition-colors disabled:opacity-50"
          style="background: var(--color-accent)"
          :disabled="retrying"
          @click="retryFailed"
        >
          {{ retrying ? '重试中...' : `重试失败 (${status.failed})` }}
        </button>
      </div>
    </div>

    <!-- Status Cards -->
    <div class="grid grid-cols-4 gap-3">
      <div class="rounded-lg p-3" style="background: var(--color-bg-sunken); border: 1px solid var(--color-border-subtle)">
        <div class="text-2xl font-bold" style="color: var(--color-warning)">{{ status.pending }}</div>
        <div class="text-xs" style="color: var(--color-text-muted)">待处理</div>
      </div>
      <div class="rounded-lg p-3" style="background: var(--color-bg-sunken); border: 1px solid var(--color-border-subtle)">
        <div class="text-2xl font-bold" style="color: var(--color-link)">{{ status.processing }}</div>
        <div class="text-xs" style="color: var(--color-text-muted)">处理中</div>
      </div>
      <div class="rounded-lg p-3" style="background: var(--color-bg-sunken); border: 1px solid var(--color-border-subtle)">
        <div class="text-2xl font-bold" style="color: var(--color-success)">{{ status.completed }}</div>
        <div class="text-xs" style="color: var(--color-text-muted)">已完成</div>
      </div>
      <div class="rounded-lg p-3" style="background: var(--color-bg-sunken); border: 1px solid var(--color-border-subtle)">
        <div class="text-2xl font-bold" style="color: var(--color-error)">{{ status.failed }}</div>
        <div class="text-xs" style="color: var(--color-text-muted)">失败</div>
      </div>
    </div>

    <!-- Progress Bar -->
    <div v-if="status.total > 0" class="space-y-1">
      <div class="flex justify-between text-xs" style="color: var(--color-text-muted)">
        <span>总体进度</span>
        <span>{{ progressPercent }}% ({{ status.completed }}/{{ status.total }})</span>
      </div>
      <div class="h-2 rounded-full overflow-hidden" style="background: var(--color-border-medium)">
        <div
          class="h-full transition-all duration-300"
          style="background: var(--color-accent)"
          :style="{ width: `${progressPercent}%` }"
        />
      </div>
    </div>

    <!-- Filter -->
    <div class="flex items-center gap-2">
      <span class="text-sm" style="color: var(--color-text-muted)">筛选:</span>
      <button
        v-for="s in ['', 'pending', 'processing', 'completed', 'failed']"
        :key="s"
        class="px-3 py-1 text-xs rounded-full transition-colors"
        :style="statusFilter === s ? 'background: var(--color-accent); color: var(--color-text-inverted)' : 'background: var(--color-bg-sunken); color: var(--color-text-secondary)'"
        @click="changeFilter(s)"
      >
        {{ s === '' ? '全部' : getStatusLabel(s) }}
      </button>
    </div>

    <!-- Tasks Table -->
    <div v-if="loading" class="py-8 flex justify-center">
      <Icon icon="mdi:loading" width="28" height="28" class="animate-spin" style="color: var(--color-accent)" />
    </div>

    <div v-else-if="error" class="rounded-lg px-4 py-3 text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
      {{ error }}
    </div>

    <div v-else-if="tasks.length === 0" class="py-8 text-center" style="color: var(--color-text-muted)">
      暂无任务
    </div>

    <div v-else class="overflow-x-auto">
      <table class="w-full text-sm">
        <thead>
          <tr style="border-bottom: 1px solid var(--color-border-subtle)">
            <th class="text-left py-2 px-3 font-medium" style="color: var(--color-text-secondary)">标签</th>
            <th class="text-left py-2 px-3 font-medium" style="color: var(--color-text-secondary)">状态</th>
            <th class="text-left py-2 px-3 font-medium" style="color: var(--color-text-secondary)">创建时间</th>
            <th class="text-left py-2 px-3 font-medium" style="color: var(--color-text-secondary)">完成时间</th>
            <th class="text-left py-2 px-3 font-medium" style="color: var(--color-text-secondary)">错误信息</th>
          </tr>
        </thead>
        <tbody>
          <tr v-for="task in tasks" :key="task.id" class="hover:bg-[var(--color-bg-hover)]" style="border-bottom: 1px solid var(--color-border-subtle)">
            <td class="py-2 px-3">
              <span v-if="task.tag">{{ task.tag.label }}</span>
              <span v-else style="color: var(--color-text-muted)">Tag #{{ task.tag_id }}</span>
            </td>
            <td class="py-2 px-3">
              <span
                class="px-2 py-0.5 text-xs rounded-full"
                :style="getStatusStyle(task.status)"
              >
                {{ getStatusLabel(task.status) }}
              </span>
            </td>
            <td class="py-2 px-3" style="color: var(--color-text-secondary)">{{ formatDate(task.created_at) }}</td>
            <td class="py-2 px-3" style="color: var(--color-text-secondary)">{{ formatDate(task.completed_at) }}</td>
            <td class="py-2 px-3 text-xs max-w-xs truncate" style="color: var(--color-error)">
              {{ task.error_message || '-' }}
            </td>
          </tr>
        </tbody>
      </table>
    </div>

    <!-- Pagination -->
    <div v-if="totalPages > 1" class="flex items-center justify-between">
      <div class="text-sm" style="color: var(--color-text-muted)">
        共 {{ totalTasks }} 条任务
      </div>
      <div class="flex items-center gap-1">
        <button
          class="px-3 py-1 text-sm rounded hover:bg-[var(--color-bg-hover)] disabled:opacity-50"
          :disabled="currentPage <= 1"
          @click="changePage(currentPage - 1)"
        >
          上一页
        </button>
        <span class="px-3 py-1 text-sm">
          {{ currentPage }} / {{ totalPages }}
        </span>
        <button
          class="px-3 py-1 text-sm rounded hover:bg-[var(--color-bg-hover)] disabled:opacity-50"
          :disabled="currentPage >= totalPages"
          @click="changePage(currentPage + 1)"
        >
          下一页
        </button>
      </div>
    </div>
  </div>
</template>
