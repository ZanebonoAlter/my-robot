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
</script>

<template>
  <div class="space-y-6">
    <div v-if="schedulerSuccess" class="p-3 bg-green-50 border border-green-200 rounded-lg text-sm text-green-600">
      {{ schedulerSuccess }}
    </div>
    <div v-if="schedulerError" class="p-3 bg-red-50 border border-red-200 rounded-lg text-sm text-red-600">
      {{ schedulerError }}
    </div>

    <div v-if="schedulerLoading && schedulerStatuses.length === 0" class="flex items-center justify-center py-12">
      <Icon icon="mdi:loading" width="48" height="48" class="animate-spin text-blue-500" />
    </div>

    <div v-else class="grid grid-cols-1 gap-4">
      <div
        v-for="scheduler in schedulerStatuses"
        :key="scheduler.name"
        class="border border-gray-200 rounded-xl p-4"
      >
        <div class="flex items-start justify-between gap-4">
          <div class="flex items-start gap-3 min-w-0">
            <span
              class="w-2 h-2 rounded-full mt-1.5 shrink-0"
              :class="getSchedulerColor(scheduler.database_state?.status || scheduler.status || '')"
            />
            <div class="min-w-0">
              <h4 class="font-medium text-gray-900 truncate">
                <Icon v-if="getSchedulerIcon(scheduler.name)" :icon="getSchedulerIcon(scheduler.name)!" width="16" class="inline mr-1.5 -mt-0.5" />
                {{ getSchedulerDisplayName(scheduler.name) }}
              </h4>
              <p class="text-xs text-gray-500 mt-0.5 truncate">{{ scheduler.name }}</p>
              <div v-if="shouldShowContentCompletionPanel(scheduler) && scheduler.current_article" class="mt-2 text-xs text-blue-600 bg-blue-50 rounded-lg px-2 py-1">
                当前处理：{{ scheduler.current_article.title || '无标题' }}
              </div>
            </div>
          </div>
          <div class="flex items-center gap-2 shrink-0">
            <span
              v-if="scheduler.database_state?.status"
              class="px-2 py-0.5 text-xs font-medium rounded-full"
              :class="{
                'bg-green-100 text-green-700': scheduler.database_state.status === 'running',
                'bg-yellow-100 text-yellow-700': scheduler.database_state.status === 'triggered',
                'bg-red-100 text-red-700': scheduler.database_state.status === 'error',
                'bg-gray-100 text-gray-600': !['running', 'triggered', 'error'].includes(scheduler.database_state.status),
              }"
            >
              {{ getSchedulerStatusLabel(scheduler) }}
            </span>
            <button
              type="button"
              class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
              :class="isSchedulerBusy(scheduler) || schedulerTriggerLoading
                ? 'bg-gray-100 text-gray-400 cursor-not-allowed'
                : 'bg-blue-50 text-blue-600 hover:bg-blue-100'"
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
            :class="getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.accepted
              ? 'bg-green-50 text-green-700'
              : 'bg-red-50 text-red-700'"
          >
            {{ getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.message || (getSchedulerFeedback(schedulerTriggerFeedback, scheduler.name)?.accepted ? '已接受请求' : '请求被拒绝') }}
          </div>
        </div>
      </div>
    </div>
  </div>
</template>
