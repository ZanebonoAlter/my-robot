# 数据库文档索引

Syntopica 数据库全景概览与索引/约束权威清单。

> **真相源 = 代码**：GORM struct gorm tag（`internal/models/*.go`、`internal/dataenrichment/repository/models.go`、`internal/topicgraph/repository/daily_report_models.go`、`internal/platform/tracing/model.go`）+ 迁移 DDL（`internal/platform/database/postgres_migrations.go`）+ 运行时建索引代码。本页据此重写。
>
> **迁移执行器能力**（`internal/platform/database/migrator.go`）：`Migration` 结构体除 `Version/Description/Up` 外，支持 `RunOutsideTx`（事务外执行，为 `CREATE INDEX CONCURRENTLY` 解锁）、`Down`（声明性占位，nil = 不可逆）；长锁 DDL（`ALTER TYPE`/`ADD CONSTRAINT UNIQUE`）用 `withLockTimeout` 守卫防大表无限阻塞。编写规范见 [`standard/backend/code-style.md`](../standard/backend/code-style.md)「迁移编写规范」。

---

## 概览

| 指标 | 值 | 说明 |
| ------ | ----- | ------ |
| 真实业务表数 | **47** | 34 Core + 5 DataEnrichment + 7 TopicGraph 日报域 + 1 Tracing（见下方清单，不含 `schema_migrations`） |
| DB 级 FK 约束 | **1** | 仅 `topic_tags_merged_into_id_fkey`（ON DELETE CASCADE，迁移 `20260601_0001` 重建）。GORM 关闭了外键迁移（`DisableForeignKeyConstraintWhenMigrating: true`），且该迁移主动 drop 了历史上的全部 `fk_*`。其余表间关系均为 **GORM 逻辑关联，DB 层不强制**。 |
| CHECK 约束 | **3** | `chk_board_persistent_topics_status` / `chk_board_persistent_topics_source` / `chk_board_topic_watches_status`（见下） |
| 业务域 | **7** | Core 文章流、Topic Tags 图谱、Semantic Labels/Board、AI Infrastructure、Narrative 叙事、Daily Report 日报域、DataEnrichment 数据增强 |
| 枢纽表 | `topic_tags` | 被多表引用（`article_topic_tags` / `topic_tag_embeddings` / `topic_tag_semantic_labels` / `topic_tag_board_labels` / `topic_tag_analyses` / `topic_tag_relations` / `embedding_queues` / `merge_reembedding_queues` 等） |
| 向量表（pgvector） | `topic_tag_embeddings`(固定 vector(4096))、`semantic_labels`(embedding + merge_embedding)、`daily_report_sections`、`daily_report_threads`、`board_persistent_topics`、`preference_vectors`、`route_embeddings` | 维度运行时决定，HNSW 索引仅在维度 ≤ 2000 时创建（`preference_vectors`/`route_embeddings` 同维不强制 HNSW，粗筛走顺序扫描） |
| 全文搜索 | `articles.search_vector` | GIN 索引 + 触发器 `articles_search_vector_trigger` |
| 向量缓存（非 pgvector） | `ai_embedding_cache.embedding` | bytea 二进制（float32 小端，`models/embedding_codec.go` 编解码，~10KB/2560维，仅字节回读不做相似度检索）。jsonb→bytea 由启动时 pre-migrate 非破坏转换（optimize-pg-storage，须先于 AutoMigrate 避免 GORM ALTER 报 cannot cast） |
| 预留/已废弃表 | 4 | `ai_summaries` / `ai_summary_feeds` / `ai_summary_topics` / `topic_analysis_jobs` / `digest_configs`：无对应 model、无 Go 代码引用 |

---

## 真实表清单（43 张，按代码权威对齐）

**Core（`migrator.go` allModels，33 张）**：
`categories` `feeds` `articles` `topic_tags` `semantic_labels` `topic_tag_semantic_labels` `topic_tag_board_labels` `board_composition` `board_upgrade_suggestions` `topic_tag_embeddings` `topic_tag_analyses` `topic_analysis_cursors` `article_topic_tags` `tag_merge_suggestions` `topic_tag_relations` `scheduler_tasks` `ai_settings` `embedding_config` `embedding_queues` `merge_reembedding_queues` `ai_providers` `ai_routes` `ai_route_providers` `ai_call_logs` `reading_behaviors` `firecrawl_jobs` `tag_jobs` `narrative_summaries` `narrative_boards` `preference_vectors` `rsshub_routes` `route_embeddings` `feed_recommendations` `route_param_options`

