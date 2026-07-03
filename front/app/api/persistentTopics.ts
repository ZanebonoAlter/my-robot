import { apiClient } from './client'
import { camelizeKeys, mapApiResponse } from '~/utils/api-helpers'
import type { ApiResponse } from '~/types'

/**
 * 手动建泳道编排态 API（切片③）。
 *
 * - 候选端点 GET .../compose-candidates 回传带 embedding 的 section，前端实时算
 *   聚合锚点 / 离群（见 composeReport.ts），不在后端逐次勾选往返。
 * - 保存走切片①的 POST .../persistent-topics/manual。
 *
 * API 边界归一：snake_case → camelCase 在本层完成，数字 id 转字符串（与
 * topicWatches 约定一致）。embedding 数组原样透传（number[]）。
 */

/** 候选 section 现有归属的轻量话题摘要（编排态只需 id + label）。 */
export interface CandidateTopicBrief {
  id: string
  label: string
}

/** 编排态候选 section（embedding 已展开为 number[]，供前端实时计算）。 */
export interface ComposeCandidate {
  id: string
  reportId?: string
  periodDate: string
  clusterLabel: string
  embedding: number[]
  persistentTopicId?: string
  topicMatchConfidence?: string
  persistentTopic?: CandidateTopicBrief
}

/** GET .../compose-candidates 的归一化响应。 */
export interface ComposeCandidatesData {
  sections: ComposeCandidate[]
  matchThreshold: number
}

/** 手动建泳道保存后返回的新话题。 */
export interface ManualTopicResult {
  id: string
  label: string
  status: string
  source: string
}

/** createManualLane 的归一化响应。skipped 为无向量被跳过的 section id。 */
export interface ManualLaneResult {
  topic: ManualTopicResult
  skipped: string[]
}

/** POST .../embed-query 的归一化响应。query 文本→embedding 向量（与 section 同模型）。 */
export interface EmbedQueryData {
  embedding: number[]
}

// ── backend snake_case payloads ─────────────────────────────────────────────

interface ComposeCandidatePayload {
  id: number
  report_id: number
  period_date: string
  cluster_label: string
  embedding: number[]
  persistent_topic_id?: number
  topic_match_confidence?: string
  persistent_topic?: { id: number, label: string, status: string, color: string }
}

interface ComposeCandidatesPayload {
  sections: ComposeCandidatePayload[]
  match_threshold: number
}

interface ManualTopicPayload {
  id: number
  label: string
  status: string
  source: string
}

interface ManualLanePayload {
  topic: ManualTopicPayload
  skipped: number[]
}

interface EmbedQueryPayload {
  embedding: number[]
}

// ── normalizers ─────────────────────────────────────────────────────────────

function normalizeCandidate(p: ComposeCandidatePayload): ComposeCandidate {
  const out: ComposeCandidate = {
    id: String(p.id),
    reportId: String(p.report_id),
    periodDate: p.period_date,
    clusterLabel: p.cluster_label,
    embedding: p.embedding,
  }
  if (p.persistent_topic_id != null) out.persistentTopicId = String(p.persistent_topic_id)
  if (p.topic_match_confidence) out.topicMatchConfidence = p.topic_match_confidence
  if (p.persistent_topic) out.persistentTopic = { id: String(p.persistent_topic.id), label: p.persistent_topic.label }
  return out
}

function normalizeManualLane(p: ManualLanePayload): ManualLaneResult {
  return {
    topic: {
      id: String(p.topic.id),
      label: p.topic.label,
      status: p.topic.status,
      source: p.topic.source,
    },
    skipped: (p.skipped ?? []).map(id => String(id)),
  }
}

// ── API ─────────────────────────────────────────────────────────────────────

export function usePersistentTopicsApi() {
  /** GET /api/semantic-boards/:boardId/persistent-topics/compose-candidates?days=N */
  async function getComposeCandidates(boardId: number | string, days: number): Promise<ApiResponse<ComposeCandidatesData>> {
    const query = days != null ? `?days=${days}` : ''
    const res = await apiClient.get<ComposeCandidatesPayload>(`/semantic-boards/${boardId}/persistent-topics/compose-candidates${query}`)
    if (!res.success || !res.data) return { success: false, error: res.error || '加载候选失败' }
    const data: ComposeCandidatesData = {
      sections: (res.data.sections ?? []).map(normalizeCandidate),
      matchThreshold: res.data.match_threshold,
    }
    return mapApiResponse(res, data)
  }

  /**
   * POST /api/semantic-boards/:boardId/persistent-topics/manual
   * sectionIds 来自 ComposeCandidate.id（字符串），请求体转回数字（后端收 []uint）。
   */
  async function createManualLane(
    boardId: number | string,
    label: string,
    sectionIds: string[],
  ): Promise<ApiResponse<ManualLaneResult>> {
    const res = await apiClient.post<ManualLanePayload>(
      `/semantic-boards/${boardId}/persistent-topics/manual`,
      { label, section_ids: sectionIds.map(id => Number(id)) },
    )
    if (!res.success || !res.data) return { success: false, error: res.error || '保存失败', message: res.message }
    return mapApiResponse(res, normalizeManualLane(camelizeKeys<ManualLanePayload>(res.data)))
  }

  /**
   * POST /api/semantic-boards/:boardId/persistent-topics/embed-query
   * 文本 → embedding（与 section 同模型），供编排态候选池语义排序（task 3.12）。
   * 失败返回 success=false，调用方负责降级（回退默认序）。
   */
  async function embedQuery(
    boardId: number | string,
    query: string,
  ): Promise<ApiResponse<EmbedQueryData>> {
    const res = await apiClient.post<EmbedQueryPayload>(
      `/semantic-boards/${boardId}/persistent-topics/embed-query`,
      { query },
    )
    if (!res.success || !res.data) return { success: false, error: res.error || '搜索向量生成失败' }
    return mapApiResponse(res, { embedding: res.data.embedding ?? [] })
  }

  return { getComposeCandidates, createManualLane, embedQuery }
}
