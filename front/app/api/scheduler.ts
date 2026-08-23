import type { ApiResponse } from '~/types'
import type { AnalysisPauseState, SchedulerAIHealthRoute, SchedulerStatus, SchedulerTriggerResult } from '~/types/scheduler'
import { apiClient } from './client'
import { buildQueryString } from '~/utils/api-helpers'

// /schedulers/status 顶层除 data 外还带 analysis_paused / analysis_paused_at / ai_healthy / ai_health_routes
export type SchedulersStatusResponse = ApiResponse<SchedulerStatus[]> & {
  analysis_paused?: boolean
  analysis_paused_at?: string
  ai_healthy?: boolean
  ai_health_routes?: SchedulerAIHealthRoute[]
}

async function triggerSchedulerRequest(name: string, params?: Record<string, string>): Promise<ApiResponse<SchedulerTriggerResult>> {
  const query = params ? buildQueryString(params) : ''
  return apiClient.post<SchedulerTriggerResult>(`/schedulers/${name}/trigger${query}`, {})
}

export function useSchedulerApi() {
  return {
    async getSchedulersStatus(): Promise<SchedulersStatusResponse> {
      return apiClient.get<SchedulerStatus[]>('/schedulers/status') as Promise<SchedulersStatusResponse>
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

    async getAnalysisPause() {
      return apiClient.get<AnalysisPauseState>('/analysis/pause')
    },

    async setAnalysisPause(paused: boolean) {
      return apiClient.post<AnalysisPauseState>('/analysis/pause', { paused })
    },

  }
}
