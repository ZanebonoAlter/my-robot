<script setup lang="ts">
import { Icon } from '@iconify/vue'
import { onMounted } from 'vue'
import { useSchedulerStatus } from '~/composables/useSchedulerStatus'
import {
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
      </div>
    </div>
  </div>
</template>
