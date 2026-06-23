## Why

标签管理板块下的话题关联（`DailyReportSection` 之间跨天的关系）当前完全由匈牙利二分匹配 + embedding 相似度驱动，每天的话题聚类是 ephemeral 的（`ClusterTags` from scratch），没有任何持久锚点。这导致两个根因性问题：

1. **聚类无记忆**：`ClusterTags`（`daily_report_cluster.go:52`）的 prompt 不注入该 board 历史上已有的叙事框架，LLM 温度 0.1 但命名每天自由漂移（"开发者 Agent 工具链进入平台化竞争" → "开发者生态重构" → "AI 编程竞争加剧"），导致 `cluster_label` 文本漂移 → 基于 label 生成的 section embedding 漂移 → 匈牙利 Phase 1 断链率上升。
2. **关系纯靠 embedding，无身份层**：`daily_report_section_relations` 只存 `(from_id, to_id, distance)`，penalty=0.28。两个 section 即使讲同一件事，只要 embedding distance > 0.28 就被判成 emerging + ending，凭空多出状态节点。用户感知就是"板块下话题越加越散乱，每日聚类随机"。

侦探墙（`TopicDetectiveWall`）的"完整生命周期"调用 `getSectionLifecycle(sectionId)` 返回的是 section 级临时连通分量，连通性完全依赖 embedding 相似度，命名漂移即断裂。

## What Changes

参照板块（`SemanticBoard`）的持久化模式，在 board 和 section 之间引入一层持久实体 `PersistentTopic`（持久叙事话题），强制 1:N 归属，自动升级管理生命周期。

### 后端（核心）

- 新增持久层 `board_persistent_topics`：一个 board 有 N 个 PersistentTopic，每个有 label / description / embedding / status（candidate/active/archived）/ 首末命中日期 / 连续命中天数。
- `daily_report_sections` 新增归属字段：`persistent_topic_id` / `topic_match_distance` / `topic_match_confidence`（anchor_hit / auto_new / unmatched）。
- `ClusterTags` 改造：prompt 注入该 board 现有 PersistentTopic 列表，LLM 输出每个 group 的 `matched_topic_id`（解决根因 A）。
- 新增 `AssignSectionsToTopics`：section 强制归属到 1 个 PersistentTopic，双重确认（embedding ≤ match_threshold 且 LLM 当轮标记一致才 anchor_hit，否则开新 candidate）。
- 新增 `UpdateTopicLifecycle`：candidate 连续命中 ≥ upgrade_threshold（3 天）后获得人工确认资格，但不自动转 active；active 超过 decay_window（30 天）无命中自动转 archived（解决噪声直接进入持久泳道与"越积越多"）。
- 关系叠加：`daily_report_section_relations` 新增 `relation_type`（identity/similarity）。同 `persistent_topic_id` 的相邻天 section 写身份边（不受 0.28 penalty 限制），匈牙利相似度边保留作补充（解决根因 B）。**匈牙利算法本身不动**，只在它下游叠加身份边。
- 新增 API `getTopicLifeline(topicId)`：按 `persistent_topic_id` 聚合该话题下全部 section，绕过 embedding 连通性。
- 现有 `getBoardSectionTimeline` / `getSectionLifecycle` 响应增加 `persistent_topic` 嵌套字段和 `relation_type` 字段（向后兼容）。
- 回刷：`BackfillPersistentTopics(boardID)` 从历史 section 用 average_link 聚类反推 PersistentTopic（直接给 active 状态）。

### 前端（展示）

- 侦探墙总览：卡片按 `persistent_topic.color` 着色，`RedString` 按 `relation_type` 区分实线（身份边）/虚线（相似度边），candidate topic 卡片虚线边框。
- 侦探墙生命周期：**新增"话题生命线"模式（默认）**，调用 `getTopicLifeline`；保留现有"section 图"模式作为可切换项。
- 日报 section 列表：按 topic 分栏渲染（关心的话题 / 突发的新话题 / 未分类）。
- topic 管理 API（合并/重命名/归档）放侦探墙详情面板入口。
- 话题总览只为 active topic 建持久泳道；达到多天门禁的 candidate 由话题管理面板人工确认。
- 时间线状态与连线只使用匈牙利 similarity 关系；hover 高亮当前视图的完整连通链。

### 算法与真实数据验证（重点）

由于双重确认阈值（match_threshold）、升级阈值（upgrade_threshold）、decay 窗口等参数是否合理**存在不确定性**，本 change 不靠纯单元测试拍板，而是：

