
## watch 基座代码地图（本 change 实现落点）

watch 基座代码现状（2026-08-24 核对，实现阶段直接用）：

**后端落点**
- 表模型：`backend-go/internal/topicgraph/repository/daily_report_models.go`（BoardTopicWatch/TopicWatchHit 定义处，AutoMigrate 建表）
- CRUD：`repository/topic_watch_repository.go` — `CreateWatch(semanticBoardID uint, label string)`（待加 watchType 参数）、ListWatchesByBoard / ListActiveWatchesByBoard / UpdateWatch / DeleteWatch / GetWatchHitsByReport
- 判定：`service/daily_report_watch.go` — `EvaluateWatchHits(ctx, boardID, report, sections)` 挂在 `GenerateAndSaveReport` 末尾（失败 log 吞）；内部 `evaluateWatchHitsWithChat`（AI 单信号）+ `buildWatchHitPrompt` / `parseWatchHitResponse`。keyword 分叉在 EvaluateWatchHits 内做
- handler：`handler/topic_watch_handler.go` — 路由注册在 `api.POST/GET("/semantic-boards/:id/topic-watches")` + `api.PATCH/DELETE("/topic-watches/:id")`（参数是 :id 不是 :boardId）
- 迁移：`platform/database/postgres_migrations.go`（版本化迁移模式；已有 status CHECK 迁移可参照，~L1026）
- 去重：topic_watch_hits 复合唯一索引 (watch_id, section_id, report_id)，FK ON DELETE CASCADE（迁移 ~L1644）

**测试现状与分层**
- repository 测试已是 testcontainer：`topic_watch_repository_test.go` 用 `testutil.SetupTestDB`（禁 SQLite 规已满足，扩展直接加用例）
- service 测试无 DB（纯构造 fixture）：`daily_report_watch_test.go` 测 prompt 构建/解析；集成在 `daily_report_watch_integration_test.go`
- FK 级联：`platform/database/watch_hit_fk_cascade_test.go`
- 新增计划文件：`service/keyword_match_unit_test.go`（纯函数无 DB）、`platform/database/watch_type_column_test.go`（迁移幂等）

**前端落点**
- API 封装：`front/app/api/topicWatches.ts`（createWatch 待加 type）
- 新建关注对话框现状内嵌于 `features/tags/components/daily-report/DailyReportWatchBar.vue`——需抽成独立 TopicWatchCreateDialog.vue 挂载于版块级管理面板（tasks 3.2，入口见 watch-manage-panel.html 原型）
- 分组逻辑：同目录 `topicWatchGrouping.ts`；快捷入口按钮落 `DailyReportTopicSection.vue`（section 详情）、`DailyReportMiniLifeline.vue`（生命线）、`TopicLandscapePanel.vue`（泳道节点）

**spec 基线**
- 主 specs 曾遗漏同步，2026-08-24 已补回 `openspec/specs/topic-watch/spec.md`（5 requirement 基线）；本 change delta：MODIFIED 实体模型/顶部栏位/管理 API、REMOVED AI 命中判定、ADDED 命中判定分叉/即时匹配/快捷入口（26 个 delta Scenario，映射表在 tasks.md §9）

<!-- pinned 2026-08-24T02:25:45Z -->

## 前端版块工作台真实结构（入口落点依据）

前端版块工作台真实结构（2026-08-24 用户纠错后核实，勿再按旧假设设计）：

```
pages/tags.vue → TagsPage（叙事工坊，retire-narrative-legacy change 可能改名）
├─ topbar（返回首页 + 标题 + 引导 + 主题）
├─ BoardListSidebar（版块列表）
└─ tags-content（选中版块后）
     ├─ tags-content-tabs（5 个平级 tab：板块内容 composition / 话题总览 topic-overview / 日报 daily-reports / 文章 articles / 数据增强 enrichment）
     └─ tab 内容：composition→BoardCompositionPanel｜topic-overview→BoardThreadBrowser（真·话题总览）｜daily-reports→BoardDailyReportTimeline（日报列表卡→Teleport 全屏 reader 详情）｜articles→BoardTimelinePanel｜enrichment→BoardEnrichmentPanel
```

**关键纠错**：日报与话题总览是 TagsPage 的**平级 tab**（contentTab 切换），不是同一面板内切换。`BoardDailyReportTimeline.vue` 内部残留的 showThreadBrowser toggle（L47/L189/L196，内嵌一份 BoardThreadBrowser）是 tab 拆分前的遗留路径，不作为新设计依据。

**本 change 入口落点**：「我在追踪 (N)」chip 挂 tags-content-tabs 右端（margin-left:auto），五 tab 常驻——watch 是版块级实体，tab 栏是唯一随版块常驻的导航层。挂 Timeline 头部会随 tab 切换消失。

测试挂载点：TagsPage 无既有测试文件（需新增 TagsPage.test.ts 断言 chip 挂载+N 计数）；BoardDailyReportTimeline.test.ts 已存在（mock dailyReports/articles API + floating-ui 模式可参照）。原型：change 目录 mockups/watch-manage-panel.html（五 tab 骨架）+ watch-bar-keyword.html（日报栏只读化，两文件互链走旅程）。

<!-- pinned 2026-08-24T03:58:10Z -->

## 日报追踪改为列表预告与详情优先分区

用户确认的呈现决策：TagsPage 的日报 tab 时间线保持原有日期顺序，不设置顶；每条日报记录下追加紧凑命中 tag（# keyword、✦ topic，超出可 +N），用于预告命中。日报详情移除独立居中、窄宽的 DailyReportWatchBar 卡片；将「追踪关键字」「追踪话题」提升为位于「关心的话题」之前的全宽同级阅读分区，内部仅保留可点击单行索引，点击定位已有 section，不复制正文/常驻 reason。keyword 不再使用绿色大视觉，改靠 # 与弱 tag 区分；管理入口「我在追踪」位置和功能不变，仅保证图标与文字不换行并调整 padding/gap。现有 DailyReportTopicSection 的 zone→topic→section→thread 层级是详情分区集成点；DailyReportSidebar 仅是侧栏，用户所说“日报列表”是 TagsPage 日报 tab 的时间线列表。

**引用**：front/app/features/tags/components/daily-report/DailyReportWatchBar.vue、front/app/features/tags/components/daily-report/DailyReportTopicSection.vue、front/app/features/tags/components/daily-report/DailyReportSidebar.vue、front/app/features/tags/components/BoardDailyReportTimeline.vue

<!-- pinned 2026-08-24T11:30:13Z -->
