import { apiClient } from './client'
import { camelizeKeys, mapApiResponse } from '~/utils/api-helpers'
import type { ApiResponse } from '~/types'

/** 关注标记状态：active 参与命中判定；paused 跳过当期判定。 */
export type TopicWatchStatus = 'active' | 'paused'

/** 关注标记（camelCase；数字 id 在 API 边界转字符串）。
 *  与 persistent_topic 完全隔离：独立实体、独立 ID 序列、无生命周期。 */
export interface TopicWatch {
  id: string
  semanticBoardId: string
  label: string
  status: TopicWatchStatus
  createdAt: string
  updatedAt: string
}

/** 一条关注命中：某个 section 被某个关注命中 + AI 给出的一句话理由。
 *  命中是只读叠加标记，不改 section 归属、不推进持久话题生命周期。 */
export interface TopicWatchHit {
  id: string
  watchId: string
  sectionId: string
  reportId: string
  periodDate: string
  reason: string
}

/** Backend snake_case payload for a watch row. */
interface TopicWatchPayload {
  id: number
  semantic_board_id: number
  label: string
  status: string
  created_at: string
  updated_at: string
}

/** Backend snake_case payload for a hit row. */
interface TopicWatchHitPayload {
  id: number
  watch_id: number
  section_id: number
  report_id: number
  period_date: string
  reason: string
}

function normalizeWatch(p: TopicWatchPayload): TopicWatch {
  const c = camelizeKeys<Omit<TopicWatch, 'id' | 'semanticBoardId' | 'status'>>(p)
  return {
    ...c,
    id: String(p.id),
    semanticBoardId: String(p.semantic_board_id),
    // backend CHECK guarantees active|paused; narrow defensively.
    status: p.status === 'paused' ? 'paused' : 'active',
  }
}

function normalizeHit(p: TopicWatchHitPayload): TopicWatchHit {
  const c = camelizeKeys<TopicWatchHit>(p)
  return {
    ...c,
    id: String(p.id),
    watchId: String(p.watch_id),
    sectionId: String(p.section_id),
    reportId: String(p.report_id),
  }
}

/**
 * 关注标记 CRUD + 命中拉取。所有 HTTP 经 ApiClient；snake→camel 在 normalizer 完成；
 * 组件内只用 camelCase；数字 id 在此边界转字符串（§5.4）。
 */
export function useTopicWatchesApi() {
  /** POST /api/semantic-boards/:boardId/topic-watches — 创建关注（默认 active）。 */
  async function createWatch(boardId: number, label: string): Promise<ApiResponse<TopicWatch>> {
    const res = await apiClient.post<TopicWatchPayload>(`/semantic-boards/${boardId}/topic-watches`, { label })
    if (res.success && res.data) return mapApiResponse(res, normalizeWatch(res.data))
    return { success: false, error: res.error }
  }

  /** GET /api/semantic-boards/:boardId/topic-watches — 列出该 board 全部关注（含 paused）。 */
  async function listWatches(boardId: number): Promise<ApiResponse<TopicWatch[]>> {
    const res = await apiClient.get<TopicWatchPayload[]>(`/semantic-boards/${boardId}/topic-watches`)
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeWatch) }
    return { ...res, data: [] as TopicWatch[] }
  }

  /** PATCH /api/topic-watches/:id — 更新 label 或切换 status（active/paused）。 */
  async function updateWatch(id: string, params: { label?: string, status?: TopicWatchStatus }): Promise<ApiResponse<TopicWatch>> {
    const res = await apiClient.patch<TopicWatchPayload>(`/topic-watches/${id}`, params)
    if (res.success && res.data) return mapApiResponse(res, normalizeWatch(res.data))
    return { success: false, error: res.error }
  }

  /** DELETE /api/topic-watches/:id — 删除关注（连同其命中记录级联清理）。 */
  async function deleteWatch(id: string): Promise<ApiResponse<null>> {
    return apiClient.delete<null>(`/topic-watches/${id}`)
  }

  /** GET /api/daily-reports/:id/watch-hits — 该期日报被 active 关注命中的 section 列表。 */
  async function getWatchHits(reportId: number): Promise<ApiResponse<TopicWatchHit[]>> {
    const res = await apiClient.get<TopicWatchHitPayload[]>(`/daily-reports/${reportId}/watch-hits`)
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeHit) }
    return { ...res, data: [] as TopicWatchHit[] }
  }

  return {
    createWatch,
    listWatches,
    updateWatch,
    deleteWatch,
    getWatchHits,
  }
}