> 旧 `user_preferences` 表已删除（preference-vector-feed-discovery，偏好转向向量画像，迁移 `20260725_0001` DROP）。

**DataEnrichment（RegisterModels，5 张）**：
`board_data_sources` `topic_lifeline_context` `topic_enrichment_result` `topic_enrichment_review` `stock_debate_result`

**TopicGraph 日报域（RegisterModels，7 张）**：
`board_daily_reports` `daily_report_sections` `daily_report_threads` `daily_report_section_relations` `board_persistent_topics` `board_topic_watches` `topic_watch_hits`

**Tracing（独立 AutoMigrate，1 张）**：
`otel_spans`

> 框架表 `schema_migrations` 不计入业务表。

---

## 索引与约束清单

> 索引来源标注：**[gorm]**=AutoMigrate 由 gorm tag 建；**[migr]**=迁移 DDL 显式建；**[runtime]**=启动时按向量维度建。带 `UNIQUE` 为唯一索引，`PARTIAL` 为部分索引。

### 文章与订阅域

| 表 | 索引/约束 |
| ---- | ----------- |
| `articles` | `idx_articles_feed_id(feed_id)` [gorm]；`idx_articles_feed_pub_date(feed_id, pub_date DESC)` [migr] **复合**；`idx_articles_feed_id_title(feed_id, title)` [migr] **复合**；`idx_articles_read(read)` [migr]；`idx_articles_favorite(favorite)` [migr]；`idx_articles_search_vector` GIN [migr] + 触发器 `articles_search_vector_trigger` |
| `feeds` | `idx_feeds_category_id(category_id)` [migr]；`idx_feeds_category_id(category_id)` [gorm]（AutoMigrate 同名） |
| `categories` | （无业务索引） |
| `reading_behaviors` | 单列索引各一 [gorm]：`idx_reading_behaviors_article_id`、`idx_reading_behaviors_feed_id`、`idx_reading_behaviors_category_id`、`idx_reading_behaviors_session_id`、`idx_reading_behaviors_event_type`、`idx_reading_behaviors_created_at` |

> **勘误**：旧文档列的复合索引 `idx_reading_behaviors_feed_created_at` **不存在**，实际是 feed_id、created_at 两个独立单列索引。
>
> 旧 `user_preferences` 表已删除（偏好转向向量画像），其索引随表一同 DROP。

### Topic Tags 图谱域

| 表 | 索引/约束 |
| ---- | ----------- |
| `topic_tags` | `idx_topic_tags_category_slug(category, slug)` [gorm] **普通复合，非唯一**；`idx_topic_tags_status(status)` [gorm]；`idx_topic_tags_merged_into_id(merged_into_id)` [gorm]；**FK** `topic_tags_merged_into_id_fkey → topic_tags.id ON DELETE CASCADE` [migr]（**全库唯一真实 DB FK**） |
| `topic_tag_embeddings` | `idx_topic_tag_embeddings_tag_type_hash(topic_tag_id, embedding_type, text_hash)` **UNIQUE** [gorm+migr]；`idx_topic_tag_embeddings_embedding` HNSW [runtime，dim≤2000]；**固定列维度 `embedding vector(4096)`**（迁移 `20260601_0001a`） |
| `article_topic_tags` | 复合主键 `(article_id, topic_tag_id)`；单列索引 `idx_article_topic_tag_topic(topic_tag_id)` [gorm]、`idx_article_topic_tag_article(article_id)` [gorm]；`idx_article_topic_tags_article_id(article_id)` [migr] |
| `topic_tag_relations` | `idx_tag_relation_pair(parent_id, child_id)` **UNIQUE** [gorm] |
| `tag_merge_suggestions` | `idx_tag_merge_suggestion_pair(new_tag_id, existing_tag_id)` **UNIQUE** [gorm]；`idx_tag_merge_suggestion_status_sim(status, similarity)` [gorm] |
| `topic_tag_analyses` | `idx_tag_analysis_date(topic_tag_id, analysis_type, window_type, anchor_date)` **UNIQUE** [gorm] |
| `topic_analysis_cursors` | `idx_cursor_tag_type_window(topic_tag_id, analysis_type, window_type)` **UNIQUE** [gorm] |

