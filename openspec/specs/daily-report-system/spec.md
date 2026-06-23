## Purpose

日报系统替代旧叙事系统，为每个 SemanticBoard 每日生成结构化日报，包含今日重点、板块动态和聚类叙事线索。

## Requirements

### Requirement: 日报数据模型
系统 SHALL 新建 `board_daily_reports` 和 `daily_report_sections` 两张表，独立于旧 `narrative_boards`/`narrative_summaries`。每个 SemanticBoard 每天至多一条 `BoardDailyReport` 记录。

**BoardDailyReport 字段**：id, semantic_board_id, period_date, title, summary, highlights(JSON), dynamics(TEXT), article_count, event_tag_count, cluster_count, status(generating/done/failed), raw_clusters(JSON), prev_report_id(可为空，指向前一日日报), generation_prompt_version, created_at, updated_at。

**DailyReportSection 字段**：id, report_id, cluster_index, cluster_label, cluster_tag_ids(JSON), article_count, best_tier, avg_score, embedding(向量列，维度由模型输出决定, 用于语义匹配), created_at。线程数据已迁移至 `daily_report_threads` 表，通过 `section_id` 外键关联。跨天关系通过 `daily_report_section_relations` 关系表表达，status 通过关系拓扑动态推导。

**DailyReportThread 字段**：id, report_id, section_id, title, summary, tag_ids(JSONB), confidence, created_at。

`highlights` JSON 结构：`[{title: string, reason: string, tag_ids: uint[]}]`，2-3 个重点项。

`raw_clusters` JSON 结构：`[{group_name: string, tag_ids: uint[]}]`，LLM 分组原始结果，用于调试。

#### Scenario: 创建日报记录
- **WHEN** 为 SemanticBoard #5 在 2026-05-25 生成日报
- **THEN** 系统 SHALL 在 `board_daily_reports` 表创建一条记录，status="generating"，period_date="2026-05-25"，semantic_board_id=5

#### Scenario: 日报记录唯一性
- **WHEN** SemanticBoard #5 在 2026-05-25 已有一条 status="done" 的日报
- **THEN** 系统 SHALL NOT 创建重复记录，而是更新已有记录

#### Scenario: 日报关联昨日报告
- **WHEN** SemanticBoard #5 在 2026-05-24 有一条已完成日报 (id=42)
- **THEN** 2026-05-25 的日报记录 SHALL 设置 prev_report_id=42

#### Scenario: 线程存储在独立表中
- **WHEN** 日报生成完成
- **THEN** 每个聚类的叙事线程 SHALL 作为独立行存储在 `daily_report_threads` 表中，通过 `section_id` 关联到对应的 section

### Requirement: 事件标签去重
系统 SHALL 在生成日报前对收集到的事件标签进行程序化精确去重，不使用 LLM。去重 SHALL 应用两条规则：(1) 关联文章集合完全相同的标签合并为一个；(2) article_count=1 且关联同一篇文章的标签合并为一个。去重 SHALL 不改变原始标签数据，仅在生成流程中使用去重后的列表。

#### Scenario: 文章集合完全相同的标签去重
- **WHEN** 标签 A 和标签 B 关联的文章 ID 集合均为 {101, 102, 103}
- **THEN** 系统 SHALL 将二者合并为一个（保留 article_count 更大或 id 更小的标签）

#### Scenario: 单篇文章标签去重
- **WHEN** 标签 X (article_count=1, 关联文章 200) 和标签 Y (article_count=1, 关联文章 200) 为不同标签但指向同一文章
- **THEN** 系统 SHALL 将二者合并为一个

#### Scenario: 去重不影响原始数据
- **WHEN** 去重流程执行后
- **THEN** `topic_tags` 表中的原始标签记录 SHALL 保持不变，去重仅在内存中进行

### Requirement: 日报质量筛选
`collectBoardTags` 查询携带 `match_reason` 和 `score`（包括 fallback 路径产生的标签）。生成管线在聚类前增加筛选层：

