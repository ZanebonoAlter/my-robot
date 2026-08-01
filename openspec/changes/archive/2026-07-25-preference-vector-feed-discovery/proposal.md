# Proposal: preference-vector-feed-discovery

## Why

现有「阅读偏好」功能是 write-only 的死功能：行为在采集、分数在算，但前端面板显示的 `read_score`/`interest_score` 后端从未返回（永远渲染 0），且偏好分数零消费——排序不读、AI 总结不引用、`getPreferenceScore()` 无人调用。文档宣称的「反过来影响排序与 AI 总结参考」从未实现闭环。

与其修补一个设计意图不明的旧物，不如基于「偏好」概念重做：把偏好表达为**按语义版块（SemanticBoard）聚合的 embedding 向量**，并让它产生真实价值——驱动 **RSSHub 订阅源发现**：向量粗筛 + LLM 精排，以持续推荐流的形式帮用户找到该订的源，顺带用问答交互解决冷启动。

## What Changes

- **偏好画像重建（替代旧偏好）**：以 `reading_behaviors`（保留现有采集）为权重、`article_topic_tags × topic_tag_board_labels × topic_tag_embeddings` 为数据源，按 SemanticBoard 聚合出每版块一个偏好向量（+ 全局兜底行），存入新表 `preference_vectors`；scheduler 定期重算。
- **旧偏好功能废弃（BREAKING）**：删除 `user_preferences` 表、`preference_update` scheduler job、`/api/user-preferences/*` 端点、`ReadingPreferencesPanel` 及相关前端 store/composable。`reading_behaviors` 采集与 `/api/reading-behavior/*` 保留。
- **RSSHub 路由目录**：新增 `rsshub_routes` 表，从自建 RSSHub 实例 `/api/namespace` 定时同步全量路由元数据；入库即解析 path 标记 `requires_parameters` / `usable_directly`；对 example 路径做异步限流可用性校验；为路由元数据生成 embedding（`route_embeddings`）。
- **订阅源推荐流**：偏好向量 × 路由向量余弦粗筛 top-N → LLM 精排生成推荐+理由 → `feed_recommendations` 卡片流（pending/accepted/dismissed 状态机 + 已订阅去重 + dismiss 负反馈降权）。`usable_directly` 卡片一键订阅；`requires_parameters` 卡片提示用户填参后 fetch 验证再订阅。手动刷新为主（「换一批」）。
- **问答式交互**：发现页支持自然语言提问（"我想看 AI 芯片相关资讯"），LLM 即时从目录检索推荐；同时将兴趣表达 embedding 为**种子偏好**写入 `preference_vectors`（冷启动，先与 board 向量匹配落版块）。
- **入口**：订阅源（feeds）区域新增「发现订阅源」入口；设置工作区 `preferences` section 由旧面板替换为新「兴趣画像」视图（各版块兴趣标签/权重可视化）。

## Capabilities

### New Capabilities

- `preference-profile`：偏好向量画像——行为加权聚合、按 SemanticBoard 分桶、时间衰减、scheduler 定期重算、问答种子写入、兴趣画像读取 API。
- `rsshub-route-catalog`：RSSHub 路由目录——实例 `/api/namespace` 同步、参数需求标记、可用性校验、路由 embedding 生成与维护。
- `feed-discovery`：订阅源发现——向量粗筛 + LLM 精排、推荐卡片状态机（pending/accepted/dismissed）、订阅落地（一键/填参）、问答式即时推荐、手动刷新。

### Modified Capabilities

- `settings-workspace`：`preferences` section 的内容由旧「阅读偏好面板」（阅读分/兴趣分列表）替换为新「兴趣画像」视图（按版块展示偏好标签与权重）；section 本身保留。

## Impact

- **后端**
  - 新增：`preference_vectors`、`rsshub_routes`、`route_embeddings`、`feed_recommendations` 四张表（pgvector 列，走既有 migrator/postgres_migrations 路径）。
  - 新增域代码：偏好聚合 service + scheduler job、RSSHub 目录同步 service + scheduler job、推荐生成 service（粗筛+精排）、发现/画像 handler 与路由。
  - 删除：`internal/admin/service/preferences_service.go`、`internal/admin/handler/preferences_handler.go` 中 user-preferences 部分、`internal/admin/scheduler/job_preference_update.go`、`models/user_preference.go`；runtime/router/routes/wire 相应清理。
  - 复用：`airouter.EmbeddingRequest` + `embedding_config`、`reading_behaviors`、`topic_tag_embeddings`、`topic_tag_board_labels`、scheduler 框架、`POST /feeds` / `POST /feeds/fetch`。
- **前端**
  - 新增：feeds 区「发现订阅源」入口 + 发现页（推荐卡片流 + 问答框）、设置 `preferences` section 新「兴趣画像」视图、相关 store/api/composable。
  - 删除：`ReadingPreferencesPanel.vue`、`useReadingPreferences.ts`、`stores/preferences.ts` 死代码；`types/reading_behavior.ts` 中 `UserPreference` 类型。
- **AI 成本**：偏好重算零 LLM（纯向量聚合）；路由 embedding 一次性 ~3245 条 + 增量 diff；推荐精排每次手动刷新一轮 LLM；问答每次提问一轮 LLM。
- **数据兼容（BREAKING）**：`user_preferences` 表数据为纯派生数据（behavior 重算产物），直接删表无迁移负担；`reading_behaviors` 历史数据保留并继续作为新偏好权重源。
- **配置**：新增 RSSHub 实例地址、目录同步间隔、推荐条数等 `ai_settings`/配置项（均可缺省）。
