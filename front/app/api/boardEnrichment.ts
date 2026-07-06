import { apiClient } from './client'
import type { ApiResponse } from '~/types'

/**
 * 数据增强（data-enrichment）API client。
 *
 * 三表认知闭环 + 板块数据源绑定，对应后端 P1-P4 已实现的 14 条 REST endpoint。
 * 数据契约设计为侦探墙（TopicDetectiveWall）后续重构可直接复用：
 *  - ContextRow / ResultSummaryRow / ReviewRow / DataSourceRow 顶层行类型稳定；
 *  - sectors / tool_calls / input_snapshot 这类 LLM 产物保留宽口子（unknown / 联合），
 *    侦探墙渲染时再按需 narrow。
 *
 * 路由分两个维度：
 *  - Topic 维度（表 1/2/3）：/persistent-topics/:topicId/enrichment/...
 *  - Board 维度（数据源绑定）：/semantic-boards/:id/data-sources
 *
 * 注意：后端 getResult / applyReview / updateReviewDeviation 虽然实现里只用 :id，
 * 但路由前缀含 :topicId，URL 仍须带上（后端解析忽略），所以函数签名是 (topicId, id)。
 */

// ── Shared scalars ──────────────────────────────────────────────────────────

export type ContextGranularity = 'week' | 'month' | 'year' | 'all'

export type EnrichmentSource = 'manual' | 'llm_assisted' | string

// ── Table 1: topic_lifeline_context ─────────────────────────────────────────

/** 单条分层新闻汇总上下文（week/month/year/all 之一）。 */
export interface ContextRow {
  id: number
  persistent_topic_id?: number
  granularity: ContextGranularity
  content: string
  /** 汇总截止日（时效判断 + 检查自愈依据）。 */
  as_of_date: string
  source: EnrichmentSource
  created_at?: string
  updated_at?: string
}

// ── Table 2: topic_enrichment_result ────────────────────────────────────────

/** 分析员结论中的一个产业切片。LLM 产物，字段保留宽口子。 */
export interface ResultSector {
  sector?: string
  evolution_role?: string
  current_signal?: string
  vs_history?: string
  judgment?: string
  confidence?: number
  [key: string]: unknown
}

/** 表2 列表项（slim 版，后端 listResults 返回）。 */
export interface ResultSummaryRow {
  id: number
  evolution_assessment: string
  sectors: ResultSector[] | null
  tool_calls_count: number
  session_id: string
  created_at: string
}

/** 表2 详情（后端 getResult 返回，含 tool_calls / input_snapshot / causal_chain）。 */
export interface ResultDetailRow {
  id: number
  persistent_topic_id?: number
  evolution_assessment: string
  sectors: ResultSector[] | null
  causal_chain: string | null
  /** 工具调用记录（名/参数/返回摘要/耗时），LLM 产物，结构未冻结。 */
  tool_calls: unknown
  /** 编排元数据（读了哪些 context 层 + as_of + section 范围 + 引用 review id）。 */
  input_snapshot: unknown
  session_id: string
  created_at: string
}

/** POST .../results/trigger 的响应。 */
export interface TriggerEnrichmentResponse {
  result: {
    id: number
    evolution_assessment: string
    sectors: ResultSector[] | null
    causal_chain: string | null
    tool_calls_count: number
    session_id: string
    created_at: string
  }
  review_generated: boolean
}

// ── Table 3: topic_enrichment_review ────────────────────────────────────────

/** 两次 result 之间的认知增量（反思）。 */
export interface ReviewRow {
  id: number
  persistent_topic_id?: number
  prev_result_id: number | null
  curr_result_id: number
  deviation_summary: string
  affected_context: ContextGranularity | null
  confidence: number | null
  applied: boolean
  source: EnrichmentSource
  created_at: string
  updated_at?: string
}

// ── Board data sources ──────────────────────────────────────────────────────

/** 板块与数据源的绑定行。 */
export interface DataSourceRow {
  id: number
  semantic_board_id: number
  source_type: string
  /** 板块级参数，schema 由 source_type 决定。 */
  config: Record<string, unknown>
  enabled: boolean
  created_at?: string
  updated_at?: string
}