1. 过滤 `direction_mismatch = true`
2. 保留 `match_reason ∈ {direct_hit, hit_rate, max_sim}`（含 downgraded）
3. 过滤 `weighted`（最弱规则）
4. 如果剩余 < 10 个标签，把 weighted 也拉回来（保底）
5. 如果剩余 > 30 个标签，按 `(tier, score)` 排序后截断到 top-30

Fallback 标签同等对待：fallback 路径产生的标签也携带 `match_reason`/`score`，筛选规则完全一致。

#### Scenario: 质量筛选过滤 weighted
- **WHEN** 收集到 20 个标签，其中 5 个 weighted
- **THEN** 系统 SHALL 过滤掉 weighted 标签，15 个进入聚类

#### Scenario: 保底机制
- **WHEN** 收集到 12 个标签，过滤 weighted 后只剩 8 个 (< 10)
- **THEN** 系统 SHALL 把 weighted 标签拉回，12 个全部进入聚类

#### Scenario: 截断机制
- **WHEN** 收集到 40 个标签，过滤后剩 35 个 (> 30)
- **THEN** 系统 SHALL 按 (tier, score) 排序后截断到 top-30

### Requirement: 聚类数限制

`ClusterTags` prompt 按标签数量条件分支：
- 标签数 ≤ 15：不限制组数
- 标签数 16-25：分成 6-12 组
- 标签数 > 25：分成 8-15 组

在 `clusterSystemPrompt` 构建时根据 `len(tags)` 动态插入对应约束。

#### Scenario: 小量标签不限组数
- **WHEN** 去重后有 14 个标签
- **THEN** LLM 分组不限制组数，自然分组即可

#### Scenario: 中量标签约束分组
- **WHEN** 去重后有 20 个标签
- **THEN** prompt SHALL 约束为 6-12 组

#### Scenario: 大量标签约束分组
- **WHEN** 去重后有 30 个标签
- **THEN** prompt SHALL 约束为 8-15 组

### Requirement: 聚类排序字段

生成报告时，系统 SHALL 为每个 DailyReportSection 计算：
- `BestTier int`：该 section 中所有 tag 的最高 tier（match_reason + downgraded 映射）
- `AvgScore float64`：该 section 中所有 tag 的 score 平均值

前端用 `best_tier ASC, avg_score DESC` 排序聚类。

#### Scenario: 计算 best_tier
- **WHEN** section 的 tags 包含 direct_hit(tier=0) 和 weighted(tier=3)
- **THEN** best_tier=0

#### Scenario: 前端按质量排序聚类
- **WHEN** 有 3 个 section，best_tier 分别为 0、2、1
- **THEN** 前端按 0→1→2 顺序展示

### Requirement: LLM 语义分组
系统 SHALL 使用单次 LLM call 对去重后的事件标签做语义分组。分组粒度 SHALL 为"同一核心事件"，每组 2-8 个标签，超过 8 个 SHALL 拆分为多组，单个标签可自成一组。LLM 调用 SHALL 使用 temperature=0.1，输出 SHALL 遵循 JSON schema 约束 `[{group_name: string, tag_ids: uint[]}]`。

#### Scenario: 正常分组
- **WHEN** 去重后有 30 个事件标签
- **THEN** LLM SHALL 输出 5-10 个分组，每组含 group_name 和对应的 tag_ids 列表

#### Scenario: 标签数少于 3
- **WHEN** 去重后只有 2 个事件标签，且语义不相关
- **THEN** LLM SHALL 输出 2 个分组，每个分组含 1 个标签

#### Scenario: 分组结果持久化
- **WHEN** LLM 分组完成
- **THEN** 分组结果 SHALL 存入 `BoardDailyReport.raw_clusters` JSON 字段，用于调试审计

