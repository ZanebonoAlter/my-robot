## ADDED Requirements

### Requirement: 状态变更必须经 API 持久化
所有 Store 中对服务端数据的变更 SHALL 通过调用 API module 方法执行，不允许仅修改内存中的 ref 而不调 API。

#### Scenario: 标记文章已读
- **WHEN** 组件调用 `useArticlesStore().markAsRead(articleId)`
- **THEN** `useArticlesStore` MUST 直接调用 `useArticlesApi().updateArticle(...)` 持久化，不得仅设 `article.read = true`，也不得绕到 `apiStore` 执行文章 mutation

#### Scenario: API 调用失败时回滚乐观更新
- **WHEN** 标记已读的 API 调用返回错误
- **THEN** UI 已乐观更新为已读状态 MUST 回滚为未读，并弹出错误通知

### Requirement: 无 API 操作的 Computed 属性不受限制
仅从已有数据派生的 computed 属性（如 `filteredArticles`、`unreadCount`、`categorizedFeeds`）SHALL 保持现状，不做改动。

#### Scenario: 过滤未读文章
- **WHEN** `filter` 选项变更
- **THEN** `filteredArticles` 从 `apiStore.articles` 重新计算，不触发任何 API 调用

### Requirement: 消除 allFeeds 双存储
`apiStore.allFeeds` SHALL 合并到 `apiStore.feeds` 中，由消费方根据需要自行缓存或过滤。

#### Scenario: 侧栏读取全部 feeds
- **WHEN** 侧栏需要不带分页限制的 feeds
- **THEN** 使用一个 computed 属性或独立 fetch，而非维护 `allFeeds` 平行数组

### Requirement: Store 之间不得形成循环知识
功能 Store SHALL 只通过小而明确的 Interface 协作，不得互相动态 import 并读写对方内部集合。

#### Scenario: 文章变为已读后更新 feed 未读数
- **WHEN** `useArticlesStore().markAsRead(articleId)` 成功
- **THEN** 如需同步 feed 未读数，MUST 调用 `useFeedsStore()` 暴露的 `adjustUnreadCount` / `clearUnreadCounts` 等小 Interface，而不是直接遍历 `apiStore.feeds`

#### Scenario: 应用初始化
- **WHEN** 应用启动需要同时加载 categories、feeds、articles
- **THEN** 初始化编排 SHOULD 在 app/page 层完成，不得由 `apiStore.initialize()` 动态 import `articlesStore` 造成 Store 间循环依赖
