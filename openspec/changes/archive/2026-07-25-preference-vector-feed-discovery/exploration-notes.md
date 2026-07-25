# 探索笔记：preference-vector-feed-discovery（apply 前必读）

> 2026-07-24 opsx-explore 阶段产出。记录关键代码现状、复用资产、废弃清单，供实现时直接索引。
> 设计决策已定稿在 `design.md`（D1-D9），本文件是其代码级证据与索引。

## 1. 旧功能为什么废（write-only 三实锤）

1. **UI 字段对不上**：`ReadingPreferencesPanel.vue` 显示 `read_score` / `interest_score`（按 interest_score 排序、画两根进度条），但后端 `UserPreference` 模型（`backend-go/internal/models/user_preference.go`）只有 `preference_score`，`ToDict()` 不返回那俩字段 → 面板恒为 0。来源是前端 commit `b956a08 调整一些硬编码` / `546978d 前端架构梳理` 引入的 mock 字段，后端从未实现。
2. **分数零消费**：前端 `getPreferenceScore()`（`front/app/stores/preferences.ts`）零调用；后端 preference 只被自己的 service/handler/scheduler 引用（`grep -rln PreferenceScore` 证实），排序、AI 总结、标签全不碰。
3. **写链路活着**：`useReadingTracker`（`front/app/features/preferences/composables/useReadingTracker.ts`）接在 `useArticleContentView.ts`（第 5、76 行），批量上报 `POST /api/reading-behavior/track-batch`；`preference_update` scheduler（1800s，`runtime.go:122-128`）全量重算。**采集链路保留，是偏好权重数据源。**

## 2. 复用资产清单（apply 时优先复用，不要新造）

| 资产 | 位置 | 用法 |
| ---- | ---- | ---- |
| 行为采集 | `front/app/features/preferences/composables/useReadingTracker.ts`；上报 API `front/app/api/reading_behavior.ts` | 不动，权重数据源 |
| 行为表 | `reading_behaviors`（后端 handler：`backend-go/internal/admin/handler/preferences_handler.go` 的 reading-behavior 部分） | 保留；偏好聚合从这里取权重 |
| 文章↔标签 | `article_topic_tags`（`backend-go/internal/models/topic_graph.go:156-173`，含 Score/Source 字段） | 互动文章的标签集合 |
| 标签模型 | `TopicTag`（`topic_graph.go:47-`）：category=event/person/keyword、status active/merged、IsWatched | 过滤 status=active |
| 标签↔版块 | `topic_tag_board_labels`（四规则匹配产物，单 tag 最多 3 板块，见 `docs/reference/flow/semantic-board.md`） | 偏好分桶依据 |
| 标签向量 | `TopicTagEmbedding`（`topic_graph.go:119-136`）：`embedding_type` 双轨（identity/semantic）、`EmbeddingVec string gorm:"type:vector"`、`text_hash` 重嵌入检测 | **偏好向量加数，取 semantic 轨；pgvector 列写法照抄此表** |
| 板块向量 | SemanticBoard embedding（board-direction-check 引入；回填端点 `POST /api/semantic-boards/backfill-embeddings`） | 冷启动种子落版块匹配 |
| embedding 通路 | `backend-go/internal/platform/airouter/embedding.go`：`EmbeddingRequest{Input, Model, Dimensions, Operation(必填), SessionID, Metadata}` / `EmbeddingResult` / `CosineSimilarity` | 路由 embedding、问答 embedding |
| embedding 配置 | `backend-go/internal/models/embedding_config.go`（`embedding_config` 表）+ `internal/platform/aisettings/config_store.go` | 取当前模型/维度，入库记 dimension/model |
| embedding 队列 | `EmbeddingQueue`（`models/embedding_queue.go`，pending/processing/completed/failed）+ `tagmanagement/service/core/embedding_queue.go` + handler `embedding_queue_handler.go` | 路由 embedding 生成复用此队列模式 |
| LLM 通路 | `internal/platform/airouter/router.go`（openai_compatible + fallback）；aisettings | 精排/问答；route 选型 apply 时定（design Open Questions） |
| 调度框架 | `internal/admin/scheduler/` + `runtime.go` `registry.Register(scheduler.Config{Name, Description, Job, Persistence})` 模式（现有样例 `job_preference_update.go`、`job_board_upgrade_suggest.go`） | 两个新 job 照此注册 |
| 订阅落地 | `backend-go/internal/reader/routes.go`：`POST /feeds`（CreateFeed）、`POST /feeds/fetch`（先试再订） | accept 双路径 |
| dismiss 冷却模式 | `board_upgrade_suggestions`：`ComputeSuggestionHash` 幂等 + `CountDismissedInCooldown`（`tagmanagement/service/board/board_upgrade_suggestion_persist.go`） | 推荐 hash 幂等 + 冷却照抄 |
| Feed 模型 | `backend-go/internal/models/feed.go`：URL unique 约束、CategoryID 可空 | 已订阅去重按 feeds.url |

## 3. RSSHub 目录实测数据（2026-07-24，自建实例 47.110.71.194:1200）

