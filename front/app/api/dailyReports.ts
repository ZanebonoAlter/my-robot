import { apiClient } from './client'
import type { ApiResponse } from '~/types'

export interface DailyReportHighlight {
  title: string
  reason: string
  tag_ids: number[]
}

export interface DailyReportThread {
  id: number
  report_id: number
  section_id: number
  title: string
  summary: string
  tag_ids: number[]
  confidence: number
  /** Thread 标题 ↔ 所属 section 标题的余弦贴合距离（observability System 3）。
   *  nil = 无信号；有值时为 0.0~2.0 的距离，越小越贴合（0.0 = 完美贴合）。
   *  embedding 字段绝不在此声明（后端 json:"-" 不外泄到前端）。 */
  fit_distance?: number
  related_article_ids: number[]
  created_at: string
}

export interface PersistentTopicBrief {
  id: number
  label: string
  status: string  // 'candidate' | 'active' | 'archived'
  /** Stable colour derived from the topic id by the backend; same topic → same colour across renders. */
  color: string
  hit_count?: number
  consecutive_hits: number
  can_activate: boolean
}

/** Full persistent topic row, as returned by the management API (merge/split/update). */
export interface PersistentTopic {
  id: number
  semantic_board_id: number
  label: string
  description: string
  status: string  // 'candidate' | 'active' | 'archived'
  first_seen_date: string
  last_seen_date: string
  hit_count: number
  consecutive_hits: number
}

/** Topic list item from GET /semantic-boards/:id/topics — adds a section count
 * and the stable colour, for the management UI. */
export interface BoardTopicListItem extends PersistentTopic {
  section_count: number
  color: string
  can_activate: boolean
}

export interface SectionTimelineNode {
  id: number
  report_id: number
  period_date: string
  cluster_label: string
  status: string  // emerging / continuing / split / merge / ending (dynamically derived)
  article_count: number
  thread_count: number
  image_url?: string
  imageUrl?: string
  /** Per-tag match-quality snapshot for timeline/lifecycle/lifeline sections.
   *  Mirrors `DailyReportSection.quality_breakdown`; null for historical
   *  sections (column added by the quality-scoring-observability change). */
  quality_breakdown?: DailyReportQualityEntry[] | null
  // Persistent topic assignment (optional — historical/unmatched sections may lack it).
  persistent_topic_id?: number
  topic_match_distance?: number
  topic_match_confidence?: string  // 'anchor_hit' | 'auto_new' | 'unmatched'
  persistent_topic?: PersistentTopicBrief
}

export interface SectionRelation {
  from_id: number
  to_id: number
  distance: number
  /** 'identity' = same persistent topic (solid line); 'similarity' = Hungarian match (dashed line). */
  relation_type?: string
}

// SectionLifecycleNode has the same shape as SectionTimelineNode
export type SectionLifecycleNode = SectionTimelineNode

/** Frozen per-tag match-quality lineage snapshot persisted on
 *  daily_report_sections.quality_breakdown. snake_case to match the rest of
 *  the daily-report API surface (the dailyReports client does not camelize).
 *  Historical sections return null for the whole array. */
export interface DailyReportQualityEntry {
  tag_id: number
  label: string
  match_reason: string
  score: number
  downgraded: boolean
}

export interface DailyReportSection {
  id: number
  cluster_index: number
  cluster_label: string
  cluster_tag_ids: number[]
  threads: DailyReportThread[]
  article_count: number
  best_tier: number
  avg_score: number
  /** Frozen match-quality lineage for this section's source tags, or null for
   *  historical sections generated before this field existed (DP-1 snake_case). */
  quality_breakdown?: DailyReportQualityEntry[] | null
  // Persistent topic assignment (optional; populated by the daily pipeline).
  persistent_topic_id?: number
  topic_match_distance?: number
  topic_match_confidence?: string
  persistent_topic?: PersistentTopicBrief
  /** 报告生成时的话题状态快照（active | candidate | null），用于阅读分区而非
   *  当前 topic.status。旧数据可能为 null；缺失时保守降级到"其他动态"。
   *  后端必须输出小写枚举值（active | candidate）；混合大小写将降级到"其他动态"。
   *  snake_case，与 daily-report API 约定一致。 */
  topic_status_at_report?: string | null
}

export interface DailyReport {
  id: number
  semantic_board_id: number
  period_date: string
  title: string
  summary: string
  status: string
  cluster_count: number
  article_count: number
  event_tag_count: number
  highlights: DailyReportHighlight[]
  dynamics: string
  sections: DailyReportSection[]
  created_at: string
}

