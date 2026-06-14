<script setup lang="ts">
import { onMounted, ref } from 'vue'
import { Icon } from '@iconify/vue'
import SchedulerStatusPanel from '~/components/dialog/SchedulerStatusPanel.vue'
import { apiClient } from '~/api/client'

const dailyReportTime = ref('21:00')
const savingTime = ref(false)
const timeSaved = ref(false)
const timeError = ref('')

onMounted(async () => {
  const res = await apiClient.get<Record<string, unknown>>('/api/settings')
  if (res.success && res.data) {
    const val = res.data.daily_report_time
    if (typeof val === 'string' && val.length > 0) {
      // Strip surrounding quotes if stored as JSON string
      dailyReportTime.value = val.replace(/^"|"$/g, '')
    }
  }
})

async function saveDailyReportTime() {
  savingTime.value = true
  timeError.value = ''
  timeSaved.value = false

  const res = await apiClient.post('/api/settings', { daily_report_time: dailyReportTime.value })
  savingTime.value = false

  if (res.success) {
    timeSaved.value = true
    setTimeout(() => { timeSaved.value = false }, 2000)
  } else {
    timeError.value = res.error || '保存失败'
  }
}
</script>

<template>
  <div class="space-y-6">
    <!-- Daily Report Time Setting -->
    <div class="rounded-xl p-4" style="border: 1px solid var(--color-border-subtle)">
      <div class="flex items-center gap-3 mb-3">
        <Icon icon="mdi:clock-edit-outline" width="20" style="color: var(--color-text-secondary)" />
        <div>
          <h4 class="font-medium text-sm" style="color: var(--color-text-primary)">日报生成时刻</h4>
          <p class="text-xs" style="color: var(--color-text-muted)">设置每日报告自动生成的时间（HH:MM 格式）</p>
        </div>
      </div>
      <div class="flex items-center gap-3">
        <input
          v-model="dailyReportTime"
          type="time"
          class="px-3 py-1.5 text-sm rounded-lg"
          style="border: 1px solid var(--color-border-subtle); background: var(--color-bg-sunken); color: var(--color-text-primary)"
        />
        <button
          class="px-3 py-1.5 text-xs font-medium rounded-lg transition-colors"
          :style="savingTime
            ? 'background: var(--color-bg-sunken); color: var(--color-text-muted); cursor: not-allowed'
            : 'background: var(--color-bg-sunken); color: var(--color-text-secondary)'"
          :disabled="savingTime"
          @click="saveDailyReportTime"
        >
          <Icon v-if="savingTime" icon="mdi:loading" width="14" class="animate-spin inline mr-1" />
          保存
        </button>
        <span v-if="timeSaved" class="text-xs" style="color: var(--color-success)">已保存</span>
        <span v-if="timeError" class="text-xs" style="color: var(--color-error)">{{ timeError }}</span>
      </div>
    </div>

    <!-- Scheduler Status Panel -->
    <SchedulerStatusPanel />
  </div>
</template>
