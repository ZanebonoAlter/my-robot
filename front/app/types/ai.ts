/**
 * AI 相关类型定义
 */

/**
 * AI 设置数据
 */
export interface AISettings {
  baseURL?: string
  apiKey?: string
  model?: string
  summaryEnabled?: boolean
  providerId?: number
  providerName?: string
  routeName?: string
  timeRange?: number
  apiKeyConfigured?: boolean
}

export interface AIProvider {
  id: number
  name: string
  provider_type: string
  base_url: string
  model: string
  enabled: boolean
  timeout_seconds: number
  max_tokens?: number | null
  temperature?: number | null
  enable_thinking: boolean
  metadata?: string
  api_key_configured: boolean
}

export interface AIRouteProviderLink {
  id: number
  route_id: number
  provider_id: number
  priority: number
  enabled: boolean
  provider: AIProvider
}

export interface AIRoute {
  id: number
  name: string
  capability: string
  enabled: boolean
  strategy: string
  description: string
  route_providers: AIRouteProviderLink[]
}

export interface AIProviderUpsertRequest {
  name: string
  provider_type?: string
  base_url: string
  api_key?: string
  model: string
  enabled?: boolean
  timeout_seconds?: number
  max_tokens?: number | null
  temperature?: number | null
  enable_thinking?: boolean
  metadata?: string
}

