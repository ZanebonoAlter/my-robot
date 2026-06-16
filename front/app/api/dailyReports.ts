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
  related_article_ids: number[]
  created_at: string
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
}

export interface SectionRelation {
  from_id: number
  to_id: number
  distance: number
}

// SectionLifecycleNode has the same shape as SectionTimelineNode
export type SectionLifecycleNode = SectionTimelineNode

export interface DailyReportSection {
  id: number
  cluster_index: number
  cluster_label: string
  cluster_tag_ids: number[]
  threads: DailyReportThread[]
  article_count: number
  best_tier: number
  avg_score: number
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
    const query = days ? `?days=${days}` : ''
    return apiClient.get(`/semantic-boards/${boardId}/section-timeline${query}`)
  }

  async function getSectionLifecycle(sectionId: number): Promise<ApiResponse<{ sections: SectionLifecycleNode[], relations: SectionRelation[] }>> {
    return apiClient.get(`/daily-reports/sections/${sectionId}/lifecycle`)
  }

  return {
    generateDailyReport,
    getBoardDailyReports,
    getDailyReportDetail,
    getBoardSectionTimeline,
    getSectionLifecycle,
  }
}
