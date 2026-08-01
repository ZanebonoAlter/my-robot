<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted, ref } from 'vue'
import { useSchedulerStatus } from '~/composables/useSchedulerStatus'
import {
  formatLastRunSummary,
  getSchedulerColor,
  getSchedulerDisplayName,
  getSchedulerIcon,
  getSchedulerStatusLabel,
  isContentCompletionScheduler,
  mapStatusToChinese,
  shouldShowContentCompletionPanel,
} from '~/utils/schedulerMeta'

const {
  schedulerStatuses, schedulerTriggerFeedback, schedulerLoading,
  schedulerTriggerLoading, schedulerError, schedulerSuccess,
  loadSchedulersStatus, triggerScheduler,
  scheduleTimeLoading, updateScheduleTime,
} = useSchedulerStatus()

onMounted(() => {
  loadSchedulersStatus()
})

function isSchedulerBusy(scheduler: { is_executing?: boolean; database_state?: { status?: string } }): boolean {
  return scheduler.is_executing === true || scheduler.database_state?.status === 'running'
}

function getSchedulerFeedback(feedback: Record<string, { accepted: boolean; started?: boolean; reason?: string; message?: string } | undefined>, name: string) {
  return feedback[name]
}

function getStatusStyle(status: string) {
  switch (status) {
    case 'running': return { background: 'var(--color-success-bg, rgba(61, 138, 74, 0.1))', color: 'var(--color-success)' }
    case 'triggered': return { background: 'var(--color-warning-bg, rgba(196, 136, 60, 0.1))', color: 'var(--color-warning)' }
    case 'error': return { background: 'var(--color-error-bg, rgba(196, 47, 60, 0.1))', color: 'var(--color-error)' }
    default: return { background: 'var(--color-bg-sunken)', color: 'var(--color-text-muted)' }
  }
}

function getStatusDotStyle(status: string) {
  switch (status) {
    case 'running': return { background: 'var(--color-success)' }
    case 'triggered': return { background: 'var(--color-warning)' }
    case 'error': return { background: 'var(--color-error)' }
    default: return { background: 'var(--color-text-muted)' }
  }
}

function isWallClockScheduler(name: string): boolean {
  return name === 'board_upgrade_suggest' || name === 'daily_report'
}

const editingScheduleName = ref<string | null>(null)
const editingScheduleTime = ref('')

function startEditSchedule(scheduler: { name: string; schedule_time?: string }) {
  editingScheduleName.value = scheduler.name
  editingScheduleTime.value = scheduler.schedule_time || ''
}

function cancelEditSchedule() {
  editingScheduleName.value = null
  editingScheduleTime.value = ''
}

async function saveScheduleTime(name: string) {
  const ok = await updateScheduleTime(name, editingScheduleTime.value)
  if (ok) {
    cancelEditSchedule()
  }
}
</script>

