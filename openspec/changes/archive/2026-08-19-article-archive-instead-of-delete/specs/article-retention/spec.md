## ADDED Requirements

### Requirement: 超限文章归档而非删除
`CleanupOldArticles` SHALL 将超出 `MaxArticles` 的文章归档（`archived = true`）而非物理删除，文章行、ID 及全部文本字段（title/description/content/firecrawl_content/ai_content_summary/link/image_url/pub_date/author）MUST 保持可读。

#### Scenario: 超出上限的文章被归档
- **GIVEN** feed 的活跃文章数超过 MaxArticles=100
- **WHEN** CleanupOldArticles 执行
- **THEN** 最旧的超限文章 `archived = true`，文章行仍存在于 articles 表，原文字段不变

#### Scenario: favorite 文章免死
- **GIVEN** 某文章 favorite = true 且位于超限区
- **WHEN** CleanupOldArticles 执行
- **THEN** 该文章不被归档，非 favorite 的更新文章代替其进入归档候选

#### Scenario: 无上限 feed 不归档
- **WHEN** feed.MaxArticles = 0 或 9999
- **THEN** CleanupOldArticles 不归档任何文章

#### Scenario: 归档幂等
- **GIVEN** 文章 A 已处于 archived = true
- **WHEN** CleanupOldArticles 再次执行
- **THEN** A 不重复进入候选集，无额外写操作

### Requirement: 归档清除衍生数据、内容字段全保留
归档时系统 SHALL 删除该文章的 `article_topic_tags` 边与 `reading_behaviors` 记录，并将 `search_vector` 置 NULL；MUST NOT 清除或截断任何文本字段。

#### Scenario: 衍生数据清除
- **WHEN** 文章被归档
- **THEN** 其 article_topic_tags 行删除（孤儿 topic tag 走现有 CleanupOrphanedTags）、reading_behaviors 行删除、search_vector 为 NULL

#### Scenario: 归档文章不被全文搜索命中
- **GIVEN** 文章已归档
- **WHEN** 用户在 reader 搜索关键词
- **THEN** 该文章不出现在搜索结果中

### Requirement: 活跃窗口计数排除归档行
CleanupOldArticles 的数量统计与排序候选 SHALL 仅基于 `archived = false` 的文章，防止归档行侵蚀活跃窗口。

#### Scenario: 归档行不占用窗口计数
- **GIVEN** feed 有 100 篇 archived=false 与 500 篇 archived=true 文章，MaxArticles=100
- **WHEN** CleanupOldArticles 执行
- **THEN** 活跃计数为 100，不触发任何新归档

### Requirement: 归档文章对列表与统计默认不可见
reader 列表与统计查询 SHALL 默认过滤 `archived = false`；文章列表 API SHALL 支持 `archived=true` 查询参数显式返回归档集。

#### Scenario: 列表默认不含归档文章
- **GIVEN** feed 存在归档文章
- **WHEN** 用户请求文章列表（不带 archived 参数）
- **THEN** 归档文章不出现在结果中

#### Scenario: 统计口径为活跃数
- **GIVEN** 全库存在归档文章
- **WHEN** 请求全局统计（total/unread/favorite）或 feed 级统计（article_count/unread_count）
- **THEN** 计数仅统计 archived = false 的文章

#### Scenario: 显式查询归档集
- **WHEN** 请求文章列表带 `archived=true`
- **THEN** 返回归档文章

### Requirement: 按文章 ID 的读取豁免归档
按 ID 获取文章详情、RSS 刷新去重、日报引用反查 SHALL NOT 过滤归档文章。

#### Scenario: 日报线索可读归档文章原文
- **GIVEN** 日报 thread 的 related_article_ids 引用的文章已归档
- **WHEN** 按该文章 ID 请求详情
- **THEN** 返回完整文章（含原文字段）

#### Scenario: RSS 去重包含归档标题
- **GIVEN** 归档文章的标题与 RSS 条目相同
- **WHEN** feed 刷新
- **THEN** 该条目不重复入库
