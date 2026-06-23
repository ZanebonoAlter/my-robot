import { apiClient } from './client'
import { buildQueryString } from '~/utils/api-helpers'
import type { ApiResponse } from '~/types'

/**
 * 通用队列状态接口
 */
export interface QueueStatus {
  pending: number
  processing: number
  completed: number
  failed: number
  total: number
}

/**
 * 通用队列 API 接口
 */
export interface QueueApi<T> {
  getStatus(): Promise<ApiResponse<QueueStatus>>
  getTasks(params?: { status?: string; limit?: number; offset?: number }): Promise<ApiResponse<{ tasks: T[]; total: number }>>
  retryFailed(): Promise<ApiResponse<{ message: string }>>
}

/**
 * 创建泛型队列 API
 * 替代 tagQueue 和 embeddingQueue 的重复代码
 */
export function createQueueApi<T>(endpoint: string): QueueApi<T> {
  return {
    getStatus(): Promise<ApiResponse<QueueStatus>> {
      return apiClient.get<QueueStatus>(`/${endpoint}/status`)
    },

    getTasks(params?: { status?: string; limit?: number; offset?: number }): Promise<ApiResponse<{ tasks: T[]; total: number }>> {
      const qs = buildQueryString(params as Record<string, unknown>)
      return apiClient.get<{ tasks: T[]; total: number }>(`/${endpoint}/tasks${qs}`)
    },

    retryFailed(): Promise<ApiResponse<{ message: string }>> {
      return apiClient.post<{ message: string }>(`/${endpoint}/retry`)
    },
  }
}