<template>
  <div class="space-y-6">
    <div v-if="schedulerSuccess" class="p-3 rounded-lg text-sm" style="background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); border: 1px solid var(--color-success-border, rgba(61, 138, 74, 0.25)); color: var(--color-success)">
      {{ schedulerSuccess }}
    </div>
    <div v-if="schedulerError" class="p-3 rounded-lg text-sm" style="background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); border: 1px solid var(--color-error-border, rgba(196, 47, 60, 0.25)); color: var(--color-error)">
      {{ schedulerError }}
    </div>

    <div v-if="schedulerLoading && schedulerStatuses.length === 0" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin" style="color: var(--color-link)" />
    </div>

    <div v-else class="grid grid-cols-1 gap-4">
      <div
        v-for="scheduler in schedulerStatuses"
        :key="scheduler.name"
        class="rounded-xl p-4"
        style="border: 1px solid var(--color-border-subtle)"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-start gap-3 min-w-0">
            <span
              class="w-2 h-2 rounded-full mt-1.5 shrink-0"
              :style="getStatusDotStyle(scheduler.database_state?.status || scheduler.status || '')"
            />
            <div class="min-w-0">
              <h4 class="font-medium truncate" style="color: var(--color-text-primary)">
                <Icon v-if="getSchedulerIcon(scheduler.name)" :icon="getSchedulerIcon(scheduler.name)!" width="16" class="inline mr-1.5 -mt-0.5" />
                {{ getSchedulerDisplayName(scheduler.name) }}
              </h4>
              <p class="text-[11px] mt-0.5 truncate font-mono" style="color: var(--color-text-muted); opacity: 0.7">{{ scheduler.name }}</p>
              <div v-if="shouldShowContentCompletionPanel(scheduler) && scheduler.current_article" class="mt-2 text-xs rounded-lg px-2 py-1" style="background: var(--color-link-subtle); color: var(--color-link)">
                当前处理：{{ scheduler.current_article.title || '无标题' }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <span
              v-if="scheduler.database_state?.status"
              class="px-2 py-0.5 text-xs font-medium rounded-full"
              :style="getStatusStyle(scheduler.database_state.status)"
            >
              {{ getSchedulerStatusLabel(scheduler) }}
            </span>
            <button
              type="button"
              class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
              :style="isSchedulerBusy(scheduler) || schedulerTriggerLoading
                ? 'background: var(--color-bg-sunken); color: var(--color-text-muted); cursor: not-allowed'
                : 'background: var(--color-bg-sunken); color: var(--color-text-secondary)'"
              :disabled="isSchedulerBusy(scheduler) || schedulerTriggerLoading"
              @click="triggerScheduler(scheduler.name)"
            >
              <Icon v-if="schedulerTriggerLoading" icon="mdi:loading" width="14" class="animate-spin inline mr-1" />
              执行
            </button>
          </div>
        </div>

        <!-- Wall-clock schedule time editor (board_upgrade_suggest / daily_report) -->
        <div v-if="isWallClockScheduler(scheduler.name)" class="mt-3 flex flex-wrap items-center gap-2 text-xs" style="color: var(--color-text-secondary)">
          <Icon icon="mdi:clock-outline" width="14" class="shrink-0" />
          <span>定时</span>
          <template v-if="editingScheduleName !== scheduler.name">
            <span class="font-mono">{{ scheduler.schedule_time || '—' }}</span>
            <button
              type="button"
              class="px-2 py-0.5 text-xs rounded-md transition-colors"
              style="background: var(--color-bg-sunken); color: var(--color-text-muted)"
              :disabled="scheduleTimeLoading"
              @click="startEditSchedule(scheduler)"
            >
              <Icon icon="mdi:pencil-outline" width="12" class="inline -mt-0.5" />
              编辑
            </button>
          </template>
          <template v-else>
            <input
              v-model="editingScheduleTime"
              type="time"
              class="px-2 py-0.5 text-xs rounded-md font-mono"
              style="border: 1px solid var(--color-border-subtle); background: var(--color-bg-sunken); color: var(--color-text-primary)"
            >
            <button
              type="button"
              class="px-2 py-0.5 text-xs font-medium rounded-md transition-colors"
              :style="scheduleTimeLoading
                ? 'background: var(--color-bg-sunken); color: var(--color-text-muted); cursor: not-allowed'
                : 'background: var(--color-link); color: #fff'"
              :disabled="scheduleTimeLoading"
              @click="saveScheduleTime(scheduler.name)"
            >
              <Icon v-if="scheduleTimeLoading" icon="mdi:loading" width="12" class="animate-spin inline -mt-0.5" />
              保存
            </button>
            <button
              type="button"
              class="px-2 py-0.5 text-xs rounded-md transition-colors"
              style="background: var(--color-bg-sunken); color: var(--color-text-secondary)"
              :disabled="scheduleTimeLoading"
              @click="cancelEditSchedule"
            >
              取消
            </button>
          </template>
        </div>

        <div v-if="getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)" class="mt-3">
          <div
            class="text-xs rounded-lg px-3 py-2"
            :style="getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.accepted
              ? 'background: var(--color-success-bg, rgba(61, 138, 74, 0.1)); color: var(--color-success)'
              : 'background: var(--color-error-bg, rgba(196, 47, 60, 0.1)); color: var(--color-error)'"
          >
            {{ getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.message || (getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.accepted ? '已接受请求' : '请求被拒绝') }}
          </div>
        </div>

        <!-- Last execution details -->
        <div
          v-if="scheduler.is_executing
            || scheduler.database_state?.last_execution_time
            || (scheduler.last_run_summary && formatLastRunSummary(scheduler.name, scheduler.last_run_summary))
            || (scheduler.database_state?.failed_executions && scheduler.database_state.failed_executions > 0)"
          class="mt-3 space-y-1"
        >
          <!-- Executing state -->
          <div v-if="scheduler.is_executing" class="text-xs font-medium" style="color: var(--color-warning)">
            执行中…
          </div>

          <!-- Last execution time + duration -->
          <div v-if="scheduler.database_state?.last_execution_time && !scheduler.is_executing" class="text-xs" style="color: var(--color-text-muted)">
            <span>上次执行：{{ scheduler.database_state.last_execution_time }}</span>
            <span v-if="scheduler.database_state.last_execution_duration != null">
              &nbsp;耗时 {{ typeof scheduler.database_state.last_execution_duration === 'number' ? scheduler.database_state.last_execution_duration.toFixed(1) : scheduler.database_state.last_execution_duration }}s
            </span>
          </div>

          <!-- Result summary -->
          <div
            v-if="!scheduler.is_executing && scheduler.last_run_summary && formatLastRunSummary(scheduler.name, scheduler.last_run_summary)"
            class="text-xs"
            style="color: var(--color-text-secondary)"
          >
            结果：{{ formatLastRunSummary(scheduler.name, scheduler.last_run_summary) }}
          </div>

          <!-- Failure info (only when > 0) -->
          <div
            v-if="!scheduler.is_executing && (scheduler.database_state?.consecutive_failures ?? 0) > 0"
            class="text-xs"
            style="color: var(--color-error)"
          >
            失败：{{ scheduler.database_state?.failed_executions ?? 0 }} 次
            <span v-if="scheduler.database_state?.last_error"> — {{ scheduler.database_state.last_error }}</span>
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
