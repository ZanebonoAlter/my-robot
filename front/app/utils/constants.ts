/**
 * 应用常量定义
 * 集中管理应用中的魔法数字、硬编码值等常量
 */

/**
 * 刷新相关常量
 */
export const REFRESH_POLLING_INTERVAL = 2000 // 轮询间隔（毫秒）
export const MAX_POLLING_TIME = 60000 // 最大轮询时间（毫秒）

/**
 * 侧边栏相关常量
 */
export const SIDEBAR_DEFAULT_WIDTH = 256 // 默认宽度（像素）
export const SIDEBAR_MIN_WIDTH = 200 // 最小宽度（像素）
export const SIDEBAR_MAX_WIDTH = 500 // 最大宽度（像素）

/**
 * 分类标识符
 */
export const CAT_FAVORITES = 'favorites'
export const CAT_UNCATEGORIZED = 'uncategorized'
export const CAT_TOPIC_GRAPH = 'topic-graph'
export const CAT_WATCHED_TAGS = 'watched-tags'

/**
 * 路由路径
 */
export const ROUTE_TOPICS = '/topics'

/**
 * 刷新消息类型
 */
export type RefreshMessageType = 'success' | 'error' | 'info'
