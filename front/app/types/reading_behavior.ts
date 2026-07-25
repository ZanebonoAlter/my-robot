/**
 * 阅读行为相关类型定义
 *
 * ⚠️ 这些类型直接映射 API 的 snake_case 响应。按约定，DTO 进入 Store/Feature
 * 前应通过 camelizeKeys() 转换为 camelCase（见 api-helpers.ts）。
 * 后续重构时可将这些类型改为 camelCase，在 API 层统一转换。
 */

export type ReadingEventType = 'open' | 'close' | 'scroll' | 'favorite' | 'unfavorite'

/**
 * 阅读行为事件
 */
export interface ReadingBehaviorEvent {
  article_id: number
  feed_id: number
  category_id?: number
  session_id: string
  event_type: ReadingEventType
  scroll_depth?: number
  reading_time?: number
}

/**
 * 阅读统计数据
 */
export interface ReadingStats {
  total_articles: number
  total_reading_time: number
  avg_reading_time: number
  avg_scroll_depth: number
  most_active_feed_id: number
  most_active_category: number
  /** @deprecated use total_articles / total_reading_time */
  read_ratio?: number
  fav_ratio?: number
}
