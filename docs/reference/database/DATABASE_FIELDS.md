# 数据库字段说明文档

本文档详细说明 Syntopica 项目中所有数据库表（**43 张业务表** + 废弃表 + 框架表）的字段用途、约束与索引。

> **真相源 = 代码**：`backend-go/internal/models/*.go`（GORM struct + gorm tag）+ `internal/dataenrichment/repository/models.go` + `internal/topicgraph/repository/daily_report_models.go` + `internal/platform/tracing/model.go` + `internal/platform/database/postgres_migrations.go`（1343 行，承载 gorm tag 表达不了的向量维度/复合索引/CHECK/唯一索引/触发器）。

---

## 阅读约定（全局事实）

1. **表名**：GORM 未自定义 `NamingStrategy`，默认蛇形 + 复数；非默认表名由 struct 的 `TableName()` 显式指定。本文档每张表的真实表名以迁移 SQL 与 struct 为准。
2. **外键（FK）几乎不存在于 DB 层**：`gorm.Config` 设了 `DisableForeignKeyConstraintWhenMigrating: true`，且迁移 `20260601_0001` 主动 drop 了历史上全部 `fk_*`。**全库唯一真实 DB FK** 是 `topic_tags_merged_into_id_fkey`（`merged_into_id → topic_tags(id) ON DELETE CASCADE`）。本文档中凡是写「关联 / FK」的地方，除特别注明外均为 GORM 逻辑关联，**不**对应 DB 级外键约束。
3. **向量列维度运行时决定**：除 `topic_tag_embeddings.embedding` 由迁移固定为 `vector(4096)` 外，其余向量列 gorm tag 仅声明 `type:vector`，实际维度由运行时 `embedding_config.embedding_dimension` / `ensureXxxEmbeddingDimension` 设置；HNSW 索引仅当维度 ≤ 2000 时创建。
4. **字段表列含义**：「类型」= DB 列类型；「约束/默认/索引」= NOT NULL / DEFAULT / 索引 / 唯一 / CHECK 等；「用途」= 业务含义。`string` 无 `size` 时 GORM 默认 256。
5. **枚举**：除非特别注明「+ CHECK」，枚举取值均为代码层约定，DB 层不强制。

---

## 完整表清单（43 张业务表 + 5 张废弃表 + 1 张框架表）

### 业务表（43 张，代码权威）

| 表名 | 说明 | 对应模型 | 域 |
| ------ | ------ | ---------- | ------ |
| `categories` | 分类 | `models.Category` | 内容 |
| `feeds` | 订阅源 | `models.Feed` | 内容 |
| `articles` | 文章 | `models.Article` | 内容 |
| `scheduler_tasks` | 调度任务状态 | `models.SchedulerTask` | 调度 |
| `ai_settings` | AI 配置（键值对） | `models.AISettings` | 调度 |
| `ai_providers` | AI 供应商 | `models.AIProvider` | AI 路由 |
| `ai_routes` | AI 路由 | `models.AIRoute` | AI 路由 |
| `ai_route_providers` | AI 路由-供应商绑定 | `models.AIRouteProvider` | AI 路由 |
| `ai_call_logs` | AI 调用日志 | `models.AICallLog` | AI 路由 |
| `topic_tags` | 主题标签主表 | `models.TopicTag` | 主题标签 |
| `topic_tag_embeddings` | 主题标签向量 | `models.TopicTagEmbedding` | 主题标签 |
| `topic_tag_analyses` | 主题分析快照 | `models.TopicTagAnalysis` | 主题标签 |
| `topic_analysis_cursors` | 主题分析游标 | `models.TopicAnalysisCursor` | 主题标签 |
| `article_topic_tags` | 文章-主题关联 | `models.ArticleTopicTag` | 主题标签 |
| `topic_tag_relations` | 标签层级关系 | `models.TopicTagRelation` | 主题标签 |
| `tag_merge_suggestions` | 标签合并建议 | `models.TagMergeSuggestion` | 主题标签 |
| `semantic_labels` | 语义标签统一表（辅助标签 + SemanticBoard） | `models.SemanticLabel` | 语义标签/板块 |
| `topic_tag_semantic_labels` | tag-辅助标签关联（中间表） | `models.TopicTagSemanticLabel` | 语义标签/板块 |
| `topic_tag_board_labels` | tag-SemanticBoard 匹配结果（中间表） | `models.TopicTagBoardLabel` | 语义标签/板块 |
| `board_composition` | board 构成（中间表） | `models.BoardComposition` | 语义标签/板块 |
| `board_upgrade_suggestions` | 板块升级建议 | `models.BoardUpgradeSuggestion` | 语义标签/板块 |
| `embedding_config` | 向量配置（键值对） | `models.EmbeddingConfig` | 向量 |
| `embedding_queues` | 向量生成队列 | `models.EmbeddingQueue` | 向量 |
| `merge_reembedding_queues` | 合并后重算向量队列 | `models.MergeReembeddingQueue` | 向量 |
| `firecrawl_jobs` | Firecrawl 抓取任务 | `models.FirecrawlJob` | 任务队列 |
| `tag_jobs` | 标签任务 | `models.TagJob` | 任务队列 |
| `narrative_summaries` | 叙事摘要 | `models.NarrativeSummary` | 叙事 |
| `narrative_boards` | 叙事板块 | `models.NarrativeBoard` | 叙事 |
| `board_daily_reports` | 板块日报主表 | `topicgraph.BoardDailyReport` | 日报/持久话题/Watch |
| `daily_report_sections` | 日报分区 | `topicgraph.DailyReportSection` | 日报/持久话题/Watch |
| `daily_report_threads` | 日报叙事线程 | `topicgraph.DailyReportThread` | 日报/持久话题/Watch |
| `daily_report_section_relations` | 跨日分区关系 | `topicgraph.SectionRelation` | 日报/持久话题/Watch |
| `board_persistent_topics` | 板块持久叙事话题 | `topicgraph.BoardPersistentTopic` | 日报/持久话题/Watch |
| `board_topic_watches` | 用户声明的话题 Watch 标签 | `topicgraph.BoardTopicWatch` | 日报/持久话题/Watch |
| `topic_watch_hits` | Watch 命中记录 | `topicgraph.TopicWatchHit` | 日报/持久话题/Watch |
| `board_data_sources` | 板块数据源绑定 | `dataenrichment.BoardDataSource` | 数据增强 |
| `topic_lifeline_context` | 话题分层新闻汇总上下文（循环 A） | `dataenrichment.TopicLifelineContext` | 数据增强 |
| `topic_enrichment_result` | 数据增强结果快照（不可变） | `dataenrichment.TopicEnrichmentResult` | 数据增强 |
| `topic_enrichment_review` | 数据增强认知演进反思 | `dataenrichment.TopicEnrichmentReview` | 数据增强 |
| `stock_debate_result` | FinGenius 个股辩论结果 | `dataenrichment.StockDebateResult` | 数据增强 |
| `reading_behaviors` | 阅读行为 | `models.ReadingBehavior` | 用户行为 |
| `user_preferences` | 用户偏好 | `models.UserPreference` | 用户行为 |
| `otel_spans` | OpenTelemetry 链路追踪 | `tracing.OtelSpan` | 追踪 |

### 废弃表（5 张，无对应 model，保留标注）

| 表名 | 说明 |
| ------ | ------ |
| `ai_summaries` | 旧版 Feed 级 AI 批量摘要（已废弃） |
| `ai_summary_feeds` | AI 摘要-Feed 关联（已废弃） |
| `ai_summary_topics` | AI 摘要-主题关联（已废弃） |
| `topic_analysis_jobs` | 主题分析任务队列（已废弃，无 migrator 注册） |
| `digest_configs` | Digest 推送配置（预留，已废弃） |

### 框架表

| 表名 | 说明 |
| ------ | ------ |
| `schema_migrations` | 迁移版本追踪（框架管理，不计入业务表） |

---

## §1 内容域

### 1.1 articles（文章表）

存储 RSS 文章的核心数据。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `feed_id` | INTEGER | NOT NULL; index | 所属订阅源 ID（逻辑关联 `feeds.id`，无 DB FK） |
| `title` | VARCHAR(500) | NOT NULL | 文章标题 |
| `description` | TEXT | — | 文章描述（参与全文检索，权重 B） |
| `content` | TEXT | — | RSS 原始内容（HTML 片段） |
| `link` | VARCHAR(1000) | — | 文章链接 |
| `image_url` | VARCHAR(1000) | — | 封面图 |
| `pub_date` | TIMESTAMP | — | 发布时间 |
| `author` | VARCHAR(200) | — | 作者 |
| `read` | BOOLEAN | DEFAULT false | 是否已读（索引 `idx_articles_read`） |
| `favorite` | BOOLEAN | DEFAULT false | 是否收藏（索引 `idx_articles_favorite`） |
| `summary_status` | VARCHAR(20) | DEFAULT 'complete' | AI 总结状态：`incomplete` / `pending` / `complete` / `failed` |
| `summary_generated_at` | TIMESTAMP | — | AI 总结生成时间 |
| `summary_processing_started_at` | TIMESTAMP | — | AI 总结开始处理时间 |
| `completion_attempts` | INTEGER | DEFAULT 0 | AI 总结重试次数 |
| `completion_error` | TEXT | — | AI 总结错误信息 |
| `ai_content_summary` | TEXT | — | AI 生成的优化总结内容（Markdown） |
| `firecrawl_status` | VARCHAR(20) | DEFAULT 'pending' | Firecrawl 抓取状态：`pending` / `processing` / `completed` / `failed` |
| `firecrawl_error` | TEXT | — | Firecrawl 抓取错误信息 |
| `firecrawl_content` | TEXT | — | Firecrawl 抓取的完整网页内容（Markdown） |
| `firecrawl_crawled_at` | TIMESTAMP | — | Firecrawl 抓取时间 |
| `search_vector` | tsvector | — | 全文检索向量（触发器维护，见下） |
| `created_at` | TIMESTAMP | — | 创建时间 |

