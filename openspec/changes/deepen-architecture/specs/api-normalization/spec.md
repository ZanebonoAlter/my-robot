## ADDED Requirements

### Requirement: 统一查询参数构建
系统 SHALL 在 `api/client.ts` 的 `buildQueryParams` 方法中自动拼接 `?` 前缀，调用者不再手动 `?${query}`。

#### Scenario: 调用者无需关心 `?`
- **WHEN** `apiClient.get<T>('/articles', { per_page: 20, page: 2 })`
- **THEN** 内部生成 `/api/articles?per_page=20&page=2`，调用者代码无 `?` 字符串

### Requirement: 共享 camelCase 转换
系统 SHALL 提供 `utils/api-helpers.ts` 中的 `camelizeKeys<T>(data: any): T` 通用函数，将 snake_case 对象键递归转为 camelCase。

#### Scenario: 转换 Article 响应
- **WHEN** `camelizeKeys<Article>({ article_id: 1, feed_id: 2, created_at: '...' })`
- **THEN** 返回 `{ articleId: 1, feedId: 2, createdAt: '...' }`

### Requirement: 泛型队列 API
`tagQueue` 和 `embeddingQueue` 的 API 模块 SHALL 合并为一个泛型工厂函数 `createQueueApi<T>(endpoint: string): QueueApi<T>`。

#### Scenario: 创建嵌入队列 API
- **WHEN** `const api = createQueueApi<EmbeddingTask>('/embedding-queue')`
- **THEN** `api.getStatus()` 返回 `QueueStatus`，`api.getTasks({ page: 1 })` 返回 `PaginatedData<EmbeddingTask>`

### Requirement: API 模块统一错误处理
所有 API 模块的 `response.success ? ... : ... as unknown` 模式 SHALL 替换为共享的 `unwrapResponse<T>(response)` 工具函数。

#### Scenario: 成功的响应
- **WHEN** `unwrapResponse<Category>(response)` 且 `response.success === true`
- **THEN** 返回 `response.data`

#### Scenario: 失败的响应
- **WHEN** `unwrapResponse<Category>(response)` 且 `response.success === false`
- **THEN** 抛出 `ApiError` 包含 `message` 和 `statusCode`，由调用方或被全局错误通知捕获
