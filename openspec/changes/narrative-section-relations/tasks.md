## 1. 数据库迁移

- [x] 新增 `daily_report_section_relations` 表 migration（字段、约束、索引）
- [x] 编写数据迁移：将现有 `prev_section_id` 转为 relation 记录
- [x] 删除 `daily_report_threads` 表的 `status`、`prev_thread_id` 列和相关索引
- [x] 删除 `daily_report_sections` 表的 `prev_section_id`、`status` 列

## 2. 后端模型与存储层

- [x] 新增 `SectionRelation` GORM 模型（from_section_id, to_section_id, distance）
- [x] `DailyReportSection` 移除 `PrevSectionID`、`Status` 字段
- [x] `DailyReportThread` 移除 `Status`、`PrevThreadID` 字段
- [x] 新增 `MatchAndSaveRelations(tx, boardID, newSections)` 函数：对每个新 section 用 embedding 查询同 board 下非当日 section，distance < 0.35 的写入 relation
- [x] 新增 `GetSectionRelations(boardID, days)` 查询函数
- [x] 新增 `GetSectionLifecycle(sectionID)` 函数：基于 relation 表双向扩展，返回 sections + relations
- [x] 新增 `DeriveSectionStatus(sectionID, relations)` 推导函数：返回 status（emerging/continuing/split/merge）+ ended（boolean）
- [x] `SaveReport()` upsert 逻辑：删除旧 section 前先清理 `daily_report_section_relations` 中涉及这些 section 的 relation 记录（from_section_id IN oldIDs OR to_section_id IN oldIDs）
- [x] 更新 `BackfillSectionEmbeddings`：Phase 2 的匹配结果改为写入 relation 表，不再写 `prev_section_id` + `status`

## 3. 后端生成流程改造

- [x] `generator.go`：移除 thread prompt 中的 status、prev_thread_id 相关要求和 JSON schema 字段
- [x] `generator.go`：移除 `matchPreviousThreads()` 和 `getPrevThreadSummaries()` 调用
- [x] `generator.go`：`SaveReport()` 中将 `MatchSectionsByEmbedding` + `prev_section_id` 替换为 `MatchAndSaveRelations`
- [x] `generator.go`：升级 prompt version 为 "3.0"

## 4. 后端 API 改造

- [x] 改造 `getBoardSectionTimeline` handler：返回 `{sections: [], relations: []}` 格式，sections 含动态推导 status
- [x] 改造 `getSectionLifecycle` handler：基于 relation 表查询，返回 `{sections: [], relations: []}`
- [x] 改造 `getDailyReportDetail` handler：section 响应移除 `status`、`prev_section_id` 字段（status 仅在 timeline/lifecycle API 中动态推导，报告详情 API 不再返回）
- [x] 移除 `getThreadLineage` handler 和路由
- [x] 移除 `getBoardThreadTimeline` handler 和路由
- [x] 更新 `router.go` 路由注册

## 5. 前端 API 层更新

- [x] 更新 `dailyReports.ts` 类型定义：`SectionTimelineNode` 移除 `prev_section_id`，新增 `SectionRelation` 类型
- [x] 更新 `DailyReportThread` 类型：移除 `status`、`prev_thread_id`
- [x] 更新 `DailyReportSection` 类型：移除 `status`、`prev_section_id` 字段
- [x] 移除 `getThreadLineage`、`getBoardThreadTimeline` 函数
- [x] `getBoardSectionTimeline` 返回类型改为 `{sections, relations}`
- [x] `getSectionLifecycle` 返回类型从 `{chain: SectionLifecycleNode[]}` 改为 `{sections: SectionLifecycleNode[], relations: SectionRelation[]}`
- [x] 移除 `ThreadLineageNode` 相关类型定义

## 6. 前端组件改造

- [x] `BoardDailyReportTimeline.vue`：thread 列表展示移除 status 徽章，移除 sitemap 图标按钮（ThreadLineagePanel 入口）
- [x] `BoardDailyReportTimeline.vue`：section 卡片移除 status 徽章（报告详情 API 不再返回 section status）
- [x] 移除 `ThreadLineagePanel.vue` 组件文件
- [x] 引入 `d3-dag` 依赖（`pnpm add d3-dag`）
- [x] 新增 `useDagLayout` composable：封装 d3-dag Sugiyama 布局计算，泛型接口，返回 positioned nodes + edge paths
- [x] `SectionLifecyclePanel.vue` 重写：水平时间线布局（日期分桶横排），SVG 贝塞尔曲线连线，羊皮纸配色
- [x] `SectionLifecyclePanel.vue` 后端 BFS 限制为直接邻居（1跳），避免拉入整个连通分量
- [x] `SectionLifecyclePanel.vue` 点击节点选中→下方展开线索列表，线索展开显示关联文章（页内预览）
- [x] `SectionLifecyclePanel.vue` hover 高亮直接关联节点和边，无关节点淡出
- [x] `SectionLifecyclePanel.vue` 节点 hover 显示导航箭头按钮，点击跳转到对应日期报告
- [x] `SectionLifecyclePanel.vue` openArticle 事件通过 BoardDailyReportTimeline 冒泡至 TagsPage
- [x] `BoardThreadBrowser.vue` 重写：简单时间线布局（按日期分桶→列，同日纵向排列），SVG 贝塞尔曲线连线
- [x] 两个组件的 ended 节点视觉处理：降低透明度 + 灰色虚线边框
- [x] `BoardThreadBrowser.vue` 增大间距（COL_W=120），默认显示截断的 cluster_label
- [x] `BoardThreadBrowser.vue` hover 高亮直接关联节点和边，无关节点淡出（修复 isEdgeHighlighted 参数名 bug）
- [x] `BoardThreadBrowser.vue` 点击话题弹窗展示关联线索（threads）列表，含标题/摘要/文章数
- [x] `BoardThreadBrowser.vue` 线索展开显示关联文章列表，点击文章触发页内 ArticleContentView 预览（emit 事件冒泡至 TagsPage）

## 7. 同日 Section 两阶段合并

- [x] 新增 `MergeSimilarSections(sections []DailyReportSection, threadBatches [][]DailyReportThread, tags []TagInput) ([]DailyReportSection, [][]DailyReportThread)` 函数
- [x] Stage 1：计算同日 section embedding pairwise distance，对 distance < 0.20 的 pairs 用 union-find 做传递闭包合并
- [x] 合并规则：保留 article_count 最大的为主 section，合并 tag_ids、重算 best_tier/avg_score，threadBatches 对应合并
- [x] Stage 2：收集 0.20-0.25 区间的 pairs，构建 LLM prompt（含 cluster_label + tag_labels），批量调用判断 merge/keep
- [x] `GenerateDailyReport` 流水线中 embedding 生成后、`SaveReport` 前插入 `MergeSimilarSections` 调用

## 8. distance=0.0 bug 修复

- [x] `MatchAndSaveRelations` 中将 `FirstOrCreate` 替换为 raw SQL `INSERT ... ON CONFLICT (from_section_id, to_section_id) DO UPDATE SET distance = EXCLUDED.distance`

## 9. 验证

- [x] 恢复前端 `DailyReportThread` 类型中的 `related_article_ids` 字段（已确认前后端均保留，无需操作）
- [x] 后端编译通过、lint 通过、受影响包测试通过
- [x] 前端 lint 通过、build 通过（typecheck 有 TagsPage.vue 既有错误，非本次变更）