**全文检索（迁移 `20260417_0002`）**：`search_vector` 列由触发器 `articles_search_vector_trigger`（BEFORE INSERT OR UPDATE OF title, description）维护，权重 A=title、B=description；GIN 索引 `idx_articles_search_vector`。

**虚拟字段（`gorm:"->"` 计算列，非持久化）**：`tag_count`（文章标签数）、`relevance_score`（相关度评分）。

> ⚠️ 旧文档曾列 `feed_summary_id` / `feed_summary_generated_at`：**代码 `Article` struct 无此字段**，已删除，请勿引用。

**复合索引（迁移 `20260417_0001`）**：`idx_articles_feed_pub_date(feed_id, pub_date DESC)`、`idx_articles_feed_id_title(feed_id, title)`。

### 1.2 feeds（订阅源表）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `title` | VARCHAR(200) | NOT NULL | 订阅源标题 |
| `description` | TEXT | — | 描述 |
| `url` | VARCHAR(500) | UNIQUE NOT NULL | RSS URL |
| `category_id` | INTEGER | index（`idx_feeds_category_id` 迁移补） | 所属分类 ID（逻辑关联 `categories.id`） |
| `icon` | VARCHAR(1000) | DEFAULT 'rss' | 图标值（iconify id 或图片 URL） |
| `icon_source` | VARCHAR(20) | DEFAULT 'fallback' | 图标来源状态机：`auto` / `custom` / `fallback` |
| `color` | VARCHAR(20) | DEFAULT '#8b5cf6' | 颜色 |
| `last_updated` | TIMESTAMP | — | 最后更新时间 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `max_articles` | INTEGER | DEFAULT 100 | 最大文章数 |
| `refresh_interval` | INTEGER | DEFAULT 60 | 刷新间隔（秒） |
| `refresh_status` | VARCHAR(20) | DEFAULT 'idle' | 刷新状态 |
| `refresh_error` | TEXT | — | 刷新错误信息 |
| `last_refresh_at` | TIMESTAMP | — | 最后刷新时间 |
| `article_summary_enabled` | BOOLEAN | DEFAULT false | 是否启用文章级 AI 总结（依赖 Firecrawl） |
| `completion_on_refresh` | BOOLEAN | DEFAULT true | 刷新时是否自动触发内容补全 |
| `max_completion_retries` | INTEGER | DEFAULT 3 | AI 总结最大重试次数 |
| `firecrawl_enabled` | BOOLEAN | DEFAULT false | 是否启用 Firecrawl 抓取 |
| `tagging_enabled` | BOOLEAN | DEFAULT true | 是否启用自动打标签 |

> ⚠️ 旧文档曾列 `ai_summary_enabled`（DEFAULT true）：**代码无此字段**。实际是 `article_summary_enabled`（DEFAULT false），两者曾被混淆，请勿再引用 `ai_summary_enabled`。

`Feed.Articles` 声明了 `constraint:OnDelete:CASCADE`（GORM 层）；`Feed.Category` 为逻辑关联。

### 1.3 categories（分类表）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `name` | VARCHAR(100) | UNIQUE NOT NULL | 分类名称 |
| `slug` | VARCHAR(50) | UNIQUE | URL 友好标识 |
| `icon` | VARCHAR(50) | DEFAULT 'folder' | 图标 |
| `color` | VARCHAR(20) | DEFAULT '#6366f1' | 颜色 |
| `description` | TEXT | — | 描述 |
| `created_at` | TIMESTAMP | — | 创建时间 |

`Category.Feeds` 声明了 `constraint:OnDelete:CASCADE`（GORM 层）。

---

## §2 调度与配置域

### 2.1 scheduler_tasks（调度任务表）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `name` | VARCHAR(50) | UNIQUE NOT NULL; index | 任务名称 |
| `description` | VARCHAR(200) | — | 任务描述 |
| `check_interval` | INTEGER | DEFAULT 60 NOT NULL | 检查间隔（秒） |
| `last_execution_time` | TIMESTAMP | — | 上次执行时间 |
| `next_execution_time` | TIMESTAMP | — | 下次执行时间 |
| `status` | VARCHAR(20) | DEFAULT 'idle'; index | 状态 |
| `last_error` | TEXT | — | 最近错误 |
| `last_error_time` | TIMESTAMP | — | 最近错误时间 |
| `total_executions` | INTEGER | DEFAULT 0 | 总执行次数 |
| `successful_executions` | INTEGER | DEFAULT 0 | 成功次数 |
| `failed_executions` | INTEGER | DEFAULT 0 | 失败次数 |
| `consecutive_failures` | INTEGER | DEFAULT 0 | 连续失败次数 |
| `last_execution_duration` | FLOAT | —（`*float64`，可空） | 上次执行耗时（秒） |
| `last_execution_result` | TEXT | — | 上次执行结果 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

| 任务名（示例） | 描述 | 执行间隔 |
| -------- | ------ | ---------- |
| `auto_refresh` | 自动刷新 RSS 订阅源 | 60 秒 |
| `ai_summary` | AI 智能总结文章内容（基于 Firecrawl） | 3600 秒 |

### 2.2 ai_settings（AI 配置键值对）

通用键值对表，承载大量运行时配置 seed（如 `semantic_board_match_*`、`persistent_topic_*`、`event_cluster_*`、`daily_report_time`、`auxiliary_label_dedupe_sim` 等）。具体键由各业务域代码读写，非表结构约束。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `key` | VARCHAR(100) | UNIQUE NOT NULL; index | 配置键 |
| `value` | TEXT | — | JSON / 字符串值 |
| `description` | VARCHAR(200) | — | 说明 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

---

## §3 AI 路由域

### 3.1 ai_providers（AI 供应商）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `name` | VARCHAR(100) | UNIQUE NOT NULL; index | 供应商名称 |
| `provider_type` | VARCHAR(50) | NOT NULL DEFAULT 'openai_compatible'; index | 供应商类型 |
| `base_url` | VARCHAR(500) | NOT NULL | API 地址 |
| `api_key` | TEXT | — | API 密钥（可空） |
| `model` | VARCHAR(100) | NOT NULL | 模型名称 |
| `enabled` | BOOLEAN | NOT NULL DEFAULT true; index | 是否启用 |
| `timeout_seconds` | INTEGER | NOT NULL DEFAULT 120 | 超时时间 |
| `max_tokens` | INTEGER | —（`*int`，可空） | 最大 token 数 |
| `temperature` | FLOAT | —（`*float64`，可空） | 温度参数 |
| `enable_thinking` | BOOLEAN | NOT NULL DEFAULT false | 是否启用模型推理（传播 `chat_template_kwargs.enable_thinking`） |
| `metadata` | TEXT | — | 扩展元数据 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> `enable_thinking` 语义曾从「事后剥离 `<think>` 标签」翻转为「启用模型推理」；迁移 `20260626_0001` 部署时批量 reset 为 false，避免旧 true 值意外拖慢打标签。