- 新增 testcontainer pgvector 集成测试，用**真实 embedding 数据 + 多天模拟**验证整条归属→升级→归档状态机。
- 新增**命名漂移场景集成测试**：连续天 section label 漂移、embedding distance > penalty，验证身份边不断链。
- 回刷脚本用真实历史 section 数据验证 topic 收敛性（单 board active topic 数稳定在合理区间）。
- 最终产出**成果报告**（`verification-report.md`）：量化指标含断链率、anchor_hit 占比、topic 收敛性，作为算法参数是否调优的依据。

## Capabilities

### New Capabilities

- `persistent-topic`: 持久叙事话题持久层 + section 强制 1:N 归属 + candidate/active/archived 状态机自动升级 + 回刷脚本
- `topic-cluster-anchoring`: `ClusterTags` 注入历史框架上下文，LLM 输出 matched_topic_id，消除命名漂移
- `topic-relation-overlay`: section 关系叠加身份边（同 topic 延续），与相似度边区分，绕过 penalty 断链
- `topic-lifeline`: 按 PersistentTopic 聚合的话题生命线查询 API + 侦探墙话题生命线展示模式

### Modified Capabilities

- `daily-report-system`: 日报生成流程在 MergeSimilarSections 后插入归属 + 生命周期更新两步，section 写入 persistent_topic_id
- `bipartite-relation-matching`: 关系表新增 relation_type 字段，身份边不受 penalty 限制（算法本身不改）
- `section-lifecycle`: 保留现有 section 级 lifecycle，新增 topic 级 lifeline（双模式）
- `topic-overview` / `topic-detective-wall`: 总览着色 + 边类型区分；生命周期新增话题生命线默认模式

## Impact

### 后端

- `backend-go/internal/topicgraph/repository/daily_report_models.go`：新增 `BoardPersistentTopic` 模型，`DailyReportSection` 增字段，`SectionRelation` 增 `RelationType`
- `backend-go/internal/topicgraph/repository/daily_report_repository.go`：新增 topic CRUD / 归属更新 / 身份边写入
- `backend-go/internal/topicgraph/service/daily_report_cluster.go`：`ClusterTags` 注入 existingTopics，输出 matched_topic_id
- `backend-go/internal/topicgraph/service/daily_report_orchestrator.go`：生成流程插入 AssignSectionsToTopics + UpdateTopicLifecycle 两步
- `backend-go/internal/topicgraph/service/daily_report_topic.go`（新）：归属算法 + 状态机
- `backend-go/internal/topicgraph/service/daily_report_backfill.go`（新或扩）：`BackfillPersistentTopics`
- `backend-go/internal/topicgraph/service/daily_report_relations.go`：身份边叠加写入
- `backend-go/internal/topicgraph/handler/daily_report_handler.go`：新增 `getTopicLifeline`，扩展现有响应
- `backend-go/internal/platform/database/migrations/`：新增迁移建 `board_persistent_topics` 表 + alter `daily_report_sections` / `daily_report_section_relations`
- `backend-go/internal/platform/database/seed_ai_settings.go`：新增 4 个 persistent_topic_* 配置项

### 前端

- `front/app/features/tags/components/detective-wall/CardGroup.ts`：卡片按 topic 着色
- `front/app/features/tags/components/detective-wall/RedString.ts`：按 relation_type 区分实/虚线
- `front/app/features/tags/components/detective-wall/InteractionLayer.ts`：生命周期双模式
- `front/app/features/tags/components/SectionTimeline.vue`（或等价组件）：分栏渲染
- `front/app/api/dailyReports.ts`：新增 getTopicLifeline，扩展类型

### 测试与报告

- `backend-go/internal/topicgraph/service/daily_report_topic_test.go`（单元，SQLite）
- `backend-go/internal/topicgraph/service/daily_report_topic_integration_test.go`（集成，testcontainer pgvector）
- `docs/v1.3.3-good-taste/changes/addon-feather/2026-06-19-persistent-topic/verification-report.md`（成果报告）

### 数据库

- 新增表 `board_persistent_topics`（含 HNSW embedding 索引）
- `daily_report_sections` 增 3 列（不加 NOT NULL，兼容回刷过渡期）
- `daily_report_section_relations` 增 1 列 `relation_type`
- 按开发执行规范 §10：迁移建表 + 索引走显式迁移，不依赖 gorm 自动建表

### 不影响

- 匈牙利算法（`hungarianAssignment` / `assignmentCost`）实现不动
- board-upgrade 机制不动（并存，粒度不同）
- 不做 N:M 归属、不做跨 board 共享、不强制删除 section 级 lifecycle