export interface DailyReportListItem {
  id: number
  semantic_board_id: number
  period_date: string
  title: string
  summary: string
  status: string
  cluster_count: number
  article_count: number
  event_tag_count: number
  created_at: string
}

export function useDailyReportsApi() {
  async function generateDailyReport(params: { date: string; board_id?: number }) {
    return apiClient.post<{ job_id: string; status: string }>('/daily-reports/generate', params)
  }

  async function getBoardDailyReports(boardId: number, params?: { days?: number }): Promise<ApiResponse<{ reports: DailyReportListItem[] }>> {
    const query = params ? apiClient.buildQueryParams(params) : ''
    return apiClient.get(`/semantic-boards/${boardId}/daily-reports${query ? `?${query}` : ''}`)
  }

  async function getDailyReportDetail(id: number): Promise<ApiResponse<{ report: DailyReport }>> {
    return apiClient.get(`/daily-reports/${id}`)
  }

  async function getBoardSectionTimeline(boardId: number, days?: number): Promise<ApiResponse<{ sections: SectionTimelineNode[], relations: SectionRelation[] }>> {
    // days 传 0 表示"全部历史"（显式 ?days=0，后端 <=0 即不限天）；省略则走后端默认。
    const query = days != null ? `?days=${days}` : ''
    return apiClient.get(`/semantic-boards/${boardId}/section-timeline${query}`)
  }

  async function getSectionLifecycle(sectionId: number): Promise<ApiResponse<{ sections: SectionLifecycleNode[], relations: SectionRelation[] }>> {
    return apiClient.get(`/daily-reports/sections/${sectionId}/lifecycle`)
  }

  /** All sections of one persistent topic (no day limit), aggregated by topic id. Identity-key based — survives label drift. */
  async function getTopicLifeline(topicId: number): Promise<ApiResponse<{ sections: SectionTimelineNode[], relations: SectionRelation[] }>> {
    return apiClient.get(`/daily-reports/topics/${topicId}/lifeline`)
  }

  /** Reconstruct persistent topics from historical sections lacking a topic. Optional boardId scopes to one board. */
  async function backfillPersistentTopics(boardId?: number): Promise<ApiResponse<{ status: string }>> {
    const query = boardId ? `?board_id=${boardId}` : ''
    return apiClient.post(`/daily-reports/backfill-topics${query}`, {})
  }

  /** Rename and/or change status (active|archived) of a topic. Omit a field to leave it unchanged. */
  async function updateTopic(topicId: number, params: { label?: string; status?: 'active' | 'archived' }): Promise<ApiResponse<PersistentTopic>> {
    return apiClient.patch(`/daily-reports/topics/${topicId}`, params)
  }

  /** Hard-delete a topic. Sections keep their content; only the topic assignment is cleared. Irreversible. */
  async function deleteTopic(topicId: number): Promise<ApiResponse<null>> {
    return apiClient.delete(`/daily-reports/topics/${topicId}`)
  }

  /** List EVERY persistent topic on a board (active + candidate + archived + orphans) with section counts. */
  async function listBoardTopics(boardId: number): Promise<ApiResponse<{ topics: BoardTopicListItem[] }>> {
    return apiClient.get(`/semantic-boards/${boardId}/topics`)
  }

  /** Merge source topics into the target; sources are archived and their sections reassigned to the target. */
  async function mergeTopics(targetTopicId: number, sourceTopicIds: number[]): Promise<ApiResponse<PersistentTopic>> {
    return apiClient.post(`/daily-reports/topics/${targetTopicId}/merge`, { source_topic_ids: sourceTopicIds })
  }

  /** Carve sections out of a topic into a freshly created topic. */
  async function splitTopic(sourceTopicId: number, sectionIds: number[], label: string): Promise<ApiResponse<PersistentTopic>> {
    return apiClient.post(`/daily-reports/topics/${sourceTopicId}/split`, { section_ids: sectionIds, label })
  }

  return {
    generateDailyReport,
    getBoardDailyReports,
    getDailyReportDetail,
    getBoardSectionTimeline,
    getSectionLifecycle,
    getTopicLifeline,
    backfillPersistentTopics,
    updateTopic,
    deleteTopic,
    listBoardTopics,
    mergeTopics,
    splitTopic,
  }
}
