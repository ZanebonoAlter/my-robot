import { apiClient } from './client'
import type {
  ApiResponse,
  ReadingBehaviorEvent,
  ReadingStats,
  UserPreference,
} from '~/types'

export function useReadingBehaviorApi() {
  async function trackBehaviorBatch(events: ReadingBehaviorEvent[]): Promise<ApiResponse<void>> {
    return apiClient.post<void>('/reading-behavior/track-batch', { events })
  }

  async function getReadingStats(): Promise<ApiResponse<ReadingStats>> {
    return apiClient.get<ReadingStats>('/reading-behavior/stats')
  }

  async function getUserPreferences(type?: 'feed' | 'category'): Promise<ApiResponse<UserPreference[]>> {
    const query = type ? `?type=${type}` : ''
    return apiClient.get<UserPreference[]>(`/user-preferences${query}`)
  }

  async function triggerPreferenceUpdate(): Promise<ApiResponse<void>> {
    return apiClient.post<void>('/user-preferences/update', {})
  }

  return {
    trackBehaviorBatch,
    getReadingStats,
    getUserPreferences,
    triggerPreferenceUpdate,
  }
}
