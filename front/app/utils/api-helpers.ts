/**
 * API 工具函数
 * camelizeKeys: 递归将 snake_case 对象键转为 camelCase
 * buildQueryString: 构建 URL 查询参数，自动处理空值
 */

/**
 * 将 snake_case 字符串转为 camelCase
 */
function snakeToCamel(str: string): string {
  return str.replace(/_([a-z])/g, (_, c) => c.toUpperCase())
}

/**
 * 递归将对象的所有 snake_case 键转为 camelCase
 */
export function camelizeKeys<T = unknown>(data: unknown): T {
  if (data === null || data === undefined) return data as T
  if (Array.isArray(data)) return data.map(camelizeKeys) as T
  if (typeof data === 'object' && !(data instanceof Date)) {
    const result: Record<string, unknown> = {}
    for (const [key, value] of Object.entries(data as Record<string, unknown>)) {
      result[snakeToCamel(key)] = camelizeKeys(value)
    }
    return result as T
  }
  return data as T
}

/**
 * 构建 URL 查询参数字符串，自动过滤 null/undefined
 * 调用者不需要手动加 `?` 前缀
 */
export function buildQueryString(params: Record<string, unknown>): string {
  const searchParams = new URLSearchParams()
  for (const [key, value] of Object.entries(params)) {
    if (value !== undefined && value !== null && value !== '') {
      searchParams.append(key, String(value))
    }
  }
  const qs = searchParams.toString()
  return qs ? `?${qs}` : ''
}

/**
 * API 响应错误
 */
export class ApiError extends Error {
  statusCode: number
  constructor(message: string, statusCode: number = 0) {
    super(message)
    this.name = 'ApiError'
    this.statusCode = statusCode
  }
}

/**
 * Type-safe API response data mapper.
 * Replaces `as unknown as ApiResponse<T>` pattern. Creates a properly typed response
 * with transformed data, preserving pagination/message when success.
 */
export function mapApiResponse<T>(response: { success: boolean; data?: unknown; error?: string; message?: string; pagination?: unknown }, data: T): import('~/types/api').ApiResponse<T> {
  if (response.success) {
    return { success: true, data, error: undefined, message: response.message, pagination: response.pagination as import('~/types/api').PaginationMeta | undefined }
  }
  return { success: false, error: response.error || '请求失败', data: undefined }
}

/**
 * 统一的 API 响应解包
 * 成功返回 data，失败抛出 ApiError
 */
export function unwrapResponse<T>(response: { success: boolean; data?: T; error?: string; message?: string }): T {
  if (response.success && response.data !== undefined) {
    return response.data
  }
  throw new ApiError(response.error || response.message || '请求失败')
}
