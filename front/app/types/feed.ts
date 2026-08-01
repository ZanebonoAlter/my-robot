/**
 * Feed-related type definitions.
 */

/**
 * Response shape returned by rss2json.com used by server/api/fetch-feed.post.ts.
 */
export interface FeedResponse {
  status: 'ok' | 'error'
  feed?: {
    url?: string
    title?: string
    link?: string
    author?: string
    description?: string
    image?: string
  }
  items?: Array<{
    title?: string
    pubDate?: string
    link?: string
    guid?: string
    author?: string
    thumbnail?: string
    description?: string
    content?: string
    enclosure?: unknown
    categories?: string[]
  }>
  message?: string
}

/**
 * RSS feed data model.
 */
export interface RssFeed {
  id: string
  title: string
  description: string
  url: string
  category: string
  icon?: string
  icon_source?: 'auto' | 'custom' | 'fallback'
  color?: string
  lastUpdated: string
  articleCount: number
  unreadCount?: number
  maxArticles?: number
  refreshInterval?: number
  refreshStatus?: 'idle' | 'refreshing' | 'success' | 'error'
  refreshError?: string
  lastRefreshAt?: string
  aiSummaryEnabled?: boolean
  articleSummaryEnabled?: boolean
  completionOnRefresh?: boolean
  maxCompletionRetries?: number
  firecrawlEnabled?: boolean
  taggingEnabled?: boolean
}

/**
 * Payload for creating a feed.
 */
export interface CreateFeedData {
  url: string
  category_id?: number
  title?: string
  description?: string
  icon?: string
  color?: string
}

/**
 * Payload for updating a feed.
 */
export interface UpdateFeedData {
  url?: string
  category_id?: number | null
  title?: string
  description?: string
  icon?: string
  color?: string
  max_articles?: number
  refresh_interval?: number
  refresh_status?: string
  refresh_error?: string
  last_refresh_at?: string
  ai_summary_enabled?: boolean
  article_summary_enabled?: boolean
  completion_on_refresh?: boolean
  max_completion_retries?: number
  firecrawl_enabled?: boolean
  tagging_enabled?: boolean
}