### 3.2 ai_routes（AI 路由）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `name` | VARCHAR(100) | NOT NULL; 复合唯一 `idx_ai_routes_capability_name(name, capability)` | 路由名称 |
| `capability` | VARCHAR(50) | NOT NULL; 复合唯一同上; index | 能力标识 |
| `enabled` | BOOLEAN | NOT NULL DEFAULT true; index | 是否启用 |
| `priority` | INTEGER | NOT NULL DEFAULT 100; index | 优先级（数值越小越高） |
| `strategy` | VARCHAR(50) | NOT NULL DEFAULT 'ordered_failover' | 路由策略 |
| `description` | VARCHAR(255) | — | 描述 |
| `max_concurrency` | INTEGER | NOT NULL DEFAULT 0 | 最大并发（0 = 用各能力的默认值） |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_ai_routes_capability_name (name, capability)`。`AIRoute.RouteProviders` 为逻辑关联（无 OnDelete）。

### 3.3 ai_route_providers（AI 路由-供应商绑定）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `route_id` | INTEGER | NOT NULL; 复合唯一 `idx_ai_route_provider_link(route_id, provider_id)` | 路由 ID |
| `provider_id` | INTEGER | NOT NULL; 复合唯一同上 | 供应商 ID |
| `priority` | INTEGER | NOT NULL DEFAULT 100; index | 优先级（数值越小越高） |
| `enabled` | BOOLEAN | NOT NULL DEFAULT true; index | 是否启用 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

### 3.4 ai_call_logs（AI 调用日志）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `operation` | VARCHAR(80) | NOT NULL; 复合索引 `idx_call_logs_op_time(operation, created_at)` priority:1 | 业务操作名（如 `daily_report.cluster_tags`） |
| `capability` | VARCHAR(50) | NOT NULL; index | 能力标识 |
| `route_name` | VARCHAR(100) | NOT NULL | 路由名称 |
| `provider_name` | VARCHAR(100) | NOT NULL | 供应商名称 |
| `model` | VARCHAR(100) | — | 实际调用的模型名 |
| `success` | BOOLEAN | NOT NULL; index | 是否成功 |
| `is_fallback` | BOOLEAN | DEFAULT false | 是否为降级调用 |
| `latency_ms` | INTEGER | — | 延迟（毫秒） |
| `error_code` | VARCHAR(100) | — | 错误码 |
| `error_message` | TEXT | — | 错误信息 |
| `prompt` | TEXT | — | 完整 messages 文本（超 20000 runes 截断标注） |
| `request_meta` | TEXT | — | 请求元数据 |
| `response_snippet` | TEXT | — | 响应片段（截取前 10000 runes） |
| `token_usage` | JSONB | — | prompt/completion/total token 用量 JSON |
| `trace_id` | VARCHAR(64) | — | OpenTelemetry trace ID |
| `session_id` | VARCHAR(120) | index（`idx_call_logs_session`） | 编排分组键（同一次编排内共享） |
| `created_at` | TIMESTAMP | index（`idx_ai_call_logs_created_at`） | 创建时间 |

`operation` / `prompt` / `token_usage` / `session_id` / `model` 五列由迁移 `20260704_0001` 补齐（R2 必记字段）。

---

## §4 主题标签域

### 4.1 topic_tags（主题标签主表）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `slug` | VARCHAR(120) | NOT NULL; 复合索引 `idx_topic_tags_category_slug(category, slug)` | 稳定标识 |
| `label` | VARCHAR(160) | NOT NULL | 展示名称 |
| `category` | VARCHAR(20) | NOT NULL DEFAULT 'keyword'; 复合索引同上 | 分类：`event` / `person` / `keyword` |
| `icon` | VARCHAR(100) | — | Iconify 图标 ID |
| `aliases` | TEXT | — | 别名列表（JSON 数组） |
| `description` | TEXT | — | LLM 生成的标签描述 |
| `is_canonical` | BOOLEAN | DEFAULT false | 是否为规范标签 |
| `source` | VARCHAR(20) | DEFAULT 'llm' | 来源：`llm` / `heuristic` / `manual` |
| `feed_count` | INTEGER | DEFAULT 0 | 引用此标签的不重复 Feed 数 |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'active'; index | 状态：`active` / `merged` |
| `merged_into_id` | INTEGER | index | 合并目标标签 ID（**唯一真实 DB FK**：`topic_tags_merged_into_id_fkey ... ON DELETE CASCADE`） |
| `is_watched` | BOOLEAN | DEFAULT false | 是否为用户关注标签 |
| `watched_at` | TIMESTAMP | —（`*time.Time`） | 关注时间 |
| `quality_score` | FLOAT | DEFAULT 0 | 质量评分 |
| `metadata` | JSONB | DEFAULT '{}'（serializer:json） | 扩展元数据 |
| `kind` | VARCHAR(20) | DEFAULT 'keyword' | **已废弃**，映射到 `category`，保留兼容 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> ⚠️ `(category, slug)` 是**普通复合索引**（gorm tag `index:idx_topic_tags_category_slug`，**非 unique**），不强制唯一。
> ⚠️ 旧文档曾列 `concept_id`：迁移 `20260522_0001` 已 `DROP COLUMN`，struct 无此字段，请勿引用。

### 4.2 topic_tag_embeddings（主题标签向量）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `topic_tag_id` | INTEGER | NOT NULL; 复合唯一 `idx_topic_tag_embeddings_tag_type_hash` | 关联标签 ID（逻辑关联，无 DB FK） |
| `embedding_type` | VARCHAR(20) | NOT NULL DEFAULT 'identity'; 复合唯一同上 | 嵌入类型：`identity` / `semantic` / `event_keyword` |
| `embedding` | **vector(4096)** | — | pgvector 向量列（**迁移 `20260403_0003` 固定 4096 维**；struct 字段名 `embedding_vec`，列名 `embedding`） |
| `dimension` | INTEGER | NOT NULL | 向量维度 |
| `model` | VARCHAR(50) | NOT NULL | 生成模型名称 |
| `text_hash` | VARCHAR(64) | 复合唯一同上 | 标签文本哈希，参与唯一约束（同一 tag+type 可有多行不同 text_hash） |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_topic_tag_embeddings_tag_type_hash (topic_tag_id, embedding_type, text_hash)`（gorm tag + 迁移 `20260514_0001` 双重声明）。

> ⚠️ `topic_tag_embeddings.embedding` 是唯一维度固定的向量列（4096）。旧文档曾写 `vector(1536)`，错误。
> ⚠️ 旧文档曾列 `vector`（TEXT 旧版 JSON 向量）：迁移 `20260601_0001b` 已 `DROP COLUMN`，请勿引用。
> `TopicTagEmbedding.TopicTag` 声明了 `constraint:OnDelete:CASCADE`（GORM 层）。

### 4.3 topic_tag_analyses（主题分析快照）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `topic_tag_id` | BIGINT | 复合唯一 `idx_tag_analysis_date` | 关联标签 ID |
| `analysis_type` | string(256) | 复合唯一同上 | 分析类型：`event` / `person` / `keyword` |
| `window_type` | string(256) | 复合唯一同上 | 时间窗：`daily` / `weekly` |
| `anchor_date` | TIMESTAMP | 复合唯一同上 | 锚点日期 |
| `article_count` | INTEGER | — | 覆盖的文章数量 |
| `payload_json` | TEXT | — | 分析结果 JSON |
| `source` | string(256) | — | 来源：`ai` / `heuristic` / `cached` |
| `version` | INTEGER | — | 分析版本号 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_tag_analysis_date (topic_tag_id, analysis_type, window_type, anchor_date)`。

> ⚠️ 旧文档曾列 `summary_count`：真实列名是 **`article_count`**。

### 4.4 topic_analysis_cursors（主题分析游标）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `topic_tag_id` | BIGINT | 复合唯一 `idx_cursor_tag_type_window` | 关联标签 ID |
| `analysis_type` | string(256) | 复合唯一同上 | 分析类型 |
| `window_type` | string(256) | 复合唯一同上 | 时间窗 |
| `last_article_id` | BIGINT | — | 上次分析已处理到的最大文章 ID |
| `last_updated_at` | TIMESTAMP | — | 上次刷新时间 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_cursor_tag_type_window (topic_tag_id, analysis_type, window_type)`。

> ⚠️ 旧文档曾列 `last_summary_id`：真实列名是 **`last_article_id`**。