### Requirement: 日报分段并行生成
系统 SHALL 并行执行两类 LLM 生成调用：
- **Call A（今日重点）**：输入全部标签(label+desc+article_count) + 昨日日报，输出 2-3 个重点项（含标题、选择理由、关联标签 ID）
- **Call C×K（聚类叙事线索）**：每个聚类一次调用，输入该聚类标签，输出 0-N 条线索（仅 title + summary + tag_ids + confidence）

Call C 生成完成后，系统 SHALL 将线程作为 `daily_report_threads` 表的行持久化。

`GenerateDailyReport` 函数 SHALL 返回 `(*BoardDailyReport, []DailyReportSection, [][]DailyReportThread, error)`，其中第三项为每个 cluster 对应的 `[]DailyReportThread` 列表，供 `SaveReport` 批量写入 `daily_report_threads` 表。`[][]DailyReportThread` 中的索引 SHALL 与 `[]DailyReportSection` 一一对应。

Thread 生成 SHALL 移除 status、prev_thread_id 相关的 prompt 要求和 JSON schema 字段。Thread 的 system prompt SHALL 简化为仅要求输出 title、summary、tag_ids、confidence。

prompt version 升级为 "3.0"。

#### Scenario: 并行生成成功
- **WHEN** 有 5 个聚类
- **THEN** 系统 SHALL 同时发起 Call A + Call C×5，共 6 个并行 LLM 调用

#### Scenario: Thread 输出不含 status
- **WHEN** Call C 为某聚类生成 3 条 thread
- **THEN** 每条 thread SHALL 只包含 title、summary、tag_ids、confidence，不包含 status 和 prev_thread_id

#### Scenario: 昨日日报不存在
- **WHEN** 某板某日为首次生成日报
- **THEN** Call A 的"昨日日报"输入 SHALL 为空，Call C 不传入任何历史线索上下文（已移除 getPrevThreadSummaries 调用）



### Requirement: 日报生成编排流水线
系统 SHALL 提供 `GenerateDailyReport(ctx, boardID, date)` 编排函数，按顺序执行：收集板内事件标签 → 质量筛选 → 去重 → LLM 分组(带组数限制) → 查询昨日日报 → 并行生成(Call A + C×K) → section embedding 生成 → **同日 section 两阶段合并** → section embedding 匹配写入关系表 → 组装 BoardDailyReport + DailyReportSection(含 best_tier/avg_score) → 存储。生成 SHALL 通过 goroutine 异步执行。

流水线 SHALL NOT 执行 thread 级别的 tag 交集匹配或 prev_thread_id 赋值。

#### Scenario: 完整流水线执行
- **WHEN** 触发 SemanticBoard #5 在 2026-05-25 的日报生成
- **THEN** 系统 SHALL 按序执行：收集标签 → 质量筛选 → 去重 → LLM分组 → 查询昨日日报 → 并行生成 → embedding 生成 → 同日合并 → section embedding 匹配写入关系表 → 组装存储 → status="done"

#### Scenario: 生成失败
- **WHEN** 流水线中任一步骤失败（如 LLM 调用超时）
- **THEN** 系统 SHALL 设置 status="failed"，保留已完成的中间结果（raw_clusters 等），WS 广播失败状态

### Requirement: 日报存储
系统 SHALL 提供 `SaveReport`（创建或更新日报+关联 sections）、`GetReport(boardID, date)`（查询单篇）、`ListReports(boardID, days)`（查询列表）三个存储接口。

`ListReports` SHALL 在 `days <= 0` 时使用 7 天默认值。对于任意正数 `days`，系统 SHALL 按请求天数查询，不得静默截断到 30 天。

#### Scenario: 保存日报和分组
- **WHEN** 流水线完成生成
- **THEN** 系统 SHALL 创建一条 BoardDailyReport 记录（status="done"）和多条 DailyReportSection 记录（每个聚类一条），并在事务中完成

#### Scenario: 查询最近 7 天日报
- **WHEN** 请求 `ListReports(boardID=5, days=7)`
- **THEN** 系统 SHALL 返回 board #5 最近 7 天的日报列表，按 period_date 倒序

