/**
 * 订阅源发现 / 兴趣画像 类型定义（preference-vector-feed-discovery）
 *
 * API 响应为 snake_case，在 api 层 normalizer 转 camelCase，组件只用 camelCase。
 * 数字 id 在 API 边界转 string。
 */

/** 推荐卡片状态机：pending → accepted | dismissed */
export type RecommendationStatus = 'pending' | 'accepted' | 'dismissed'

/** 推荐来源：手动刷新 | 问答；二者共享幂等池与 dismiss 冷却池 */
export type RecommendationSource = 'manual_refresh' | 'qa'

/** 参数可选值字典条目（feed-param-options）。source 仅为 manual/scraped，绝不由 LLM 生成。 */
export interface RouteParamOption {
  value: string
  label: string
  source: string
}

/** 推荐卡片（GET /api/discovery/recommendations、POST /api/discovery/ask 的单条） */
export interface DiscoveryRecommendation {
  id: string
  routeId: string
  boardId: string | null
  /** 所属版块名；全局桶/无版块时后端返回空串，前端兜底「全局推荐」 */
  boardLabel: string
  source: RecommendationSource
  score: number
  llmReason: string
  status: RecommendationStatus
  routeNamespace: string
  routePath: string
  routeName: string
  routeExample: string
  /** true = 无必填参数，可一键订阅 */
  usableDirectly: boolean
  /** true = 有必填参数，需用户填写后验证订阅 */
  requiresParameters: boolean
  /** 目录自带的参数说明（原始 JSON 字符串，对象或数组），由 utils/routeParams 解析 */
  parameters: string
  /** 参数可选值字典（按 param_name 分组，只来自 manual/scraped 真实数据）；无字典数据为空对象 */
  paramOptions: Record<string, RouteParamOption[]>
  createdAt: string
}

/** POST /api/discovery/recommendations/refresh 的产出摘要 */
export interface RefreshSummary {
  candidates: number
  inserted: number
  skipped: number
  cooldownBlocked: number
}

/** GET /api/discovery/catalog/status 的目录统计 */
export interface CatalogStatus {
  total: number
  ok: number
  broken: number
  unknown: number
  gone: number
  embedded: number
}

/**
 * POST /api/discovery/catalog/sync 的产出摘要。
 * 注意：后端 CatalogSyncSummary 未加 json tag，键为 PascalCase。
 */
export interface CatalogSyncSummary {
  Inserted: number
  Updated: number
  Gone: number
  Total: number
  NewToEmbed: number
}

/** 接受推荐成功后后端返回的 Feed（取展示所需字段） */
export interface AcceptedFeed {
  id: string
  title: string
  url: string
}

/** 偏好来源：行为重算 | 问答种子 */
export type PreferenceSource = 'behavior' | 'seed'

/** GET /api/preference-profile 的单条画像 */
export interface PreferenceProfileItem {
  boardId: string | null
  /** 版块名；全局桶后端返回「全局」 */
  boardLabel: string
  source: PreferenceSource
  /** { 标签名: 权重 } top 列表 */
  tagWeights: Record<string, number>
  lastComputedAt: string | null
}

/**
 * POST /api/preference-profile/recompute 的产出摘要。
 * 注意：后端 RecomputeSummary 未加 json tag，键为 PascalCase。
 */
export interface RecomputeSummary {
  BoardsComputed: number
  TagsUsed: number
  ArticleCount: number
}
