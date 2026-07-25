import { apiClient } from './client'
import type { ApiResponse } from '~/types'
import type {
  AcceptedFeed,
  CatalogStatus,
  CatalogSyncSummary,
  DiscoveryRecommendation,
  RecommendationStatus,
  RefreshSummary,
} from '~/types/discovery'

/** 后端推荐卡片 payload（snake_case；route/board 预加载字段此处不展开）。 */
interface RecommendationPayload {
  id: number
  route_id: number
  board_id?: number | null
  source: string
  score: number
  llm_reason: string
  status: string
  created_at: string
  route_namespace: string
  route_path: string
  route_name: string
  route_example: string
  usable_directly: boolean
  requires_parameters: boolean
  parameters: string
  board_label: string
}

interface AcceptedFeedPayload {
  id: number
  title: string
  url: string
}

function normalizeCard(p: RecommendationPayload): DiscoveryRecommendation {
  return {
    id: String(p.id),
    routeId: String(p.route_id),
    boardId: p.board_id != null ? String(p.board_id) : null,
    boardLabel: p.board_label || '',
    source: p.source === 'qa' ? 'qa' : 'manual_refresh',
    score: p.score ?? 0,
    llmReason: p.llm_reason || '',
    status: (p.status || 'pending') as RecommendationStatus,
    routeNamespace: p.route_namespace || '',
    routePath: p.route_path || '',
    routeName: p.route_name || '',
    routeExample: p.route_example || '',
    usableDirectly: Boolean(p.usable_directly),
    requiresParameters: Boolean(p.requires_parameters),
    parameters: p.parameters || '{}',
    createdAt: p.created_at || '',
  }
}

/**
 * 订阅源发现 API：推荐卡片 / 问答 / 目录同步。
 * 所有 HTTP 经 ApiClient；snake→camel 与 id 字符串化在 normalizer 完成。
 */
export function useDiscoveryApi() {
  /** GET /api/discovery/recommendations?status=pending — 推荐卡片列表。 */
  async function getRecommendations(status: RecommendationStatus = 'pending'): Promise<ApiResponse<DiscoveryRecommendation[]>> {
    const res = await apiClient.get<RecommendationPayload[]>(`/discovery/recommendations?status=${status}`)
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeCard) }
    return { ...res, data: [] as DiscoveryRecommendation[] }
  }

  /** POST /api/discovery/recommendations/refresh — 换一批（粗筛+精排+幂等落库）。 */
  async function refreshRecommendations(): Promise<ApiResponse<RefreshSummary>> {
    return apiClient.post<RefreshSummary>('/discovery/recommendations/refresh', {})
  }

  /** POST /api/discovery/recommendations/:id/accept — 接受（直订 / 填参验证后订阅）。 */
  async function acceptRecommendation(
    id: string,
    opts: { categoryId?: string, parameters?: Record<string, string> } = {},
  ): Promise<ApiResponse<AcceptedFeed>> {
    const body: Record<string, unknown> = {}
    if (opts.categoryId) body.category_id = Number(opts.categoryId)
    if (opts.parameters && Object.keys(opts.parameters).length > 0) body.parameters = opts.parameters
    const res = await apiClient.post<AcceptedFeedPayload>(`/discovery/recommendations/${id}/accept`, body)
    if (res.success && res.data) {
      return { ...res, data: { id: String(res.data.id), title: res.data.title, url: res.data.url } }
    }
    return { success: false, error: res.error }
  }

  /** POST /api/discovery/recommendations/:id/dismiss — 拒绝（冷却期内不再推荐）。 */
  async function dismissRecommendation(id: string): Promise<ApiResponse<null>> {
    return apiClient.post<null>(`/discovery/recommendations/${id}/dismiss`, {})
  }

  /** POST /api/discovery/ask — 问答式即时推荐（落库 + 种子写入）。 */
  async function ask(question: string): Promise<ApiResponse<DiscoveryRecommendation[]>> {
    const res = await apiClient.post<RecommendationPayload[]>('/discovery/ask', { question })
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeCard) }
    return { success: false, error: res.error }
  }

  /** GET /api/discovery/catalog/status — 目录状态统计。 */
  async function getCatalogStatus(): Promise<ApiResponse<CatalogStatus>> {
    return apiClient.get<CatalogStatus>('/discovery/catalog/status')
  }

  /** POST /api/discovery/catalog/sync — 手动触发目录同步。 */
  async function syncCatalog(): Promise<ApiResponse<CatalogSyncSummary>> {
    return apiClient.post<CatalogSyncSummary>('/discovery/catalog/sync', {})
  }

  return {
    getRecommendations,
    refreshRecommendations,
    acceptRecommendation,
    dismissRecommendation,
    ask,
    getCatalogStatus,
    syncCatalog,
  }
}