// ── Request body types ──────────────────────────────────────────────────────

export interface CreateReviewBody {
  curr_result_id: number
  deviation_summary: string
  prev_result_id?: number
}

export interface UpsertDataSourceBody {
  source_type: string
  config?: Record<string, unknown>
  enabled?: boolean
}

// ── API factory ─────────────────────────────────────────────────────────────

export function useBoardEnrichmentApi() {
  // ── Table 1: contexts ───────────────────────────────────────────────────
  async function listContexts(topicId: number): Promise<ApiResponse<ContextRow[]>> {
    return apiClient.get(`/persistent-topics/${topicId}/enrichment/contexts`)
  }

  async function getContext(topicId: number, granularity: ContextGranularity): Promise<ApiResponse<ContextRow>> {
    return apiClient.get(`/persistent-topics/${topicId}/enrichment/contexts/${granularity}`)
  }

  async function updateContext(
    topicId: number,
    granularity: ContextGranularity,
    body: { content: string },
  ): Promise<ApiResponse<ContextRow>> {
    return apiClient.put(`/persistent-topics/${topicId}/enrichment/contexts/${granularity}`, body)
  }

  async function regenerateContext(
    topicId: number,
    granularity: ContextGranularity,
  ): Promise<ApiResponse<ContextRow>> {
    return apiClient.post(`/persistent-topics/${topicId}/enrichment/contexts/${granularity}/regenerate`)
  }

  // ── Table 2: results ────────────────────────────────────────────────────
  async function listResults(topicId: number): Promise<ApiResponse<ResultSummaryRow[]>> {
    return apiClient.get(`/persistent-topics/${topicId}/enrichment/results`)
  }

  async function getResult(topicId: number, id: number): Promise<ApiResponse<ResultDetailRow>> {
    return apiClient.get(`/persistent-topics/${topicId}/enrichment/results/${id}`)
  }

  async function triggerEnrichment(topicId: number): Promise<ApiResponse<TriggerEnrichmentResponse>> {
    return apiClient.post(`/persistent-topics/${topicId}/enrichment/results/trigger`)
  }

  // ── Table 3: reviews ────────────────────────────────────────────────────
  async function listReviews(topicId: number): Promise<ApiResponse<ReviewRow[]>> {
    return apiClient.get(`/persistent-topics/${topicId}/enrichment/reviews`)
  }

  async function createReview(topicId: number, body: CreateReviewBody): Promise<ApiResponse<ReviewRow>> {
    return apiClient.post(`/persistent-topics/${topicId}/enrichment/reviews`, body)
  }

  async function updateReviewDeviation(
    topicId: number,
    id: number,
    body: { deviation_summary: string },
  ): Promise<ApiResponse<ReviewRow>> {
    return apiClient.put(`/persistent-topics/${topicId}/enrichment/reviews/${id}`, body)
  }

  async function applyReview(topicId: number, id: number): Promise<ApiResponse<ReviewRow>> {
    return apiClient.post(`/persistent-topics/${topicId}/enrichment/reviews/${id}/apply`)
  }

  // ── Board data sources ──────────────────────────────────────────────────
  async function listDataSources(boardId: number): Promise<ApiResponse<DataSourceRow[]>> {
    return apiClient.get(`/semantic-boards/${boardId}/data-sources`)
  }

  async function upsertDataSource(boardId: number, body: UpsertDataSourceBody): Promise<ApiResponse<DataSourceRow>> {
    return apiClient.put(`/semantic-boards/${boardId}/data-sources`, body)
  }

  async function deleteDataSource(boardId: number, sourceType: string): Promise<ApiResponse<{ deleted: boolean }>> {
    return apiClient.delete(`/semantic-boards/${boardId}/data-sources/${sourceType}`)
  }

  return {
    listContexts,
    getContext,
    updateContext,
    regenerateContext,
    listResults,
    getResult,
    triggerEnrichment,
    listReviews,
    createReview,
    updateReviewDeviation,
    applyReview,
    listDataSources,
    upsertDataSource,
    deleteDataSource,
  }
}
