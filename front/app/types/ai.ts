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
  /** 模型类型：llm=对话/总结类，embedding=向量嵌入类 */
  model_kind: 'llm' | 'embedding'
  /** 是否已配置本地进程启动命令（命令原文不下发） */
  start_command_configured: boolean
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
  model_kind?: 'llm' | 'embedding'
  /** 本地进程启动命令；留空=外部托管服务（更新时不改动已保存值） */
  start_command?: string
  /** true 时清除已保存的启动命令 */
  clear_start_command?: boolean
  enabled?: boolean
  timeout_seconds?: number
  max_tokens?: number | null
  temperature?: number | null
  enable_thinking?: boolean
  metadata?: string
  clear_api_key?: boolean
}

/** GET /api/ai/health 的单条路由健康明细 */
export interface AIHealthRoute {
  route_name: string
  capability: string
  primary_provider: string
  model_kind: string
  reachable: boolean
  launched_by_backend: boolean
  last_checked: string
  error: string
}

/** GET /api/ai/health 的健康快照（checked_at 为 null 表示启动后首次检测未完成） */
export interface AIHealthSnapshot {
  healthy: boolean
  checked_at: string | null
  auto_start_models: boolean
  routes: AIHealthRoute[]
}

/** POST /api/ai/health/reprobe 的结果（triggered=已启动一次异步重探；skipped=已有探测在跑，未并发） */
export interface AIHealthReprobeResult {
  triggered: boolean
  skipped: boolean
}