> **勘误**：旧文档称 `idx_article_topic_tags_topic_article(topic_tag_id, article_id)` 复合索引 **不存在**，实际是 `idx_article_topic_tag_topic` 单列。另 `topic_tags.(category, slug)` 为普通复合索引，**不强制唯一**。

### Semantic Labels / Board 域

| 表 | 索引/约束 |
| ---- | ----------- |
| `semantic_labels` | `idx_semantic_labels_slug(slug)` **UNIQUE 单列** [gorm+migr]；`idx_semantic_labels_label_type(label_type)` [gorm+migr]；`idx_semantic_labels_status(status)` [gorm+migr] |
| `topic_tag_semantic_labels` | 复合主键 `(topic_tag_id, semantic_label_id)`，无 id；`idx_topic_tag_semantic_labels_topic_tag_id(topic_tag_id)` [migr]；`idx_topic_tag_semantic_labels_semantic_label_id(semantic_label_id)` [migr] |
| `topic_tag_board_labels` | 复合主键 `(topic_tag_id, semantic_board_id)`，无 id；`idx_topic_tag_board_labels_topic_tag_id(topic_tag_id)` [migr]；`idx_topic_tag_board_labels_semantic_board_id(semantic_board_id)` [migr] |
| `board_composition` | 复合主键 `(board_id, auxiliary_label_id)`，无 id；`idx_board_composition_board_id(board_id)` [migr]；`idx_board_composition_auxiliary_label_id(auxiliary_label_id)` [migr] |
| `board_upgrade_suggestions` | `uq_board_upgrade_suggestions_hash(suggestion_hash)` **PARTIAL UNIQUE WHERE status='pending'** [migr]；`idx_board_upgrade_suggestions_status(status)` [gorm+migr] |

> **勘误**：旧文档称 `semantic_labels` 唯一约束为 `(label_type, slug)` **错误**，实际是 `slug` 单列唯一。

### AI Infrastructure 域

| 表 | 索引/约束 |
| ---- | ----------- |
| `ai_call_logs` | `idx_call_logs_session(session_id)` [migr]；`idx_call_logs_op_time(operation, created_at)` **复合** [migr]；`idx_ai_call_logs_created_at(created_at)` [gorm]；单列 [gorm]：capability、success |
| `scheduler_tasks` | 单列 [gorm]：name、status |
| `ai_settings` | 单列 [gorm]：key |
| `ai_providers` | 单列 [gorm]：name、provider_type、enabled |
| `ai_routes` | `idx_ai_routes_capability_name(name, capability)` **UNIQUE 复合** [gorm]；单列 [gorm]：capability、enabled、priority |
| `ai_route_providers` | `idx_ai_route_provider_link(route_id, provider_id)` **UNIQUE 复合** [gorm]；单列 [gorm]：priority、enabled |
| `embedding_config` | 单列 [gorm]：key |

### 队列域

| 表 | 索引/约束 |
| ---- | ----------- |
| `firecrawl_jobs` | 单列索引各一 [gorm]：article_id、status、priority、available_at、lease_expires_at（**均为单列，非复合**） |
| `tag_jobs` | 单列索引各一 [gorm]：article_id、status、priority、available_at、lease_expires_at（**均为单列，非复合**） |
| `embedding_queues` | 单列 [gorm]：tag_id、status |
| `merge_reembedding_queues` | 单列 [gorm]：source_tag_id、target_tag_id、status |

