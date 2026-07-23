import type { ApiResponse } from '~/types'
import type { SchedulerStatus, SchedulerTriggerResult } from '~/types/scheduler'
import { apiClient } from './client'
import { buildQueryString } from '~/utils/api-helpers'

async function triggerSchedulerRequest(name: string, params?: Record<string, string>): Promise<ApiResponse<SchedulerTriggerResult>> {
  const query = params ? buildQueryString(params) : ''
  return apiClient.post<SchedulerTriggerResult>(`/schedulers/${name}/trigger${query}`, {})
}

export function useSchedulerApi() {
  return {
    async getSchedulersStatus() {
      return apiClient.get<SchedulerStatus[]>('/schedulers/status')
    },

    async getSchedulerStatus(name: string) {
      return apiClient.get<SchedulerStatus>(`/schedulers/${name}/status`)
    },

    async triggerScheduler(name: string, params?: Record<string, string>) {
      return triggerSchedulerRequest(name, params)
    },

    async updateScheduleTime(name: string, time: string) {
      return apiClient.put<{ name: string; time: string }>(`/schedulers/${name}/schedule-time`, { time })
    },

  }
}
