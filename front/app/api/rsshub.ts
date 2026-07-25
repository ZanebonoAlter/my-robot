import { apiClient } from './client'
import type { ApiResponse } from '~/types'

export interface RSSHubStatus {
  rsshub_base_url: string
  configured: boolean
  default: string
}

export interface RSSHubConfig {
  rsshub_base_url: string
}

export function useRsshubApi() {
  async function getStatus(): Promise<ApiResponse<RSSHubStatus>> {
    return apiClient.get('/settings/rsshub')
  }

  async function saveSettings(config: RSSHubConfig): Promise<ApiResponse<RSSHubStatus>> {
    return apiClient.post('/settings/rsshub', config)
  }

  return {
    getStatus,
    saveSettings,
  }
}