- `GET /api/namespace` → 200，**2.9 MB 全量目录**：**1563 命名空间 / 3245 条路由**
- **91.9%（2981 条）带 example** → 可用性校验可行；59.5% 中文名
- 每条含：path（如 `/81rc/:category{.+}?`）、name、url、maintainers、example、parameters（中文说明）、description（markdown）
- 参数语法：`:param` 必填、`:param?` 可选、`{.+}?` 正则约束——D3 解析规则按此
- 外网注意：rsshub.app / raw.githubusercontent.com 从本环境不可达，**目录来源只能走自建实例**（design D2 已锁）
- dump-sanitizer 有现成实例地址参考：`backend-go/cmd/dump-sanitizer/sanitize.go:91-103`（`defaultRSSHubRewrite = "47.110.71.194:1200=rsshub.app"`，env `RSSHUB_REWRITE`）

## 4. 旧功能废弃清单（design D9 的代码级落点）

**后端删除**：
- `backend-go/internal/models/user_preference.go`（整文件）+ migrator 中 `user_preferences` 注册 + DROP TABLE 迁移
- `backend-go/internal/admin/service/preferences_service.go`（PreferenceService 全量重算，`calculatePreferenceScore` 权重 0.4/0.3/0.3 + 30 天衰减——公式语义记录在 `docs/reference/flow/reading.md` 业务约束 #2，删除时该约束一并重写）
- `backend-go/internal/admin/handler/preferences_handler.go` 的 user-preferences 部分（**reading-behavior 部分保留！**）
- `backend-go/internal/admin/scheduler/job_preference_update.go`
- `backend-go/internal/app/runtime.go:122-128`（preference_update 注册）、`internal/admin/routes.go:41`（`/user-preferences` group，注意 `:34` 的 `/reading-behavior` group 保留）、`internal/admin/wire.go` 相应装配

**前端删除**：
- `front/app/components/dialog/ReadingPreferencesPanel.vue`（245 行）
- `front/app/composables/useReadingPreferences.ts`
- `front/app/stores/preferences.ts`（119 行，含 computed 内原地 sort 污染源数组的已知 Low 缺陷，随删除消除）
- `front/app/types/reading_behavior.ts` 的 `UserPreference` 接口（`ReadingBehaviorEvent`/`ReadingStats` 保留）
- `front/app/features/settings/components/SettingsSectionPreferences.vue`（7 行壳，由新画像视图接替）
- 引用方：`GlobalSettingsDialog.vue`、`SettingsWorkspace.vue`

**保留不动**：`reading_behaviors` 采集全链路、`/api/reading-behavior/*`、`useReadingTracker`、`front/app/api/reading_behavior.ts` 的 track/stats 方法。

## 5. 新功能代码落点建议（沿用现有域划分）

- 新域建议 `backend-go/internal/discovery/`（目录同步 + 推荐 + 问答），偏好聚合放 `internal/admin/service/` 继任者或一并入 discovery 域——apply 步骤 1 定。
- 前端发现页落 `front/app/features/feeds/` 或新 `features/discovery/`；画像视图落 `features/settings/components/` 接替 SettingsSectionPreferences。
- 设置工作区 section 列表见 `openspec/specs/settings-workspace/spec.md` Sections（preferences section 保留，内容替换）。

## 6. 相关 flow 文档（改代码前 doc-impact.sh context 会 dump，必读）

- `docs/reference/flow/reading.md`：业务约束 #2（旧偏好公式，将重写）、#3（全量重建幂等哲学，新设计沿用）、#4（1800s 调度）
- `docs/reference/flow/semantic-board.md`：四规则匹配、MaxBoards=3、direction_mismatch、suggestion_hash 幂等 + 冷却、watch GC——推荐状态机借鉴对象

## 7. 进行中的相邻 change（不冲突，注意叙事）

- `watch-keyword-and-quickadd`（in-progress，0/43）：watch 关键字轨 + 内容流快捷关注。与本 change「盯内容 vs 找源」互补；无代码重叠，但 apply 时若它已落地，发现页文案避免与「关注」概念混淆。

## 8. apply 时的 Open Questions / 已拍板决策

**已拍板（2026-07-24，design.md「已拍板决策」节为权威）：**

- A 种子累积 = 加权合并（α=0.4 可配），保 `UNIQUE(board_id, source)` 单行，不放宽
- B 去重 = route_id 维度状态机去重（accepted/dismissed 的 route 不再推），`feeds.url` 仅对 usable_directly
- C `recommendation_hash` = route_id+board_id，不含 source；qa/manual_refresh 共享幂等池与 dismiss 冷却池

**apply step1 待验/待定：**

1. 精排 LLM 走哪条 capability route（复用现有 route 还是新建 `feed_discovery`）——看 aisettings 现有 route 表定
2. 发现页前端落位（feeds 侧栏入口 vs 独立路由页）——按 shell 结构定
3. **favorite 上报链路**（tasks 2.6）：grep 确认收藏是否写 `reading_behaviors.event_type='favorite'`；若无 → 补上报或下掉 D1 favorite=1.0 档