#### Scenario: 查询超过 30 天日报
- **WHEN** 请求 `ListReports(boardID=5, days=42)`
- **THEN** 系统 SHALL 查询最近 42 天，不得将范围截断为 30 天

#### Scenario: 非正数天数使用默认值
- **WHEN** 请求 `ListReports(boardID=5, days=0)`
- **THEN** 系统 SHALL 查询最近 7 天

### Requirement: 日报生成 API — 异步触发
系统 SHALL 提供 `POST /api/daily-reports/generate` 端点，接受 `{date: string, board_id?: number}` 参数。board_id 为空时生成所有活跃 board 的日报。端点 SHALL 立即返回 `{job_id: string, status: "processing"}`，后台 goroutine 异步执行生成。

#### Scenario: 触发单板日报生成
- **WHEN** 请求 `POST /api/daily-reports/generate {date: "2026-05-25", board_id: 5}`
- **THEN** 系统 SHALL 立即返回 `{job_id: "xxx", status: "processing"}`，后台开始为 board #5 生成日报

#### Scenario: 触发全板日报生成
- **WHEN** 请求 `POST /api/daily-reports/generate {date: "2026-05-25"}`
- **THEN** 系统 SHALL 立即返回 `{job_id: "xxx", status: "processing"}`，后台依次为所有活跃 board 生成日报

### Requirement: 日报生成 WebSocket 进度广播
生成 goroutine SHALL 通过 `ws.GetHub().BroadcastRaw()` 广播两类消息：
- `daily_report_progress`：每完成一个 board 后广播 `{"type": "daily_report_progress", "job_id": "...", "board_id": N, "board_name": "...", "status": "completed|failed", "saved": N, "progress": "current/total"}`
- `daily_report_done`：全部完成后广播 `{"type": "daily_report_done", "job_id": "...", "total_saved": N, "total_boards": N}`

#### Scenario: 单板生成完成广播
- **WHEN** board #5 的日报生成成功
- **THEN** 系统 SHALL 广播 `{"type": "daily_report_progress", "job_id": "xxx", "board_id": 5, "board_name": "AI与机器学习", "status": "completed", "saved": 1, "progress": "1/1"}`

#### Scenario: 全部生成完成广播
- **WHEN** 3 个 board 的日报全部生成完毕
- **THEN** 系统 SHALL 广播 `{"type": "daily_report_done", "job_id": "xxx", "total_saved": 3, "total_boards": 3}`

### Requirement: 日报查询 API
系统 SHALL 提供以下查询端点：
- `GET /api/semantic-boards/:id/daily-reports?days=7`：查询该 board 最近 N 天的日报列表
- `GET /api/daily-reports/:id`：查询单篇日报详情（含关联 sections，每个 section 通过 GORM Preload 包含 threads 列表）
- `GET /api/semantic-boards/:id/section-timeline?days=14`：查询板块 section 时间线（含关系）

`GET /api/semantic-boards/:id/daily-reports` SHALL 在未提供有效正数 `days` 时使用 7 天默认值，并对任意正数 `days` 查询实际请求范围，不得静默截断到 30 天。

`GetReportByID` SHALL 使用嵌套 Preload `"Sections.Threads"` 加载 section 及其关联线程。`DailyReportSection` 的 JSON 响应中 `threads` 字段 SHALL 包含 `[]DailyReportThread` 对象数组（每条含 id、report_id、section_id、title、summary、tag_ids、confidence），section 和 thread 均不含 status/prev_*_id 字段。

#### Scenario: 查询板块日报列表
- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=7`
- **THEN** 系统 SHALL 返回 board #5 最近 7 天的日报列表，每条含 id、title、summary、period_date、status、cluster_count、article_count

#### Scenario: 查询日报详情含线程
- **WHEN** 请求 `GET /api/daily-reports/42`
- **THEN** 系统 SHALL 返回日报 #42 完整内容，每个 section 包含 threads 列表（每条含 id、title、summary、tag_ids、confidence、related_article_ids），section 和 thread 均不含 status/prev_*_id 字段

#### Scenario: API 查询超过 30 天
- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=42`
- **THEN** 系统 SHALL 返回最近 42 天范围内的全部匹配日报

