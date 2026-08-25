import { apiClient } from './client'
import { camelizeKeys, mapApiResponse } from '~/utils/api-helpers'
import type { ApiResponse } from '~/types'

/** 关注标记状态：active 参与命中判定；paused 跳过当期判定。 */
export type TopicWatchStatus = 'active' | 'paused'

/** 关注类型：label = 话题语义关注（AI 单信号判定）；keyword = 关键字文本匹配（空格 AND / `|` OR，即时回扫）；
 *  keyword_topic = 关键字物化轨（当天命中文章聚合为固定名临时 section，零 AI、无持久话题）；
 *  sentence_topic = 一句话物化轨（向量检索当天辅助标签，物化为挂专属持久话题的 section）。 */
export type TopicWatchType = 'label' | 'keyword' | 'keyword_topic' | 'sentence_topic'

/** 关注标记（camelCase；数字 id 在 API 边界转字符串）。
 *  提示轨（label/keyword）与 persistent_topic 完全隔离；sentence_topic 持有
 *  专属持久话题（persistentTopicId，首次物化时建立，删除关注时确认归档）。 */
export interface TopicWatch {
  id: string
  semanticBoardId: string
  label: string
  /** sentence_topic 的检索句（embedding 输入）；空则回退 label。 */
  query?: string
  /** 关注类型；历史/缺省行为 label（后端列默认值，向后兼容）。 */
  type: TopicWatchType
  /** sentence_topic 专属持久话题 id（物化 section 归属它）；其他类型无。 */
  persistentTopicId?: string
  status: TopicWatchStatus
  createdAt: string
  updatedAt: string
}

/** 创建关注响应：keyword 类额外带即时回扫命中数（label 类无此字段）。 */
export interface CreateWatchResult extends TopicWatch {
  /** type=keyword 时后端同步回扫近 14 天写入的命中条数。 */
  instantHitCount?: number
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
  /** Enriched hit fields returned by the active-only report hit endpoint. */
  watchLabel?: string
  watchType?: TopicWatchType
}

/** Backend snake_case payload for a watch row. */
interface TopicWatchPayload {
  id: number
  semantic_board_id: number
  label: string
  query?: string
  type?: string
  persistent_topic_id?: number
  status: string
  created_at: string
  updated_at: string
}

/** 创建响应 payload：keyword 类额外带 instant_hit_count（snake_case）。 */
interface CreateWatchPayload extends TopicWatchPayload {
  instant_hit_count?: number
}

/** Backend snake_case payload for a hit row. */
interface TopicWatchHitPayload {
  id: number
  watch_id: number
  section_id: number
  report_id: number
  period_date: string
  reason: string
  watch_label?: string
  watch_type?: string
  watchLabel?: string
  watchType?: string
  label?: string
  type?: string
  watch?: { label?: string; type?: string }
}

function normalizeWatch(p: TopicWatchPayload): TopicWatch {
  const c = camelizeKeys<Omit<TopicWatch, 'id' | 'semanticBoardId' | 'status' | 'type'>>(p)
  return {
    ...c,
    id: String(p.id),
    semanticBoardId: String(p.semantic_board_id),
    // backend CHECK guarantees active|paused; narrow defensively.
    status: p.status === 'paused' ? 'paused' : 'active',
    // type 为新增可选字段：缺省（历史行 / 旧后端）按 label 处理，向后兼容。
    type: normalizeWatchType(p.type),
    ...(p.query ? { query: p.query } : {}),
    ...(p.persistent_topic_id ? { persistentTopicId: String(p.persistent_topic_id) } : {}),
  }
}

function normalizeWatchType(t?: string): TopicWatchType {
  if (t === 'keyword' || t === 'keyword_topic' || t === 'sentence_topic') return t
  return 'label'
}

function normalizeCreateResult(p: CreateWatchPayload): CreateWatchResult {
  const w = normalizeWatch(p)
  // instant_hit_count 仅 keyword 类返回；显式 undefined 保证 JSON 语义干净
  return p.instant_hit_count !== undefined && p.instant_hit_count !== null
    ? { ...w, instantHitCount: p.instant_hit_count }
    : w
}

function normalizeHit(p: TopicWatchHitPayload): TopicWatchHit {
  const c = camelizeKeys<TopicWatchHit>(p)
  const watchLabel = p.watchLabel ?? p.watch_label ?? p.watch?.label ?? p.label
  const watchType = p.watchType ?? p.watch_type ?? p.watch?.type ?? p.type
  return {
    ...c,
    id: String(p.id),
    watchId: String(p.watch_id),
    sectionId: String(p.section_id),
    reportId: String(p.report_id),
    ...(watchLabel ? { watchLabel } : {}),
    ...(watchType === 'keyword' || watchType === 'label' ? { watchType } : {}),
  }
}

/**
 * 关注标记 CRUD + 命中拉取。所有 HTTP 经 ApiClient；snake→camel 在 normalizer 完成；
 * 组件内只用 camelCase；数字 id 在此边界转字符串（§5.4）。
 */
export function useTopicWatchesApi() {
  /** POST /api/semantic-boards/:boardId/topic-watches — 创建关注（默认 active）。
   *  type 缺省不传该字段（后端默认 label，向后兼容）；keyword 类响应带即时回扫命中数；
   *  sentence_topic 类可带 query（检索句，缺省回退 label）。 */
  async function createWatch(boardId: number, label: string, type?: TopicWatchType, query?: string): Promise<ApiResponse<CreateWatchResult>> {
    const body: { label: string, type?: TopicWatchType, query?: string } = { label }
    if (type) body.type = type
    if (query) body.query = query
    const res = await apiClient.post<CreateWatchPayload>(`/semantic-boards/${boardId}/topic-watches`, body)
    if (res.success && res.data) return mapApiResponse(res, normalizeCreateResult(res.data))
    return { success: false, error: res.error }
  }

  /** GET /api/semantic-boards/:boardId/topic-watches — 列出该 board 全部关注（含 paused）。 */
  async function listWatches(boardId: number): Promise<ApiResponse<TopicWatch[]>> {
    const res = await apiClient.get<TopicWatchPayload[]>(`/semantic-boards/${boardId}/topic-watches`)
    if (res.success && res.data) return { ...res, data: res.data.map(normalizeWatch) }
    return { ...res, data: [] as TopicWatch[] }
  }

  /** PATCH /api/topic-watches/:id — 更新 label/query 或切换 status（active/paused）；
   *  更新 label/query 会使 sentence_topic 的向量缓存失效（下次日报生成时惰性补算）。 */
  async function updateWatch(id: string, params: { label?: string, query?: string, status?: TopicWatchStatus }): Promise<ApiResponse<TopicWatch>> {
    const res = await apiClient.patch<TopicWatchPayload>(`/topic-watches/${id}`, params)
    if (res.success && res.data) return mapApiResponse(res, normalizeWatch(res.data))
    return { success: false, error: res.error }
  }

  /** DELETE /api/topic-watches/:id — 删除关注（提示轨连同命中记录级联清理）。
   *  sentence_topic 类必须传 confirmArchiveTopic=true：后端会同步归档其专属
   *  持久话题（不确认则 400，错误信息含话题名）；keyword_topic 直接删、历史 section 保留。 */
  async function deleteWatch(id: string, confirmArchiveTopic?: boolean): Promise<ApiResponse<null>> {
    const query = confirmArchiveTopic ? '?confirm_archive_topic=true' : ''
    return apiClient.delete<null>(`/topic-watches/${id}${query}`)
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
