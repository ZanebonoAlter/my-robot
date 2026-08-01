import { onUnmounted, ref } from 'vue'
import { useSchedulerApi } from '~/api'
import type { SchedulerStatus, SchedulerTriggerResult } from '~/types/scheduler'
import { isHotScheduler } from '~/utils/schedulerMeta'

export function useSchedulerStatus() {
  const schedulerStatuses = ref<SchedulerStatus[]>([])
  // 分析暂停总闸是后端全局开关，用 useState 跨组件共享（同 useNotify 模式，SSR 安全）
  const analysisPaused = useState<boolean>('scheduler:analysis-paused', () => false)
  const analysisPausedAt = useState<string>('scheduler:analysis-paused-at', () => '')
  const schedulerTriggerFeedback = ref<Record<string, SchedulerTriggerResult | undefined>>({})
  const lastSchedulerTriggerAt = ref<number | null>(null)
  const schedulerLoading = ref(false)
  const schedulerTriggerLoading = ref(false)
  const scheduleTimeLoading = ref(false)
  const schedulerError = ref<string | null>(null)
  const schedulerSuccess = ref<string | null>(null)
  let schedulerPollTimer: ReturnType<typeof setTimeout> | null = null

  async function loadSchedulersStatus() {
    schedulerLoading.value = true
    schedulerError.value = null
    try {
      const { getSchedulersStatus } = useSchedulerApi()
      const response = await getSchedulersStatus()
      if (response.success && response.data) {
        schedulerStatuses.value = response.data
        analysisPaused.value = response.analysis_paused === true
        analysisPausedAt.value = response.analysis_paused_at ?? ''
      }
    } catch {
      schedulerError.value = '加载定时任务状态失败'
    } finally {
      schedulerLoading.value = false
      scheduleSchedulerPolling()
    }
  }

  function stopSchedulerPolling() {
    if (schedulerPollTimer) {
      clearTimeout(schedulerPollTimer)
      schedulerPollTimer = null
    }
  }

  function scheduleSchedulerPolling() {
    stopSchedulerPolling()
    const hasHotScheduler = schedulerStatuses.value.some(
      s => isHotScheduler(s.name) && s.is_executing === true,
    )
    const hasRecentFeedback = lastSchedulerTriggerAt.value !== null
      && Date.now() - lastSchedulerTriggerAt.value < 20000
    const interval = hasHotScheduler ? 8000 : hasRecentFeedback ? 15000 : 30000

    schedulerPollTimer = setTimeout(() => {
      loadSchedulersStatus()
    }, interval)
  }

  async function triggerScheduler(name: string) {
    schedulerTriggerLoading.value = true
    schedulerError.value = null
    schedulerSuccess.value = null
    try {
      const { triggerScheduler: trigger } = useSchedulerApi()
      const response = await trigger(name)
      if (response.success) {
        schedulerTriggerFeedback.value[name] = response.data
        lastSchedulerTriggerAt.value = Date.now()
        schedulerSuccess.value = response.data?.message || response.message || '任务请求已处理'
        setTimeout(() => { schedulerSuccess.value = null }, 2000)
        await loadSchedulersStatus()
      } else {
        schedulerTriggerFeedback.value[name] = response.data ?? {
          name, accepted: false, started: false, reason: 'request_rejected', message: '请求被拒绝',
        }
        lastSchedulerTriggerAt.value = Date.now()
        schedulerError.value = response.error || '触发失败'
      }
    } catch {
      schedulerTriggerFeedback.value[name] = {
        name, accepted: false, started: false, reason: 'request_failed', message: '请求失败',
      }
      lastSchedulerTriggerAt.value = Date.now()
      schedulerError.value = '触发失败'
    } finally {
      schedulerTriggerLoading.value = false
    }
  }

  async function updateScheduleTime(name: string, time: string) {
    scheduleTimeLoading.value = true
    schedulerError.value = null
    schedulerSuccess.value = null
    try {
      const { updateScheduleTime: update } = useSchedulerApi()
      const response = await update(name, time)
      if (response.success) {
        schedulerSuccess.value = response.message || '定时时间已更新'
        setTimeout(() => { schedulerSuccess.value = null }, 2000)
        await loadSchedulersStatus()
        return true
      }
      schedulerError.value = response.error || '更新定时时间失败'
      return false
    } catch {
      schedulerError.value = '更新定时时间失败'
      return false
    } finally {
      scheduleTimeLoading.value = false
    }
  }

  async function setAnalysisPaused(paused: boolean): Promise<{ ok: boolean; message: string }> {
    try {
      const { setAnalysisPause } = useSchedulerApi()
      const response = await setAnalysisPause(paused)
      if (response.success && response.data) {
        analysisPaused.value = response.data.paused
        analysisPausedAt.value = response.data.paused_at ?? ''
        return { ok: true, message: response.message || (paused ? '分析已暂停' : '分析已恢复') }
      }
      return { ok: false, message: response.error || '操作失败' }
    } catch {
      return { ok: false, message: '操作失败' }
    }
  }

  onUnmounted(() => {
    stopSchedulerPolling()
  })

  return {
    schedulerStatuses, schedulerTriggerFeedback, schedulerLoading,
    schedulerTriggerLoading, schedulerError, schedulerSuccess,
    analysisPaused, analysisPausedAt, setAnalysisPaused,
    loadSchedulersStatus, stopSchedulerPolling, triggerScheduler,
    scheduleTimeLoading, updateScheduleTime,
  }
}