#### Scenario: 无日报时返回空
- **WHEN** 请求 `GET /api/semantic-boards/5/daily-reports?days=7`，但该 board 无日报
- **THEN** 系统 SHALL 返回空数组

### Requirement: 日报时间线组件 BoardDailyReportTimeline（报纸布局）
前端 SHALL 提供 `BoardDailyReportTimeline.vue` 组件，替代 `BoardNarrativeTimeline.vue`。组件 SHALL 展示板块日报列表，采用长滚动报纸布局。

**纸张尺寸**：`min(1100px, 92vw)` × `92vh`，单页长滚动（不分页）。

**布局结构**（从上到下）：
1. 报头：日期大标题
2. 今日重点：highlights 展示（title + reason）
3. **质量分区**：按 `best_tier` 将聚类分为区域
   - 核心事件（Tier 0-1）：双列 CSS Grid
   - 相关事件（Tier 2）：单列
   - 其他动态（Tier 3+）：单列
4. 每个分区显示区头标签 + 聚类数

**每个聚类卡片**：默认折叠状态，显示聚类名称、文章数、「N 条线索 ▸」文本。点击卡片或「N 条线索」文本 SHALL 展开显示所有线索（title + summary），线索不显示独立状态徽章。点击 section 的 header 区域（名称） SHALL 打开右侧 SectionLifecyclePanel。

组件顶部 SHALL 提供"话题总览"按钮，点击切换到 BoardThreadBrowser 视图展示板块级话题 DAG 时间线。

**线索文章浮窗**：使用 `@floating-ui/vue`，展示 `related_article_ids` 对应的文章标题列表。首批加载 5 篇，支持"加载更多"。点选文章→emit `openArticle(articleId)`。

`dynamics` 为空时前端不渲染"板块动态"区块。

"加载更早" SHALL 每次将 `days` 增加 7 并重新请求该完整时间范围。组件 SHALL 用响应替换当前列表，避免累计请求造成重复日报。

#### Scenario: 展示日报卡片列表
- **WHEN** 选中 board "AI与机器学习"，该 board 有 3 天的日报
- **THEN** BoardDailyReportTimeline SHALL 渲染 3 张日报卡片，按日期倒序

#### Scenario: 展开日报详情
- **WHEN** 用户点击某日报卡片
- **THEN** 组件 SHALL 展开长滚动报纸布局：highlights 列表、质量分区（核心/相关/其他）

#### Scenario: 核心事件双列布局
- **WHEN** 有 3 个聚类分别属于 tier 0、tier 2、tier 3
- **THEN** tier 0 聚类在"核心事件"双列区域，tier 2 在"相关事件"单列，tier 3 在"其他动态"单列

#### Scenario: 线索默认折叠
- **WHEN** 某聚类有 3 条线索
- **THEN** 该聚类卡片 SHALL 默认只显示文章数和「3 条线索 ▸」，不显示线索标题和摘要

#### Scenario: 展开线索详情
- **WHEN** 用户点击聚类卡片或「N 条线索 ▸」
- **THEN** 卡片 SHALL 展开显示全部线索的 title + summary + 文章图标，线索不显示独立状态徽章

#### Scenario: 切换到话题总览
- **WHEN** 用户点击 "话题总览" 按钮
- **THEN** SHALL 显示 BoardThreadBrowser 组件，展示话题 DAG 时间线

#### Scenario: 点击 section header 打开 Lifecycle Panel
- **WHEN** 用户点击聚类卡片的 header 区域（名称）
- **THEN** 系统 SHALL 在 viewport 右侧弹出 SectionLifecyclePanel，展示该 section 的跨天生命周期链

