import { apiClient } from './client'
import type { ApiResponse } from '~/types'
import type {
  PreferenceProfileItem,
  PreferenceSource,
  RecomputeSummary,
} from '~/types/discovery'

/** 后端画像条目 payload（snake_case）。 */
interface PreferenceProfilePayload {
  board_id?: number | null
  board_label: string
  source: string
  tag_weights: Record<string, number>
  last_computed_at?: string | null
}

function normalizeItem(p: PreferenceProfilePayload): PreferenceProfileItem {
  return {
    boardId: p.board_id != null ? String(p.board_id) : null,
    boardLabel: p.board_label || '全局',
    source: (p.source === 'seed' ? 'seed' : 'behavior') as PreferenceSource,
    tagWeights: p.tag_weights || {},
    lastComputedAt: p.last_computed_at ?? null,
  }
}

/**
 * 兴趣画像 API：按版块聚合的偏好向量画像 + 手动重算。
 */
export function usePreferenceProfileApi() {
  /** GET /api/preference-profile — 画像读取（无数据返回空列表）。 */
  async function getProfile(): Promise<ApiResponse<PreferenceProfileItem[]>> {
    const res = await apiClient.get<PreferenceProfilePayload[]>('/preference-profile')
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeItem) }
    return { ...res, data: [] as PreferenceProfileItem[] }
  }

  /** POST /api/preference-profile/recompute — 手动触发重算（与 scheduler 同路径）。 */
  async function recompute(): Promise<ApiResponse<RecomputeSummary>> {
    return apiClient.post<RecomputeSummary>('/preference-profile/recompute', {})
  }

  return {
    getProfile,
    recompute,
  }
}
