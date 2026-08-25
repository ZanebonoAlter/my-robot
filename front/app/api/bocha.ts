import { apiClient } from './client'
import type { ApiResponse } from '~/types'

export interface BochaStatus {
  enabled: boolean
  endpoint: string
  /** 脱敏 key（"已配置"提示或末 4 位，不会回显完整 key） */
  api_key?: string
  /** 是否已配置 key（脱敏布尔形式） */
  api_key_configured?: boolean
}

export interface BochaConfig {
  enabled: boolean
  endpoint: string
  /** 空串 = 不修改已有 key，非空才覆盖 */
  api_key: string
}

export function useBochaApi() {
  async function getStatus(): Promise<ApiResponse<BochaStatus>> {
    return apiClient.get('/settings/bocha')
  }

  async function saveSettings(config: BochaConfig): Promise<ApiResponse<BochaStatus>> {
    return apiClient.post('/settings/bocha', config)
  }

  return {
    getStatus,
    saveSettings,
  }
}
