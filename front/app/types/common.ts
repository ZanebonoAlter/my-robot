/**
 * 常用类型定义
 */

/**
 * 排序选项
 */
export type SortOption = 'latest' | 'popular' | 'unread'

/**
 * 筛选选项
 */
export type FilterOption = 'all' | 'unread' | 'favorites'

/**
 * 筛选状态
 */
export interface FilterState {
  sort: SortOption
  filter: FilterOption
  category: string | null
  search: string
}
