## ADDED Requirements

### Requirement: 统一查询参数构建
系统 SHALL 只保留一个权威 query builder，自动拼接 `?` 前缀并过滤空值，调用者不再手动 `?${query}`。

#### Scenario: 调用者无需关心 `?`
- **WHEN** API module 使用统一 query builder 构建 `{ per_page: 20, page: 2 }`
- **THEN** 生成 `?per_page=20&page=2`，调用者代码无手写 `?` 拼接

#### Scenario: API client 复用 query builder
- **WHEN** `apiClient.buildQueryParams(...)` 仍作为兼容方法存在
- **THEN** 其内部 MUST 委托同一个共享 query builder，不得维护第二套 `URLSearchParams` 逻辑

### Requirement: 共享 camelCase 转换
系统 SHALL 提供共享 normalizer，将 snake_case 对象键递归转为 camelCase；跨 feature 复用的数据 normalizer SHALL 位于 `api/normalizers/` 或 `types/` 等底层目录，不得放在某个 feature 内部。

#### Scenario: 转换 Article 响应
- **WHEN** `camelizeKeys<Article>({ article_id: 1, feed_id: 2, created_at: '...' })`
- **THEN** 返回 `{ articleId: 1, feedId: 2, createdAt: '...' }`

### Requirement: 泛型队列 API
`tagQueue` 和 `embeddingQueue` 的 API 模块 SHALL 合并为一个泛型工厂函数 `createQueueApi<T>(endpoint: string): QueueApi<T>`。

#### Scenario: 创建嵌入队列 API
- **WHEN** `const api = createQueueApi<EmbeddingTask>('/embedding-queue')`
- **THEN** `api.getStatus()` 返回 `QueueStatus`，`api.getTasks({ page: 1 })` 返回 `PaginatedData<EmbeddingTask>`

#### Scenario: 队列 API 构建查询参数
- **WHEN** `createQueueApi().getTasks({ status: 'failed', limit: 20 })`
- **THEN** MUST 复用统一 query builder，不得在 `createQueueApi` 内手写 `new URLSearchParams()`

### Requirement: API 模块统一错误处理
所有 API 模块的 `response.success ? ... : ... as unknown` 模式 SHALL 替换为共享的 `unwrapResponse<T>(response)` 工具函数。

#### Scenario: 成功的响应
- **WHEN** `unwrapResponse<Category>(response)` 且 `response.success === true`
- **THEN** 返回 `response.data`

#### Scenario: 失败的响应
- **WHEN** `unwrapResponse<Category>(response)` 且 `response.success === false`
- **THEN** 抛出 `ApiError` 包含 `message` 和 `statusCode`，由调用方或被全局错误通知捕获

### Requirement: Feature 不得依赖其他 feature 的 normalizer
跨 feature 消费同一后端 DTO 时 SHALL 依赖底层 normalizer，而不是 import 其他 feature 内部工具。

#### Scenario: TagsPage 预览文章
- **WHEN** `features/tags` 需要将 Article API payload 转成前端 Article 类型
- **THEN** MUST import 共享 `normalizeArticle`，不得 import `features/articles/utils/normalizeArticle`

#### Scenario: TopicGraph 预览文章
- **WHEN** `features/topic-graph` 需要 normalize Article payload
- **THEN** MUST 使用同一共享 normalizer，以保持转换规则单一
