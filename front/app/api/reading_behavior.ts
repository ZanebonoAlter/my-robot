import { apiClient } from './client'
import type {
  ApiResponse,
  ReadingBehaviorEvent,
  ReadingStats,
} from '~/types'

export function useReadingBehaviorApi() {
  async function trackBehaviorBatch(events: ReadingBehaviorEvent[]): Promise<ApiResponse<void>> {
    return apiClient.post<void>('/reading-behavior/track-batch', { events })
  }

  async function getReadingStats(): Promise<ApiResponse<ReadingStats>> {
    return apiClient.get<ReadingStats>('/reading-behavior/stats')
  }

  return {
    trackBehaviorBatch,
    getReadingStats,
  }
}