> **勘误**：旧文档称 `idx_firecrawl_jobs_status_available_at` / `idx_tag_jobs_status_available_at` 复合索引 **不存在**，实际是 status、available_at 等各自单列索引。状态流转与清理见 [DATA_LIFECYCLE.md](DATA_LIFECYCLE.md#数据清理与保留策略)。

### Narrative 叙事域

| 表 | 索引/约束 |
|----|-----------|
| `narrative_summaries` | `idx_narrative_scope(scope_category_id)` [migr]；`idx_narrative_scope_period(scope_type, scope_category_id, period_date)` **复合** [migr]；`idx_narrative_summaries_board_id(board_id)` [migr]；`idx_narrative_period_date(period_date)` [gorm]；单列 [gorm]：status、board_id |
| `narrative_boards` | `idx_narrative_boards_period(period_date)` [gorm+migr]；`idx_narrative_boards_scope(scope_category_id)` [gorm+migr]；`idx_narrative_boards_semantic_board_id(semantic_board_id)` [migr] |

### 偏好向量与订阅源发现域（preference-vector-feed-discovery）

| 表 | 索引/约束 |
| ---- | ----------- |
| `preference_vectors` | `idx_preference_vectors_board_source(board_id, source)` **UNIQUE** [gorm]（board_id 允许多 NULL，全局桶单行由 service 层 upsert 保证）；逻辑关联 `semantic_labels`(board_id)；向量列 `embedding vector`（运行时维度，无固定列维度） |
| `rsshub_routes` | `idx_rsshub_routes_ns_path(namespace, path)` **UNIQUE** [gorm]；单列 [gorm]：`content_hash`、`status` |
| `route_embeddings` | `idx_route_embeddings_route(route_id)` **UNIQUE** [gorm]；单列 [gorm]：`text_hash`；逻辑关联 `rsshub_routes`(route_id, OnDelete CASCADE)；向量列 `embedding vector` |
| `feed_recommendations` | `idx_feed_recommendations_hash(recommendation_hash)` **UNIQUE** [gorm]；复合 `idx_feed_rec_status(status, score)` [gorm]；单列 [gorm]：`route_id`、`board_id`、`accepted_feed_id`；逻辑关联 `rsshub_routes`(route_id) / `semantic_labels`(board_id) / `feeds`(accepted_feed_id) |
| `route_param_options` | `idx_route_param_option_uniq(route_id, param_name, value)` **UNIQUE** [gorm]；单列 [gorm]：`route_id`；逻辑关联 `rsshub_routes`(route_id, OnDelete CASCADE) |

> `recommendation_hash = hash(route_id + board_id)`，**不含 source**，qa 与 manual_refresh 共享幂等池与 dismiss 冷却池（见 `flow/discovery.md`）。

### Daily Report 日报域

| 表 | 索引/约束 |
| ---- | ----------- |
| `board_daily_reports` | `idx_board_daily_reports_semantic_board_id(semantic_board_id)` [migr]；单列 [gorm]：semantic_board_id |
| `daily_report_sections` | `idx_daily_report_sections_report_id(report_id)` [migr]；`idx_daily_report_sections_embedding` HNSW [runtime，dim≤2000]；单列 [gorm]：persistent_topic_id |
| `daily_report_threads` | `idx_daily_report_threads_report_id(report_id)` [migr]；`idx_daily_report_threads_section_id(section_id)` [migr]（同名列 [gorm]） |
| `daily_report_section_relations` | `uq_section_relations_pair(from_section_id, to_section_id, relation_type)` **UNIQUE 三列** [migr]；`idx_section_relations_from(from_section_id)` [gorm]；`idx_section_relations_to(to_section_id)` [gorm]；`idx_section_relations_type(relation_type)` [gorm+migr] |
| `board_persistent_topics` | `idx_persistent_topics_board_status(semantic_board_id, status)` **复合** [gorm+migr]；`idx_board_persistent_topics_embedding` HNSW [runtime，dim≤2000]；**CHECK** ×2（见下） |
| `board_topic_watches` | 单列 [gorm]：semantic_board_id；**CHECK** ×1（见下） |
| `topic_watch_hits` | `idx_watch_section_report(watch_id, section_id, report_id)` **UNIQUE 复合** [gorm+migr] |

### DataEnrichment 数据增强域

| 表 | 索引/约束 |
| ---- | ----------- |
| `board_data_sources` | `idx_board_src(semantic_board_id, source_type)` **UNIQUE 复合** [gorm] |
| `topic_lifeline_context` | `idx_topic_gran_period(persistent_topic_id, granularity, period)` **UNIQUE 复合** [gorm] |
| `topic_enrichment_result` | 单列 [gorm]：persistent_topic_id |
| `topic_enrichment_review` | 单列 [gorm]：persistent_topic_id、prev_result_id、curr_result_id |
| `stock_debate_result` | 单列 [gorm]：topic_enrichment_result_id、persistent_topic_id |

### Tracing

| 表 | 索引/约束 |
|----|-----------|
| `otel_spans` | 单列 [gorm]：`idx_otel_spans_trace_id`、`idx_otel_spans_name`、`idx_otel_spans_kind`、`idx_otel_spans_status`、`idx_otel_spans_start_time` |

---

## CHECK 约束（文档历史零覆盖，现补全）

| 约束名 | 表.列 | 取值 | 出处迁移 |
| -------- | ------- | ------ | --------- |
| `chk_board_persistent_topics_status` | `board_persistent_topics.status` | `IN ('candidate','active','archived')` | `20260619_0001` |
| `chk_board_persistent_topics_source` | `board_persistent_topics.source` | `IN ('auto','manual')` | `20260702_0001` |
| `chk_board_topic_watches_status` | `board_topic_watches.status` | `IN ('active','paused')` | `20260630_0001` |

> 代码中另有大量**约定型枚举**（如 `embedding_queues.status`、`narrative_summaries.status`、`firecrawl_jobs.status`）**无 DB CHECK**，仅由 Go 常量约束，详见 [DATABASE_FIELDS.md](DATABASE_FIELDS.md)。

---

## 文档导航

| 文档 | 描述 |
| ------ | ------ |
| [DATABASE_FIELDS.md](DATABASE_FIELDS.md) | 各表完整字段字典（含类型、约束、用途；需结合本页勘误阅读） |
| [ER_DIAGRAM.md](ER_DIAGRAM.md) | 实体关系图（注：FK 引用矩阵多为 GORM 逻辑关联，DB 层未强制，仅 `topic_tags_merged_into_id_fkey` 真实存在） |
| [DATA_LIFECYCLE.md](DATA_LIFECYCLE.md) | 数据链路状态流转 + 数据清理与保留策略 |

---

## 如何阅读

1. **先看本页概览与表清单**：了解数据库规模与 43 张真实表归属
2. **本页索引/约束清单**：按表查真实索引、唯一约束、CHECK
3. **[DATA_LIFECYCLE.md](DATA_LIFECYCLE.md)**：理解队列状态流转、清理回收机制、哪些表无自动清理
4. **[ER_DIAGRAM.md](ER_DIAGRAM.md)**：了解表间逻辑关系（注意 FK 多为逻辑关联）
5. **[DATABASE_FIELDS.md](DATABASE_FIELDS.md)**：按章节查阅具体字段定义

---

## 相关文档

- [项目架构总览](../architecture/overview.md) — 系统组件和子系统总览
- [业务流程](../flow/README.md) — 链路概要设计、前后端协作（"业务怎么跑的"）
- [数据库审计报告](../_audit/database-gaps.md) — 文档与代码差异的逐条审计
- [开发指南](../development.md) — 构建、测试、验证命令

---

## 更新日志

### 2026-07-30

- 新增 `route_param_options` 表（feed-param-options）：RSSHub 路由参数可选值字典，UNIQUE(route_id,param_name,value)，`source` ∈ {`manual`,`scraped`}（拒 `llm`，LLM 不生成参数值铁律）（Core 33→34，总数 46→**47**）
- 偏好/发现域索引节补 `route_param_options` 复合唯一索引 + 逻辑关联 `rsshub_routes`(OnDelete CASCADE)

### 2026-07-25

- 偏好转向量画像（preference-vector-feed-discovery）：删除 `user_preferences` 表（迁移 `20260725_0001` DROP，破坏性），新增 4 张表 `preference_vectors` / `rsshub_routes` / `route_embeddings` / `feed_recommendations`（Core 30→33，总数 43→**46**）
- 新增「偏好向量与订阅源发现域」索引节（4 表的唯一/复合/单列索引）
- 向量表清单补 `preference_vectors` / `route_embeddings`

### 2026-07-19

- 以代码（gorm tag + `postgres_migrations.go` + 运行时建索引）为真相重写全页
- 表数由「38」更正为 **43**（Core 30 + DataEnrichment 5 + TopicGraph 7 + Tracing 1）
- FK 数由「35」更正为 **1**（仅 `topic_tags_merged_into_id_fkey`；其余为 GORM 逻辑关联）
- 新增「真实表清单（43 张）」与「索引与约束清单」（按表逐条）
- 修正 §6 复合索引误报：`idx_articles_feed_*`、`idx_reading_behaviors_*`、`idx_firecrawl/tag_jobs_status_available_at`、`idx_article_topic_tag*` 实为单列
- 补遗漏索引：semantic_labels 系列、中间表 FK 侧、narrative_*、ai_call_logs、board_upgrade_suggestions、daily_report_section_relations、board_persistent_topics、board_topic_watches、topic_watch_hits
- 新增「CHECK 约束」节（3 个，历史零覆盖）

### 2026-05-14

- 初始版本：全景概览 + 文档导航