### 4.5 article_topic_tags（文章-主题关联）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `article_id` | INTEGER | NOT NULL; index `idx_article_topic_tag_article`; 复合唯一 `idx_article_topic_tags_link` | 文章 ID |
| `topic_tag_id` | INTEGER | NOT NULL; index `idx_article_topic_tag_topic`; 复合唯一同上 | 标签 ID |
| `score` | FLOAT | DEFAULT 0 | 相关度评分 |
| `source` | VARCHAR(20) | DEFAULT 'llm' | 来源 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_article_topic_tags_link (article_id, topic_tag_id)`。两端关联均声明 `constraint:OnDelete:CASCADE`（GORM 层）。

> 标签任务写入关联前会在短事务内以 `FOR KEY SHARE` 锁定对应文章。若 Feed 清理已删除该文章，则跳过关联写入并正常完成任务。
> 单列索引：`idx_article_topic_tags_article_id(article_id)`（迁移 `20260417_0001`）。

### 4.6 topic_tag_relations（标签层级关系）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `parent_id` | INTEGER | NOT NULL; 复合唯一 `idx_tag_relation_pair` | 父标签 ID（逻辑关联 `topic_tags.id`，无 DB FK） |
| `child_id` | INTEGER | NOT NULL; 复合唯一同上 | 子标签 ID |
| `relation_type` | VARCHAR(20) | NOT NULL DEFAULT 'abstract' | 关系类型：`abstract` / `synonym` / `related` |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

唯一约束：`idx_tag_relation_pair (parent_id, child_id)`。两端关联均为逻辑关联（无 OnDelete）。

### 4.7 tag_merge_suggestions（标签合并建议）

记录一对相似标签供人工合并决策。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `new_tag_id` | INTEGER | NOT NULL; 复合唯一 `idx_tag_merge_suggestion_pair` | 新标签 ID |
| `existing_tag_id` | INTEGER | NOT NULL; 复合唯一同上 | 既有标签 ID |
| `new_label` | VARCHAR(160) | NOT NULL | 新标签名快照 |
| `existing_label` | VARCHAR(160) | NOT NULL | 既有标签名快照 |
| `category` | VARCHAR(20) | NOT NULL | 分类 |
| `similarity` | FLOAT | NOT NULL; 复合索引 `idx_tag_merge_suggestion_status_sim(status, similarity)` | 相似度 |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending'; 复合索引同上 | 状态：`pending` / `merged` / `dismissed` |
| `source` | VARCHAR(20) | NOT NULL DEFAULT 'incremental' | 来源：`incremental` / `full_scan` |
| `llm_verdict` | TEXT | — | LLM 判定文本 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

---

## §5 语义标签 / 板块域

### 5.1 semantic_labels（语义标签统一表）

辅助标签（`label_type=auxiliary`）和 SemanticBoard（`label_type=board`）共存于同一张表，通过 `label_type` 区分。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `label` | VARCHAR(160) | NOT NULL | 展示名称 |
| `slug` | VARCHAR(**160**) | NOT NULL; **单列唯一** `idx_semantic_labels_slug` | 稳定标识 |
| `embedding` | vector | —（运行时维度） | 语义向量（struct `*string`，列名 `embedding`） |
| `merge_embedding` | vector | —（运行时维度） | 合并去重用向量（struct `*string`，列名 `merge_embedding`） |
| `label_type` | VARCHAR(20) | NOT NULL; index `idx_semantic_labels_label_type` | 类型：`auxiliary` / `board`（约定，无 CHECK） |
| `aliases` | JSONB | DEFAULT '[]'（serializer:json） | 别名列表 |
| `ref_count` | INTEGER | NOT NULL DEFAULT 0 | 引用计数（辅助标签被 tag 引用次数） |
| `description` | TEXT | — | 描述 |
| `display_order` | INTEGER | NOT NULL DEFAULT 0 | 显示排序 |
| `source` | VARCHAR(**50**) | NOT NULL DEFAULT 'llm_extract' | 来源：`llm_extract` / `llm_suggest` / `manual`（约定） |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'active'; index `idx_semantic_labels_status` | 状态：`active` / `disabled` |
| `protected` | BOOLEAN | NOT NULL DEFAULT false | 是否受保护（不可自动删除） |
| `enrichment_enabled` | BOOLEAN | NOT NULL DEFAULT false | 循环 B 增强开关（默认关，耗资源需先绑数据源） |
| `window_days` | INTEGER | NOT NULL DEFAULT 14 | 循环 B 实时详情窗口天数（范围 1-365） |
| `context_layers` | JSONB | DEFAULT '["week","month","year","all"]' | 解读员读取的分层粒度配置 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> ⚠️ `slug` 约束是**单列 UNIQUE**（迁移 `20260521_0001` `UNIQUE(slug)`）。旧文档曾写「唯一约束 `(label_type, slug)`」，错误。
> ⚠️ `embedding` / `merge_embedding` 均为运行时维度向量列（旧文档曾写 `vector(1536)`，错误）。

### 5.2 topic_tag_semantic_labels（tag-辅助标签关联，中间表）

纯关联表，**无 `id` 列，无时间戳**，复合主键。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `topic_tag_id` | BIGINT | PK; NOT NULL | 关联 tag ID（逻辑关联，GORM 声明 OnDelete:CASCADE） |
| `semantic_label_id` | BIGINT | PK; NOT NULL | 关联辅助标签 ID |

主键：`(topic_tag_id, semantic_label_id)`。索引：`idx_topic_tag_semantic_labels_topic_tag_id`、`idx_topic_tag_semantic_labels_semantic_label_id`（迁移 `20260521_0001`）。

> ⚠️ 旧文档曾列 `id BIGSERIAL PK`：代码无 `id`，复合主键。

### 5.3 topic_tag_board_labels（tag-SemanticBoard 匹配结果，中间表）

复合主键，无 `id`。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `topic_tag_id` | BIGINT | PK; NOT NULL | 关联 tag ID |
| `semantic_board_id` | BIGINT | PK; NOT NULL | 关联 SemanticBoard ID |
| `score` | FLOAT | NOT NULL DEFAULT 0 | 匹配分数 |
| `match_reason` | TEXT | —（自由文本，无 size/CHECK） | 匹配原因说明（如 `direct_hit` / `hit_rate` / `max_sim` / `weighted`，仅约定非强制） |
| `downgraded` | BOOLEAN | NOT NULL DEFAULT false | 是否被降级（命中后人工/规则下调） |
| `direction_mismatch` | BOOLEAN | NOT NULL DEFAULT false | 方向不匹配标记 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

主键：`(topic_tag_id, semantic_board_id)`。索引：`idx_topic_tag_board_labels_topic_tag_id`、`idx_topic_tag_board_labels_semantic_board_id`（迁移 `20260521_0001`）。

> ⚠️ 旧文档曾列 `id BIGSERIAL PK` 且 `match_reason VARCHAR(20)`：代码无 `id`，`match_reason` 为 `type:text` 自由文本。

### 5.4 board_composition（board 构成，中间表）

纯关联表，**无 `id` 列，无时间戳**，复合主键。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `board_id` | BIGINT | PK; NOT NULL | 关联 board ID（`label_type=board`） |
| `auxiliary_label_id` | BIGINT | PK; NOT NULL | 关联辅助标签 ID（`label_type=auxiliary`） |

主键：`(board_id, auxiliary_label_id)`。索引：`idx_board_composition_board_id`、`idx_board_composition_auxiliary_label_id`（迁移 `20260521_0001`）。

> ⚠️ 旧文档曾列 `id BIGSERIAL PK`：代码无 `id`，复合主键。

### 5.5 board_upgrade_suggestions（板块升级建议）

每次板块升级生成批次产出的建议，带生命周期（pending → confirmed / dismissed）。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `batch_id` | VARCHAR(64) | NOT NULL; index | 生成批次 |
| `mode` | VARCHAR(32) | NOT NULL | 模式 |
| `decision` | VARCHAR(32) | NOT NULL | 决策：`create_new` / `merge_into_existing` / `watch` |
| `board_label` | VARCHAR(160) | NOT NULL | 板块标签 |
| `description` | TEXT | — | 描述 |
| `target_board_id` | INTEGER | —（`*uint`，可空） | 目标板块 ID（merge_into_existing 时） |
| `auxiliary_label_ids` | JSONB | DEFAULT '[]'（serializer:json） | 辅助标签 ID 列表 |
| `confidence` | VARCHAR(16) | NOT NULL DEFAULT 'llm' | 置信度：`high` / `llm` |
| `evidence` | JSONB | —（serializer:json） | 证据快照 `{shortlist, margins, cotag_events, lane_briefs}` |
| `status` | VARCHAR(16) | NOT NULL DEFAULT 'pending'; index `idx_board_upgrade_suggestions_status` | 状态：`pending` / `confirmed` / `dismissed` |
| `dismiss_reason` | TEXT | —（`*string`，可空） | 驳回原因 |
| `suggestion_hash` | VARCHAR(64) | NOT NULL | 稳定指纹 `(mode, decision, target_board_id, sorted_auxiliary_label_ids)` |
| `resolved_at` | TIMESTAMP | —（`*time.Time`） | 处理时间 |
| `resolved_by` | VARCHAR(50) | —（`*string`） | 处理人 |
| `created_at` | TIMESTAMP | — | 创建时间 |

**部分唯一索引（迁移 `20260717_0001`）**：`uq_board_upgrade_suggestions_hash UNIQUE(suggestion_hash) WHERE status='pending'`，保证同簇同决策不重复插入 pending 行；dismissed 行同 hash 在重新生成时用于冷却复查。

---

## §6 向量域

### 6.1 embedding_config（向量配置，键值对）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `key` | VARCHAR(100) | UNIQUE NOT NULL; index | 配置键 |
| `value` | TEXT | NOT NULL | 配置值 |
| `description` | VARCHAR(200) | — | 说明 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> ⚠️ **默认配置项已变更**：`high_similarity_threshold` / `low_similarity_threshold` / `embedding_dimension` / `embedding_model` 曾由迁移 `20260413_0002` seed，但已被迁移 `20260614_0001` **DELETE 清除**，现由运行时代码管理，不再 seed。`narrative_board_embedding_threshold` / `narrative_board_hotspot_threshold` 亦已删（迁移 `20260522_0001`，已废弃）。
> 当前仍 seed 的键：`event_cluster_kw_min_overlap`、`event_cluster_sem_threshold`（迁移 `20260514_0002`）。

### 6.2 embedding_queues（向量生成队列）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `tag_id` | BIGINT | NOT NULL; index | 关联标签 ID（逻辑关联 `topic_tags.id`，无 DB FK） |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending'; index | 状态：`pending` / `processing` / `completed` / `failed` |
| `error_message` | TEXT | — | 错误信息 |
| `retry_count` | INTEGER | DEFAULT 0 | 重试次数 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `started_at` | TIMESTAMP | —（`*time.Time`） | 开始时间 |
| `completed_at` | TIMESTAMP | —（`*time.Time`） | 完成时间 |

> 关联为逻辑关联（无 OnDelete，DB FK 已 drop）。

### 6.3 merge_reembedding_queues（合并后重算向量队列）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `source_tag_id` | BIGINT | NOT NULL; index | 源标签 ID（逻辑关联，无 DB FK） |
| `target_tag_id` | BIGINT | NOT NULL; index | 目标标签 ID（逻辑关联，无 DB FK） |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending'; index | 状态：`pending` / `processing` / `completed` / `failed` |
| `error_message` | TEXT | — | 错误信息 |
| `retry_count` | INTEGER | DEFAULT 0 | 重试次数 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `started_at` | TIMESTAMP | —（`*time.Time`） | 开始时间 |
| `completed_at` | TIMESTAMP | —（`*time.Time`） | 完成时间 |

---

## §7 任务队列域

### 7.1 firecrawl_jobs（Firecrawl 抓取任务）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `article_id` | INTEGER | NOT NULL; index | 关联文章 ID（GORM 声明 OnDelete:CASCADE） |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending'; index | 状态：`pending` / `leased` / `completed` / `failed` |
| `priority` | INTEGER | DEFAULT 0; index | 优先级 |
| `attempt_count` | INTEGER | DEFAULT 0 | 尝试次数 |
| `max_attempts` | INTEGER | DEFAULT 5 | 最大尝试次数 |
| `available_at` | TIMESTAMP | NOT NULL; index | 可执行时间 |
| `leased_at` | TIMESTAMP | —（`*time.Time`，无 gorm tag 普通列） | 租约获取时间 |
| `lease_expires_at` | TIMESTAMP | —（`*time.Time`）; index | 租约过期时间 |
| `last_error` | TEXT | — | 最近错误 |
| `url_snapshot` | VARCHAR(1000) | — | URL 快照 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

### 7.2 tag_jobs（标签任务）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `article_id` | INTEGER | NOT NULL; index | 关联文章 ID（GORM 声明 OnDelete:CASCADE） |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'pending'; index | 状态：`pending` / `leased` / `completed` / `failed` |
| `priority` | INTEGER | DEFAULT 0; index | 优先级 |
| `attempt_count` | INTEGER | DEFAULT 0 | 尝试次数 |
| `max_attempts` | INTEGER | DEFAULT 5 | 最大尝试次数 |
| `available_at` | TIMESTAMP | NOT NULL; index | 可执行时间 |
| `leased_at` | TIMESTAMP | —（`*time.Time`） | 租约获取时间 |
| `lease_expires_at` | TIMESTAMP | —（`*time.Time`）; index | 租约过期时间 |
| `last_error` | TEXT | — | 最近错误 |
| `feed_name_snapshot` | VARCHAR(200) | — | Feed 名称快照 |
| `category_name_snapshot` | VARCHAR(100) | — | 分类名称快照 |
| `force_retag` | BOOLEAN | DEFAULT false | 是否强制重新打标签 |
| `reason` | VARCHAR(50) | — | 入队原因 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> ⚠️ `firecrawl_jobs` / `tag_jobs` 的 `status`、`available_at`、`lease_expires_at` 均为各自单列索引（非旧文档所写复合索引）。

---

## §8 叙事域

### 8.1 narrative_summaries（叙事摘要）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `title` | VARCHAR(300) | NOT NULL | 叙事标题 |
| `summary` | TEXT | NOT NULL | 叙事内容 |
| `status` | VARCHAR(20) | NOT NULL; index | 状态：`emerging` / `continuing` / `splitting` / `merging` / `ending` |
| `period` | VARCHAR(20) | NOT NULL DEFAULT 'daily' | 周期：`daily` / `watched_tag` |
| `period_date` | TIMESTAMP | NOT NULL; index `idx_narrative_period_date` | 周期日期 |
| `generation` | INTEGER | NOT NULL DEFAULT 0 | 代际 |
| `parent_ids` | TEXT | — | 父叙事 ID 列表 |
| `related_tag_ids` | TEXT | — | 关联标签 ID 列表 |
| `related_article_ids` | TEXT | — | 关联文章 ID 列表 |
| `source` | VARCHAR(20) | DEFAULT 'ai' | 来源 |
| `scope_type` | VARCHAR(20) | NOT NULL DEFAULT 'global' | 作用域：`global` / `feed_category` / **`board`** |
| `scope_category_id` | INTEGER | —（`*uint`）; index `idx_narrative_scope` | 分类 ID |
| `scope_label` | VARCHAR(100) | — | 分类名称 |
| `board_id` | INTEGER | —（`*uint`）; index | 所属 Board ID（逻辑关联 `narrative_boards.id`） |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> `scope_type` 比旧文档多一个 `board` 值（`NarrativeScopeTypeBoard`）。
> 复合索引：`idx_narrative_scope_period(scope_type, scope_category_id, period_date)`；单列 `idx_narrative_summaries_board_id(board_id)`。

### 8.2 narrative_boards（叙事板块）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `period_date` | TIMESTAMP | NOT NULL; index `idx_narrative_boards_period` | 周期日期 |
| `name` | VARCHAR(300) | NOT NULL | 板块名称 |
| `description` | TEXT | — | 板块描述 |
| `scope_type` | VARCHAR(20) | NOT NULL DEFAULT 'global' | 作用域：`global` / `feed_category` |
| `scope_category_id` | INTEGER | —（`*uint`）; index `idx_narrative_boards_scope` | 分类 ID |
| `scope_label` | VARCHAR(100) | — | 分类名称 |
| `event_tag_ids` | TEXT | — | 关联 event 标签 ID 列表（JSON 数组） |
| `prev_board_ids` | TEXT | — | 前日关联 Board ID 列表（JSON 数组，跨日延续） |
| `semantic_board_id` | INTEGER | —（`*uint`）; index `idx_narrative_boards_semantic_board_id` | 关联 SemanticBoard ID（逻辑关联 `semantic_labels.id`） |
| `is_system` | BOOLEAN | NOT NULL DEFAULT false | 是否为系统自动生成 |
| `created_at` | TIMESTAMP | — | 创建时间 |

> 注：迁移 `20260522_0001` 曾 `DROP COLUMN is_system`，但 struct 仍保留 `IsSystem`，AutoMigrate 每次启动重建该列，**实际列存在**。

---

## §9 日报 / 持久话题 / Watch 域

> 本域 7 张表由 `internal/topicgraph` 的 `RegisterModels` 注册（`init()`），部署若未引入该包则表不存在；迁移对它们用 `tableExists` 守卫，缺失时安全跳过。

### 9.1 board_daily_reports（板块日报主表）

每天每板块一条日报。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `semantic_board_id` | INTEGER | NOT NULL; index `idx_board_daily_reports_semantic_board_id` | 所属语义板块 |
| `period_date` | DATE | NOT NULL | 周期日期 |
| `title` | VARCHAR(256) | — | 日报标题 |
| `summary` | VARCHAR(256) | — | 日报摘要 |
| `highlights` | JSONB | — | 要点 |
| `dynamics` | TEXT | — | 动态 |
| `article_count` | INTEGER | — | 文章数 |
| `event_tag_count` | INTEGER | — | 事件标签数 |
| `cluster_count` | INTEGER | — | 聚类数 |
| `status` | VARCHAR(20) | DEFAULT 'generating' | 状态（`generating` / 完成） |
| `raw_clusters` | JSONB | — | 原始聚类 |
| `prev_report_id` | INTEGER | —（`*uint`） | 前一日报告 ID |
| `generation_prompt_version` | VARCHAR(20) | — | 生成 prompt 版本 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

`Sections` 关联为逻辑关联（无 OnDelete）。

### 9.2 daily_report_sections（日报分区）

一分区 = 一聚类。承载归属持久话题的字段。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `report_id` | INTEGER | NOT NULL; index `idx_daily_report_sections_report_id` | 所属日报 |
| `cluster_index` | INTEGER | — | 聚类序号 |
| `cluster_label` | VARCHAR(200) | — | 聚类标签 |
| `cluster_tag_ids` | JSONB | — | 聚类标签 ID 列表 |
| `article_count` | INTEGER | — | 文章数 |
| `best_tier` | INTEGER | DEFAULT 0 | 最佳层级 |
| `avg_score` | FLOAT | DEFAULT 0 | 平均分 |
| `quality_breakdown` | JSONB | —（迁移 `20260625_0001`） | 质量分解 |
| `embedding` | vector | —（运行时维度，迁移 `20260601_0002`） | 聚类向量 |
| `persistent_topic_id` | INTEGER | —（`*uint`，**刻意无 NOT NULL**）; index | 归属持久话题 ID（容忍回填窗口） |
| `topic_match_distance` | FLOAT | — | 归属匹配距离 |
| `topic_match_confidence` | VARCHAR(20) | — | 归属置信度：`anchor_hit` / `auto_new` / `unmatched` / **`manual`** |
| `topic_status_at_report` | VARCHAR(20) | —（`*string`，可空，迁移 `20260627_0001`） | 报告生成时话题状态快照（`candidate` / `active` / NULL） |
| `created_at` | TIMESTAMP | — | 创建时间 |

> HNSW 索引 `idx_daily_report_sections_embedding`：运行时由 `ensureSectionEmbeddingDimension` 创建（dim ≤ 2000 才建）。
> **瞬态字段（`gorm:"-"`，非持久化）**：`PersistentTopic`、`MatchedTopicID`。
> **已删列**：`threads`（JSONB，迁移 `20260529_0003` 迁移到独立表后 drop）、`prev_section_id`、`status`（迁移 `20260603_0001`）。

### 9.3 daily_report_threads（日报叙事线程）

从分区独立出来的叙事线程（迁移 `20260529_0002` 从 sections.threads JSONB 迁出）。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `report_id` | INTEGER | NOT NULL; index `idx_daily_report_threads_report_id` | 所属日报 |
| `section_id` | INTEGER | NOT NULL; index `idx_daily_report_threads_section_id` | 所属分区 |
| `title` | VARCHAR(256) | — | 线程标题 |
| `summary` | VARCHAR(256) | — | 线程摘要 |
| `tag_ids` | JSONB | — | 关联标签 ID 列表 |
| `confidence` | FLOAT | DEFAULT 0 | 置信度 |
| `related_article_ids` | JSONB | — | 关联文章 ID 列表 |
| `embedding` | vector | —（运行时维度） | 线程向量 |
| `fit_distance` | FLOAT | —（`*float64`，**无 default**，刻意区分 nil 与 0.0） | 与所属分区的契合距离（nil = 无信号，0.0 = 完美契合） |
| `created_at` | TIMESTAMP | — | 创建时间 |

> **已删列**：`status`、`prev_thread_id`（迁移 `20260603_0001`）。

### 9.4 daily_report_section_relations（跨日分区关系，多对多）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `from_section_id` | INTEGER | NOT NULL; index `idx_section_relations_from` | 起点分区 |
| `to_section_id` | INTEGER | NOT NULL; index `idx_section_relations_to` | 终点分区 |
| `distance` | FLOAT | NOT NULL | 距离 |
| `relation_type` | VARCHAR(20) | NOT NULL DEFAULT 'similarity'; index `idx_section_relations_type` | 关系：`similarity`（匈牙利时间线匹配）/ `identity`（持久话题连续性） |
| `created_at` | TIMESTAMP | — | 创建时间 |

**唯一约束（迁移 `20260620_0001`，三列宽化）**：`uq_section_relations_pair UNIQUE(from_section_id, to_section_id, relation_type)` —— 允许同一分区对的 identity 边与 similarity 边并存。

### 9.5 board_persistent_topics（板块持久叙事话题）

板块内持久叙事框架。一个板块 N 个话题，每个 section 归属一个话题（1:N）。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `semantic_board_id` | INTEGER | NOT NULL; 复合索引 `idx_persistent_topics_board_status(semantic_board_id, status)` priority:1 | 所属语义板块 |
| `label` | VARCHAR(200) | NOT NULL | 持久叙事标题 |
| `description` | TEXT | — | 描述 |
| `embedding` | vector | —（运行时维度） | 归属匹配与历史回刷聚类向量 |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'candidate'; 复合索引同上 priority:2; **CHECK** `chk_board_persistent_topics_status (status IN ('candidate','active','archived'))` | 状态：`candidate`（观察中）/ `active`（连续命中后晋升）/ `archived`（衰减归档） |
| `source` | VARCHAR(10) | NOT NULL DEFAULT 'auto'; **CHECK** `chk_board_persistent_topics_source (source IN ('auto','manual'))` | 来源：`auto`（算法聚类）/ `manual`（用户手动建泳道，绕过 candidate 直接 active） |
| `first_seen_date` | DATE | NOT NULL | 首次命中日期 |
| `last_seen_date` | DATE | NOT NULL | 最近命中日期 |
| `hit_count` | INTEGER | NOT NULL DEFAULT 1 | 总命中数 |
| `consecutive_hits` | INTEGER | NOT NULL DEFAULT 0 | 连续命中天数 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

> HNSW 索引 `idx_board_persistent_topics_embedding`：运行时由 `ensurePersistentTopicEmbeddingDimension` 创建（dim ≤ 2000 才建）。

**归属与生命周期（算法语义）**：`daily_report_sections` 通过 `persistent_topic_id` / `topic_match_distance` / `topic_match_confidence` 记录归属。`topic_match_confidence` 四态：`anchor_hit`（双确认命中既有 topic）/ `auto_new`（双确认失败，新开 candidate）/ `unmatched`（section 无 embedding，无法归属）/ `manual`（用户手动建泳道覆盖归属，非算法三态）。`manual` 态由手动建泳道事务写入，前端独立样式区分。`topic_status_at_report` 与归属在同事务写入，不随后续状态回填，历史数据统一 NULL。

**排序与窗口边界**：可锚定话题选择器（`ListAnchorableTopicsByBoard`）选出全部 active 及 `last_seen_date` 在 `persistent_topic_candidate_decay_window`（默认 7 天）内的 candidate，按 `last_seen_date DESC, hit_count DESC, id ASC` 排序，candidate 最多保留 `persistent_topic_candidate_prompt_limit`（默认 20）条。`candidate_decay_window` 仅用于 prompt 卫生过滤，不触发任何状态变更；所有 status → archived 仅由用户在话题管理界面手动操作。

**候选展示门槛**：`consecutive_hits < upgrade_threshold`（默认 3）的 candidate（"observing"）在话题管理 UI 中隐藏，但仍持久化并参与可锚定集合。达门槛后自动可见。

**一次性清理迁移 `20260628_0001`**：幂等硬删所有 `status=candidate AND consecutive_hits < upgrade_threshold` 的历史 candidate。删除采用 `DeleteTopic` 语义：先将引用 section 的 `persistent_topic_id` / `topic_match_distance` / `topic_match_confidence` / `topic_status_at_report` 置 NULL（section 内容保留，仍可渲染为"其他动态"独立节点），再硬删 candidate 行，最后按 board 重建 relations 以清除指向已删 topic 的 identity/similarity 边。不可逆但 section 内容完整保留。第二次执行是 no-op。

### 9.6 board_topic_watches（用户声明的话题 Watch 标签）

板块上用户声明的 Watch 标签。与持久话题刻意独立：无共享 FK、无共享生命周期。Watch 是单信号 AI 检测器，在日报生成结束时触发，不影响任何 topic 状态。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `semantic_board_id` | INTEGER | NOT NULL; index | 所属语义板块 |
| `label` | VARCHAR(200) | NOT NULL | Watch 标签文本 |
| `status` | VARCHAR(20) | NOT NULL DEFAULT 'active'; **CHECK** `chk_board_topic_watches_status (status IN ('active','paused'))` | 状态：`active` / `paused` |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

`Hits` 关联声明 `constraint:OnDelete:CASCADE`（GORM 层）。

### 9.7 topic_watch_hits（Watch 命中记录）

单次 AI 检测到的 Watch 与日报分区的匹配。只读覆盖层，**不得**改变任何 section 的 `persistent_topic_id` 或任何 topic 状态。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `watch_id` | INTEGER | NOT NULL; 复合唯一 `idx_watch_section_report` | 关联 Watch ID |
| `section_id` | INTEGER | NOT NULL; 复合唯一同上 | 命中分区 |
| `report_id` | INTEGER | NOT NULL; 复合唯一同上 | 所属日报 |
| `period_date` | DATE | NOT NULL | 周期日期 |
| `reason` | TEXT | — | 命中理由 |
| `created_at` | TIMESTAMP | — | 创建时间 |

复合唯一索引：`idx_watch_section_report (watch_id, section_id, report_id)`（gorm tag + 迁移 `20260630_0002` 双重声明）。

---

## §10 数据增强域

> 本域 5 张表由 `internal/dataenrichment` 的 `RegisterModels`（`init()`）注册。

### 10.1 board_data_sources（板块数据源绑定）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `semantic_board_id` | BIGINT | NOT NULL; 复合唯一 `idx_board_src` | 所属板块 ID |
| `source_type` | VARCHAR(40) | NOT NULL; 复合唯一同上 | 数据源类型：`etf_quote` / `exchange_rate` / `gdelt_event` |
| `config` | JSONB | DEFAULT '{}'（serializer:json） | 板块级参数，schema 由 source_type 决定 |
| `enabled` | BOOLEAN | NOT NULL DEFAULT true | 是否启用 |
| `created_at` | TIMESTAMPTZ | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | — | 更新时间 |

### 10.2 topic_lifeline_context（话题分层新闻汇总上下文，循环 A）

按 `granularity + period` 存储各周期新闻叙事汇总。档案式存储——历史周期独立保留不覆盖。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `persistent_topic_id` | BIGINT | NOT NULL; 复合唯一 `idx_topic_gran_period` | 持久话题 ID |
| `granularity` | VARCHAR(10) | NOT NULL; 复合唯一同上 | 粒度：`week` / `month` / `year` / `all` |
| `period` | VARCHAR(12) | NOT NULL; 复合唯一同上 | 具体周期（`2026-W27` / `2026-06` / `2026` / `all`） |
| `content` | TEXT | NOT NULL | 新闻叙事汇总 + 数据波动快照 |
| `as_of_date` | DATE | NOT NULL | 汇总截止日（时效判断 + 检查自愈扫描缺口的依据） |
| `source` | VARCHAR(12) | NOT NULL DEFAULT 'manual' | 来源：`manual` / `llm_assisted` |
| `created_at` | TIMESTAMPTZ | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | — | 更新时间 |

### 10.3 topic_enrichment_result（数据增强结果快照，循环 B）

一次增强一行，**不可变**——存档不修改，确保 review 有对比基准。不含 `report_id`（循环 B 不挂日报管线）。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `persistent_topic_id` | BIGINT | NOT NULL; index | 持久话题 ID |
| `evolution_assessment` | TEXT | — | 分析员结论：演进评估文本 |
| `sectors` | JSONB | — | 受影响产业方向 JSON：`[{sector, evolution_role, current_signal, vs_history, judgment, confidence}]` |
| `causal_chain` | TEXT | — | 因果链文本 |
| `tool_calls` | JSONB | — | 工具调用记录（名/参数/返回摘要/耗时） |
| `input_snapshot` | JSONB | — | 编排元数据（读的 context 层 / as_of / section 范围 / 引用 review ID） |
| `session_id` | VARCHAR(120) | — | 编排分组键，关联 `ai_call_logs.session_id` |
| `created_at` | TIMESTAMPTZ | — | 创建时间 |

### 10.4 topic_enrichment_review（数据增强认知演进反思）

两次 result 快照间的偏差记录，追加写入。`applied` 不回写 result 表，仅标记认知已纳入。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `persistent_topic_id` | BIGINT | NOT NULL; index | 持久话题 ID |
| `prev_result_id` | BIGINT | —（`*uint`）; index | 上次 result ID（可空，手动批注时无 prev 对比） |
| `curr_result_id` | BIGINT | NOT NULL; index | 本次 result ID |
| `verdict` | JSONB | — | 兑现度逐条结算：`[{sector, predicted_dir, actual, mark:"hit" | "part" | "miss"}]` |
| `deviation_summary` | TEXT | NOT NULL | 偏差说明（LLM 基底 + 人工可调） |
| `affected_context` | VARCHAR(10) | — | 建议关注的粒度层：`week` / `month` / `year` |
| `confidence` | REAL | —（`*float64`） | review_judge 置信度 |
| `applied` | BOOLEAN | NOT NULL DEFAULT false | 用户采纳标记（不回写 result，仅标示认知已纳入） |
| `source` | VARCHAR(12) | NOT NULL DEFAULT 'llm_assisted' | 来源：`llm_assisted` / `manual` |
| `created_at` | TIMESTAMPTZ | — | 创建时间 |
| `updated_at` | TIMESTAMPTZ | — | 更新时间 |

### 10.5 stock_debate_result（FinGenius 个股辩论结果）

FinGenius 多角色辩论输出，按 `(result_id, sector, code)` 维度 append-only（同一 result 内可多次辩论，最新覆盖）。独立存档，不回写前述三表。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK | 主键 |
| `topic_enrichment_result_id` | BIGINT | NOT NULL; index `idx_stock_debate_result_id` | 关联的增强结果 ID |
| `persistent_topic_id` | BIGINT | NOT NULL; index `idx_stock_debate_topic` | 持久话题 ID（冗余，便于按 topic 查） |
| `sector` | VARCHAR(80) | NOT NULL | 关联分析员输出的 sector 名 |
| `code` | VARCHAR(20) | NOT NULL | 标的代码（如 `161129`） |
| `name` | VARCHAR(60) | — | 标的名称 |
| `verdict` | VARCHAR(8) | NOT NULL | 综合结论：`up` / `down` / `flat` |
| `consensus` | VARCHAR(12) | — | 共识度文本（如 `"4/6"`） |
| `agents` | JSONB | — | 各 agent 立场提炼：`[{role, stance, note, raw_vote}]` |
| `votes` | JSONB | — | 三档统计：`{up:N, flat:N, down:N}` |
| `fingenius_research` | JSONB | — | FinGenius 原始研究输出（提炼失败时降级展示） |
| `fingenius_battle` | JSONB | — | FinGenius 原始辩论输出 |
| `fingenius_task_id` | VARCHAR(120) | — | FinGenius 异步任务 ID |
| `distill_status` | VARCHAR(12) | NOT NULL DEFAULT 'done' | 提炼状态：`done` / `failed` / `skipped` |
| `html_content` | TEXT | — | FinGenius 完整 HTML 报告字符串（前端 iframe `srcdoc` 渲染） |
| `created_at` | TIMESTAMPTZ | — | 创建时间 |

**字段分工**：提炼后字段（`verdict` / `consensus` / `agents` / `votes`）由 Syntopica LLM `debate_distill` 产出；原始字段（`fingenius_research` / `fingenius_battle`）为 FinGenius 原始输出，提炼失败时降级展示原文。

---

## §11 用户行为域

### 11.1 reading_behaviors（阅读行为）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `article_id` | INTEGER | NOT NULL; index | 文章 ID（逻辑关联，无 OnDelete） |
| `feed_id` | INTEGER | index | 订阅源 ID |
| `category_id` | INTEGER | —（`*uint`）; index | 分类 ID |
| `session_id` | VARCHAR(100) | index | 会话 ID |
| `event_type` | VARCHAR(20) | index | 事件类型 |
| `scroll_depth` | INTEGER | DEFAULT 0 | 滚动深度 |
| `reading_time` | INTEGER | DEFAULT 0 | 阅读时间 |
| `created_at` | TIMESTAMP | index | 创建时间 |

> `feed_id` / `created_at` 均为各自单列索引（非复合）。

### 11.2 user_preferences（用户偏好）

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | SERIAL | PK | 主键 |
| `feed_id` | INTEGER | —（`*uint`）; index | 订阅源 ID |
| `category_id` | INTEGER | —（`*uint`）; index | 分类 ID |
| `preference_score` | FLOAT | DEFAULT 0 | 偏好评分 |
| `avg_reading_time` | INTEGER | DEFAULT 0 | 平均阅读时间 |
| `interaction_count` | INTEGER | DEFAULT 0 | 交互次数 |
| `scroll_depth_avg` | FLOAT | DEFAULT 0 | 平均滚动深度 |
| `last_interaction_at` | TIMESTAMP | —（`*time.Time`）; index | 最后交互时间 |
| `created_at` | TIMESTAMP | — | 创建时间 |
| `updated_at` | TIMESTAMP | — | 更新时间 |

---

## §12 链路追踪域

### 12.1 otel_spans（OpenTelemetry 链路追踪）

存储 GORM Span Exporter 导出的 OpenTelemetry span 数据，通过独立 `EnsureTracingTable` AutoMigrate 创建。

| 字段名 | 类型 | 约束/默认/索引 | 用途 |
| -------- | ------ | ------ | ------ |
| `id` | BIGSERIAL | PK autoIncrement | 主键 |
| `trace_id` | CHAR(32) | NOT NULL; index `idx_otel_spans_trace_id` | 追踪 ID |
| `span_id` | CHAR(16) | NOT NULL | Span ID |
| `parent_span_id` | CHAR(16) | DEFAULT '' | 父 Span ID |
| `trace_state` | TEXT | DEFAULT '' | W3C trace state |
| `name` | VARCHAR(255) | NOT NULL; index `idx_otel_spans_name` | Span 名称 |
| `kind` | INTEGER | DEFAULT 1; index `idx_otel_spans_kind` | Span 类型（Internal=1, Server=2, Client=3, Producer=4, Consumer=5） |
| `status_code` | INTEGER | DEFAULT 0; index `idx_otel_spans_status` | 状态码（0=Unset, 1=Error, 2=OK） |
| `status_message` | TEXT | DEFAULT '' | 状态信息 |
| `start_time_unix_nano` | BIGINT | NOT NULL; index `idx_otel_spans_start_time` | 开始时间（Unix 纳秒） |
| `end_time_unix_nano` | BIGINT | NOT NULL | 结束时间（Unix 纳秒） |
| `duration_ms` | BIGINT | DEFAULT 0 | 持续时间（毫秒） |
| `service_name` | VARCHAR(100) | DEFAULT 'syntopica' | 服务名称 |
| `service_version` | VARCHAR(50) | DEFAULT '' | 服务版本 |
| `resource_attributes` | TEXT | DEFAULT '{}' | 资源属性（JSON） |
| `scope_name` | VARCHAR(100) | DEFAULT '' | Scope 名称 |
| `scope_version` | VARCHAR(50) | DEFAULT '' | Scope 版本 |
| `attributes` | TEXT | DEFAULT '{}' | Span 属性（JSON） |
| `events` | TEXT | DEFAULT '[]' | Span 事件（JSON） |
| `links` | TEXT | DEFAULT '[]' | Span 链接（JSON） |
| `created_at` | TIMESTAMP | — | 创建时间 |

---

## §13 已废弃 / 预留表（无对应 model）

以下表当前无 Go 代码引用，保留标注以避免误用。

### 13.1 ai_summaries / ai_summary_feeds / ai_summary_topics

对应旧版 Feed 级 AI 批量摘要功能，模型已从 `internal/models/` 移除。**数据库中可能存有旧数据**。

> ⚠️ 旧文档曾称「`articles.feed_summary_id` 仍然指向 `ai_summaries.id`」：**代码 `Article` struct 无 `feed_summary_id` 字段**，此关联已不存在。

字段（仅供历史参考）：

- `ai_summaries`：`id`(BIGSERIAL PK) / `feed_id` / `category_id` / `title`(VARCHAR 200) / `summary`(TEXT) / `key_points` / `articles` / `article_count` / `time_range` / `created_at` / `updated_at`
- `ai_summary_feeds`：`id` / `summary_id` / `feed_id` / `feed_title` / `feed_icon` / `feed_color` / `article_count` / `created_at`
- `ai_summary_topics`：`id` / `summary_id` / `topic_tag_id` / `score` / `source` / `created_at`

### 13.2 topic_analysis_jobs（主题分析任务队列，已废弃）

无 migrator 注册。字段（历史参考）：`id`(VARCHAR(64) PK) / `topic_tag_id` / `analysis_type` / `window_type` / `anchor_date` / `priority` / `status` / `retry_count` / `error_message` / `progress` / `created_at` / `started_at` / `completed_at`。

### 13.3 digest_configs（Digest 推送配置，预留）

无 Go 代码引用，0 行数据。字段（历史参考）：`id` / `daily_enabled` / `daily_time` / `weekly_enabled` / `weekly_day` / `weekly_time` / `feishu_enabled` / `feishu_webhook_url` / `feishu_push_summary` / `feishu_push_details` / `obsidian_enabled` / `obsidian_vault_path` / `obsidian_daily_digest` / `obsidian_weekly_digest` / `created_at` / `updated_at`。

---

## §14 框架表

### schema_migrations

迁移版本追踪表，由 GORM 迁移框架管理，不计入业务表。

---

## 字段用途说明：文章三个内容字段

| 字段 | 来源 | 格式 | 特点 | 用途 |
| -------- | ------ | ------ | ------ | ------ |
| `content` | RSS Feed 解析 | HTML 片段 | 可能不完整，含 HTML 标签 | 基础内容展示 |
| `firecrawl_content` | Firecrawl 抓取 | Markdown | 完整网页内容，过滤广告/导航栏 | AI 总结输入源，不对用户直接展示 |
| `ai_content_summary` | AI 生成 | Markdown | 保留核心，移除冗余 | 前端默认展示内容 |

---

## 索引与约束总览

### pgvector 扩展（迁移 `20260403_0001`）

`CREATE EXTENSION IF NOT EXISTS vector`

### 全文检索（迁移 `20260417_0002`）

- `articles.search_vector`（tsvector）+ GIN 索引 `idx_articles_search_vector` + 触发器 `articles_search_vector_trigger`（BEFORE INSERT OR UPDATE OF title, description）。

### 性能索引（迁移 `20260417_0001`）

| 索引名 | 表 | 列 |
| -------- | ------ | ------ |
| `idx_articles_read` | articles | `(read)` |
| `idx_articles_favorite` | articles | `(favorite)` |
| `idx_articles_feed_pub_date` | articles | `(feed_id, pub_date DESC)` |
| `idx_articles_feed_id_title` | articles | `(feed_id, title)` |
| `idx_article_topic_tags_article_id` | article_topic_tags | `(article_id)` |
| `idx_feeds_category_id` | feeds | `(category_id)` |

### 向量唯一索引

| 索引名 | 表 | 列 | 迁移 |
| -------- | ------ | ------ | ------ |
| `idx_topic_tag_embeddings_tag_type_hash` | topic_tag_embeddings | UNIQUE `(topic_tag_id, embedding_type, text_hash)` | `20260514_0001` |

> 注：`topic_tag_embeddings.embedding` 无独立 HNSW 迁移语句；运行时 HNSW 仅作用于 `daily_report_sections.embedding` 与 `board_persistent_topics.embedding`（dim ≤ 2000 才建）。

### 语义标签 / 板块域索引（迁移 `20260521_0001`）

| 索引名 | 表 | 列 |
| -------- | ------ | ------ |
| `idx_semantic_labels_slug` | semantic_labels | UNIQUE `(slug)` |
| `idx_semantic_labels_label_type` | semantic_labels | `(label_type)` |
| `idx_semantic_labels_status` | semantic_labels | `(status)` |
| `idx_topic_tag_semantic_labels_topic_tag_id` | topic_tag_semantic_labels | `(topic_tag_id)` |
| `idx_topic_tag_semantic_labels_semantic_label_id` | topic_tag_semantic_labels | `(semantic_label_id)` |
| `idx_topic_tag_board_labels_topic_tag_id` | topic_tag_board_labels | `(topic_tag_id)` |
| `idx_topic_tag_board_labels_semantic_board_id` | topic_tag_board_labels | `(semantic_board_id)` |
| `idx_board_composition_board_id` | board_composition | `(board_id)` |
| `idx_board_composition_auxiliary_label_id` | board_composition | `(auxiliary_label_id)` |
| `idx_narrative_boards_semantic_board_id` | narrative_boards | `(semantic_board_id)` |

### 叙事域索引（迁移 `20260420_0001` / `20260430_0001`）

| 索引名 | 表 | 列 |
| -------- | ------ | ------ |
| `idx_narrative_scope` | narrative_summaries | `(scope_category_id)` |
| `idx_narrative_scope_period` | narrative_summaries | `(scope_type, scope_category_id, period_date)` |
| `idx_narrative_summaries_board_id` | narrative_summaries | `(board_id)` |
| `idx_narrative_boards_period` | narrative_boards | `(period_date)` |
| `idx_narrative_boards_scope` | narrative_boards | `(scope_category_id)` |

### AI 调用日志索引（迁移 `20260704_0001`）

| 索引名 | 表 | 列 |
| -------- | ------ | ------ |
| `idx_call_logs_session` | ai_call_logs | `(session_id)` |
| `idx_call_logs_op_time` | ai_call_logs | `(operation, created_at)` |

### 日报 / 持久话题 / Watch 域索引

| 索引名 | 表 | 列 | 迁移 |
| -------- | ------ | ------ | ------ |
| `idx_board_daily_reports_semantic_board_id` | board_daily_reports | `(semantic_board_id)` | `20260526_0001` |
| `idx_daily_report_sections_report_id` | daily_report_sections | `(report_id)` | `20260526_0001` |
| `idx_daily_report_threads_report_id` | daily_report_threads | `(report_id)` | `20260529_0001` |
| `idx_daily_report_threads_section_id` | daily_report_threads | `(section_id)` | `20260529_0001` |
| `idx_section_relations_from` | daily_report_section_relations | `(from_section_id)` | gorm tag |
| `idx_section_relations_to` | daily_report_section_relations | `(to_section_id)` | gorm tag |
| `idx_section_relations_type` | daily_report_section_relations | `(relation_type)` | `20260619_0001` |
| `idx_persistent_topics_board_status` | board_persistent_topics | `(semantic_board_id, status)` | `20260619_0001` |
| `idx_watch_section_report` | topic_watch_hits | UNIQUE `(watch_id, section_id, report_id)` | `20260630_0002` |
| `idx_board_upgrade_suggestions_status` | board_upgrade_suggestions | `(status)` | `20260717_0001` |

### 唯一约束（表级）

| 约束名 | 表 | 列 |
| -------- | ------ | ------ |
| `uq_section_relations_pair` | daily_report_section_relations | UNIQUE `(from_section_id, to_section_id, relation_type)` |
| `uq_board_upgrade_suggestions_hash` | board_upgrade_suggestions | 部分唯一 `(suggestion_hash) WHERE status='pending'` |

### CHECK 约束（迁移添加，DB 层强制）

| 约束名 | 表 | 表达式 | 迁移 |
| -------- | ------ | ------ | ------ |
| `chk_board_persistent_topics_status` | board_persistent_topics | `status IN ('candidate','active','archived')` | `20260619_0001` |
| `chk_board_persistent_topics_source` | board_persistent_topics | `source IN ('auto','manual')` | `20260702_0001` |
| `chk_board_topic_watches_status` | board_topic_watches | `status IN ('active','paused')` | `20260630_0001` |

### DB 级外键（全库唯一）

| 约束名 | 表.列 | 引用 | 行为 | 迁移 |
| -------- | ------ | ------ | ------ | ------ |
| `topic_tags_merged_into_id_fkey` | `topic_tags.merged_into_id` | `topic_tags(id)` | ON DELETE CASCADE | `20260601_0001` |

> 其余所有表间关联均为 GORM 逻辑关联，**DB 层未强制**（`DisableForeignKeyConstraintWhenMigrating: true`）。

---

## 向量维度规则总述

| 表.列 | 维度来源 | 说明 |
| -------- | ------ | ------ |
| `topic_tag_embeddings.embedding` | **迁移固定 4096** | 唯一维度固定的向量列（迁移 `20260403_0003`） |
| `semantic_labels.embedding` | 运行时 | `auxlabel.EnsureVectorDimensionOnce` 按 `embedding_config.embedding_dimension` 设置 |
| `semantic_labels.merge_embedding` | 运行时 | 同上 |
| `daily_report_sections.embedding` | 运行时 | `ensureSectionEmbeddingDimension` |
| `daily_report_threads.embedding` | 运行时 | （随分区维度） |
| `board_persistent_topics.embedding` | 运行时 | `ensurePersistentTopicEmbeddingDimension` |

> HNSW 索引仅当维度 ≤ 2000 时创建（pgvector 限制）；> 2000 时跳过 HNSW，仅保留向量列。

---

## 更新日志

### 2026-07-18（全面对齐代码审计）

- 表清单从 40 张修正为 **43 张业务表** + 5 张废弃表 + 1 张框架表（与 `migrator.go` 注册清单一致）。
- **补 10 张缺失/半缺表的完整字段表**：`board_upgrade_suggestions`、`board_daily_reports`、`daily_report_sections`、`daily_report_threads`、`daily_report_section_relations`、`board_persistent_topics`（补完整）、`board_topic_watches`、`topic_watch_hits`、`topic_tag_relations`、`tag_merge_suggestions`。
- **删幽灵字段**：`articles.feed_summary_id` / `feed_summary_generated_at`、`topic_tags.concept_id`、`topic_tag_embeddings.vector`。
- **修正凭空字段**：`feeds.ai_summary_enabled` → 实际 `article_summary_enabled`(default false)；`topic_tag_analyses.summary_count` → `article_count`；`topic_analysis_cursors.last_summary_id` → `last_article_id`。
- **补漏字段**：`topic_tags.metadata`、`feeds.tagging_enabled`、`ai_providers.enable_thinking`、`ai_routes.priority` / `max_concurrency`、`semantic_labels.merge_embedding`、`topic_tag_board_labels.downgraded` / `direction_mismatch`、`articles.search_vector`（FTS）。
- **修正中间表主键**：`topic_tag_semantic_labels` / `topic_tag_board_labels` / `board_composition` 均无 `id`，改为复合主键。
- **修正约束**：`topic_tag_embeddings.embedding` 维度 1536 → **4096**（迁移固定）；`semantic_labels.slug` size 120 → **160**、唯一约束 `(label_type,slug)` → **单列 slug**；`topic_tags (category,slug)` 由「唯一」改为「普通复合索引」。
- **枚举对齐**：`narrative_summaries.scope_type` 加 `board`；`daily_report_sections.topic_match_confidence` 加 `manual`；各新表枚举补全。
- **新增 CHECK 约束章节**：3 个 CHECK + 唯一 DB FK 说明。
- 重写「索引与约束总览」，按真实迁移/gorm tag 列出所有索引。
- 新增「向量维度规则总述」与「阅读约定」全局章节。

### 2026-07-07

- 新增 `stock_debate_result` 表（FinGenius 个股辩论结果）。
- `topic_lifeline_context` 补 `period` 字段，唯一约束修正为 `(topic_id, granularity, period)`。
- `topic_enrichment_review` 补 `verdict` 字段。

### 2026-07-06

- 新增 4 张数据增强相关表。
- `semantic_labels` 新增 `enrichment_enabled` / `window_days` / `context_layers`。

### 2026-07-04

- `board_persistent_topics` 新增 `source` 字段（+ CHECK）。
- `daily_report_sections.topic_match_confidence` 新增 `manual`。

### 2026-05-22

- 语义标签/板块体系重构：移除 hierarchy/board_concepts 相关表，新增 semantic_labels 等四张表。

### 2026-05-14

- `topic_tag_embeddings` 新增 `embedding_type`，唯一约束改为三列。

### 2026-04-16

- 全面重写，覆盖所有表的完整字段说明。

### 2026-03-05

- 创建本文档。

---

## 相关文档

- [全局实体关系图](ER_DIAGRAM.md) — 表关系图（ASCII + Mermaid）和约束矩阵
- [数据生命周期](DATA_LIFECYCLE.md) — 数据链路的状态字段流转说明
- [业务流程](../flow/README.md) — 链路概要设计、函数调用链、前后端协作
