import { apiClient } from './client'
import type { ApiResponse } from '~/types'

export interface ProxyStatus {
  http_proxy_url: string
  configured: boolean
}

export interface ProxyConfig {
  http_proxy_url: string
}

export function useProxyApi() {
  async function getStatus(): Promise<ApiResponse<ProxyStatus>> {
    return apiClient.get('/settings/proxy')
  }

  async function saveSettings(config: ProxyConfig): Promise<ApiResponse<ProxyStatus>> {
    return apiClient.post('/settings/proxy', config)
  }

  return {
    getStatus,
    saveSettings,
  }
}