#### Scenario: 空状态
- **WHEN** 选中 board 但该 board 无日报
- **THEN** 组件 SHALL 展示"暂无日报"

#### Scenario: 加载超过 30 天
- **WHEN** 当前 `days=28`，用户连续两次点击"加载更早"
- **THEN** 组件 SHALL 依次以 `days=35` 和 `days=42` 请求，并展示对应范围内的日报

#### Scenario: 切换 board 重置时间范围
- **WHEN** 用户从一个 board 切换到另一个 board
- **THEN** 组件 SHALL 将 `days` 重置为 7 并加载新 board 的日报

### Requirement: 日报生成进度前端
前端 SHALL 提供 `useDailyReportProgress.ts` composable，连接 `/ws`，过滤 `daily_report_progress`/`daily_report_done` 消息。`NarrativeGenerateDialog.vue` SHALL 改为触发日报生成（调用 `generateDailyReport`），触发后显示进度板模式：每个 board 一行，实时更新状态（等待/生成中/完成+条数），使用 `useDailyReportProgress` composable。

#### Scenario: 触发生成并显示进度
- **WHEN** 用户在 NarrativeGenerateDialog 选择日期和 board，点击"生成"
- **THEN** 对话框 SHALL 切换为进度板模式，实时显示每个 board 的生成状态

#### Scenario: 生成完成
- **WHEN** 收到 `daily_report_done` 消息
- **THEN** 进度板 SHALL 显示"全部完成"提示和总数统计

### Requirement: TagsPage 内容 Tab
TagsPage 选中 board 时 SHALL 显示三个 Tab：板块内容(composition)、日报(daily-reports)、文章(articles)。Tab 切换 SHALL 用 `v-if` 控制三个面板的显隐。默认 Tab 为"板块内容"。"日报" Tab 面板 SHALL 使用 `BoardDailyReportTimeline` 组件。

#### Scenario: Tab 切换到日报
- **WHEN** 用户点击"日报" Tab
- **THEN** 系统 SHALL 显示 BoardDailyReportTimeline 面板，隐藏 BoardCompositionPanel 和文章列表

#### Scenario: Tab 切换到文章
- **WHEN** 用户点击"文章" Tab
- **THEN** 系统 SHALL 显示带筛选的文章列表，隐藏其他面板

### Requirement: 定时任务复用
系统 SHALL 复用 `scheduler_tasks` 表中的 `narrative_summary` 任务，改造执行逻辑调用 `daily_report.GenerateDailyReport`。check_interval 保持 86400s。定时任务 SHALL 异步执行并通过 WS 广播进度。

#### Scenario: 定时触发日报生成
- **WHEN** `narrative_summary` 定时任务按 check_interval 触发
- **THEN** 系统 SHALL 为所有活跃 board 生成当日日报，使用与手动触发相同的异步 WS 流程

### Requirement: 日报 LLM 调用路由绑定
日报生成的所有 LLM 调用（事件标签语义聚类、要闻 highlights 生成、叙事线程 narrative 生成）SHALL 通过 `digest_polish` capability 加载路由与 provider，SHALL NOT 复用 `topic_tagging` 路由。这使得日报可独立配置 provider、并发上限与温度，不再与标签提取共享配额。

#### Scenario: 语义聚类调用使用 digest_polish
- **WHEN** `daily_report_cluster` 对去重后的事件标签执行 LLM 语义分组
- **THEN** LLM 调用 SHALL 使用 `digest_polish` capability

#### Scenario: 要闻与叙事调用使用 digest_polish
- **WHEN** `daily_report_llm` 生成要闻（highlights）或叙事线程（narrative）
- **THEN** LLM 调用 SHALL 使用 `digest_polish` capability

#### Scenario: 日报独立配置 provider
- **WHEN** 用户在能力路由面板为 `digest_polish` 配置了与 `topic_tagging` 不同的 provider
- **THEN** 日报生成 SHALL 使用 `digest_polish` 配置的 provider，标签提取 SHALL NOT 受影响
